package yhttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/yprotocol"
)

// Server implementa `http.Handler` para acoplar o provider local a qualquer
// stack HTTP compatível com `net/http`.
type Server struct {
	provider       *yprotocol.Provider
	resolveRequest ResolveRequestFunc
	acceptOptions  *websocket.AcceptOptions
	readLimitBytes int64
	writeTimeout   time.Duration
	persistTimeout time.Duration
	metrics        Metrics
	onError        ErrorHandler

	registry   roomRegistry
	nextConnID atomic.Uint64
}

// NewServer valida a configuração e constrói o handler HTTP/WebSocket.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Provider == nil {
		return nil, ErrNilProvider
	}
	if cfg.ResolveRequest == nil {
		return nil, ErrNilResolveRequest
	}

	readLimit := cfg.ReadLimitBytes
	if readLimit <= 0 {
		readLimit = defaultReadLimitBytes
	}

	writeTimeout := cfg.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = defaultWriteTimeout
	}

	persistTimeout := cfg.PersistTimeout
	if persistTimeout <= 0 {
		persistTimeout = defaultPersistTimeout
	}

	return &Server{
		provider:       cfg.Provider,
		resolveRequest: cfg.ResolveRequest,
		acceptOptions:  cloneAcceptOptions(cfg.AcceptOptions),
		readLimitBytes: readLimit,
		writeTimeout:   writeTimeout,
		persistTimeout: persistTimeout,
		metrics:        normalizeMetrics(cfg.Metrics),
		onError:        cfg.OnError,
		registry:       newRoomRegistry(),
	}, nil
}

// ServeHTTP executa upgrade WebSocket, abre a conexão no provider e faz o
// fanout local dos frames retornados pelo runtime de protocolo.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	req, err := s.resolveRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := req.DocumentKey.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ConnectionID) == "" {
		req.ConnectionID = s.newConnectionID()
	}

	connection, err := s.provider.Open(r.Context(), req.DocumentKey, req.ConnectionID, req.ClientID)
	if err != nil {
		s.metrics.Error(req, "open", err)
		http.Error(w, err.Error(), statusFromOpenError(err))
		return
	}

	accepted := false
	defer func() {
		if accepted {
			return
		}
		if _, closeErr := connection.Close(); closeErr != nil {
			s.report(r, req, closeErr)
		}
	}()

	socket, err := websocket.Accept(w, r, s.acceptOptions)
	if err != nil {
		return
	}
	accepted = true
	socket.SetReadLimit(s.readLimitBytes)

	peer := s.registry.add(req.DocumentKey, req.ConnectionID, socket)
	s.metrics.ConnectionOpened(req)
	defer s.cleanup(r, req, connection, socket)

	for {
		msgType, payload, err := socket.Read(r.Context())
		if err != nil {
			if status := websocket.CloseStatus(err); status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway && !isIgnorableTransportError(err) {
				s.metrics.Error(req, "read", err)
				s.report(r, req, err)
			}
			return
		}
		s.metrics.FrameRead(req, len(payload))
		if msgType != websocket.MessageBinary {
			if closeErr := socket.Close(websocket.StatusUnsupportedData, "yjs-go-bridge aceita apenas frames binarios"); closeErr != nil {
				s.metrics.Error(req, "close_unsupported_data", closeErr)
				s.report(r, req, closeErr)
			}
			return
		}

		handleStart := time.Now()
		result, err := connection.HandleEncodedMessages(payload)
		s.metrics.Handle(req, time.Since(handleStart), err)
		if err != nil {
			s.metrics.Error(req, "handle", err)
			s.report(r, req, err)
			if closeErr := socket.Close(websocket.StatusPolicyViolation, "payload do y-protocol invalido"); closeErr != nil {
				s.metrics.Error(req, "close_policy_violation", closeErr)
				s.report(r, req, closeErr)
			}
			return
		}

		if len(result.Direct) > 0 {
			if err := s.writeBinary(peer, result.Direct); err != nil {
				if !isIgnorableTransportError(err) {
					s.metrics.Error(req, "write_direct", err)
					s.report(r, req, err)
				}
				return
			}
			s.metrics.FrameWritten(req, "direct", len(result.Direct))
		}
		if len(result.Broadcast) > 0 {
			s.fanout(r, req, result.Broadcast)
		}
	}
}

func (s *Server) cleanup(r *http.Request, req Request, connection *yprotocol.Connection, socket *websocket.Conn) {
	s.registry.remove(req.DocumentKey, req.ConnectionID)
	defer s.metrics.ConnectionClosed(req)

	if req.PersistOnClose {
		ctx, cancel := context.WithTimeout(context.Background(), s.persistTimeout)
		persistStart := time.Now()
		_, err := connection.Persist(ctx)
		s.metrics.Persist(req, time.Since(persistStart), err)
		if err != nil {
			s.metrics.Error(req, "persist", err)
			s.report(r, req, err)
		}
		cancel()
	}

	result, err := connection.Close()
	if err != nil {
		s.metrics.Error(req, "close_connection", err)
		s.report(r, req, err)
	} else if len(result.Broadcast) > 0 {
		s.fanout(r, req, result.Broadcast)
	}

	if err := socket.CloseNow(); err != nil {
		if isIgnorableTransportError(err) {
			return
		}
		s.metrics.Error(req, "close_socket", err)
		s.report(r, req, err)
	}
}

func (s *Server) fanout(r *http.Request, req Request, payload []byte) {
	for _, peer := range s.registry.peersExcept(req.DocumentKey, req.ConnectionID) {
		if err := s.writeBinary(peer, payload); err != nil {
			if !isIgnorableTransportError(err) {
				s.metrics.Error(req, "write_broadcast", err)
				s.report(r, req, err)
			}
			if closeErr := peer.conn.Close(websocket.StatusGoingAway, "falha ao entregar broadcast local"); closeErr != nil {
				if isIgnorableTransportError(closeErr) {
					continue
				}
				s.metrics.Error(req, "close_broadcast_peer", closeErr)
				s.report(r, req, closeErr)
			}
			continue
		}
		s.metrics.FrameWritten(req, "broadcast", len(payload))
	}
}

func (s *Server) writeBinary(peer *peerSocket, payload []byte) error {
	peer.writeMu.Lock()
	defer peer.writeMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), s.writeTimeout)
	defer cancel()
	return peer.conn.Write(ctx, websocket.MessageBinary, payload)
}

func (s *Server) report(r *http.Request, req Request, err error) {
	if s.onError != nil && err != nil {
		s.onError(r, req, err)
	}
}

func (s *Server) newConnectionID() string {
	return fmt.Sprintf("conn-%d", s.nextConnID.Add(1))
}

func statusFromOpenError(err error) int {
	switch {
	case errors.Is(err, storage.ErrInvalidDocumentKey), errors.Is(err, yprotocol.ErrInvalidConnectionID):
		return http.StatusBadRequest
	case errors.Is(err, yprotocol.ErrConnectionExists):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func cloneAcceptOptions(src *websocket.AcceptOptions) *websocket.AcceptOptions {
	if src == nil {
		return nil
	}

	cloned := *src
	if len(src.Subprotocols) > 0 {
		cloned.Subprotocols = append([]string(nil), src.Subprotocols...)
	}
	if len(src.OriginPatterns) > 0 {
		cloned.OriginPatterns = append([]string(nil), src.OriginPatterns...)
	}
	return &cloned
}

func isIgnorableTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return true
	}
	return strings.Contains(err.Error(), "use of closed network connection")
}
