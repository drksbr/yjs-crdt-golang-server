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
	"yjs-go-bridge/pkg/ynodeproto"
	"yjs-go-bridge/pkg/yprotocol"
)

// Server implementa `http.Handler` para acoplar o provider local a qualquer
// stack HTTP compatível com `net/http`.
type Server struct {
	provider                      *yprotocol.Provider
	resolveRequest                ResolveRequestFunc
	acceptOptions                 *websocket.AcceptOptions
	readLimitBytes                int64
	writeTimeout                  time.Duration
	persistTimeout                time.Duration
	authorityRevalidationInterval time.Duration
	metrics                       Metrics
	onError                       ErrorHandler

	registry   roomRegistry
	nextConnID atomic.Uint64
}

const (
	authorityLostCloseReason        = "authority_lost"
	retryableRemoteOwnerCloseReason = "retryable_close"
)

type remoteOwnerCloseSignal struct {
	reason    string
	retryable bool
}

type serverSocketSessionOptions struct {
	observeConnectionLifecycle bool
	bootstrap                  bool
	authorityLossHandler       AuthorityLossHandler
}

type authorityLossHandoffContextKey struct{}

type authorityLossHandoffState struct {
	clientErrCh <-chan error
	cancelClient context.CancelFunc
	upstreamPeer *switchableRemoteStreamPeer
}

func (s remoteOwnerCloseSignal) metricReason(fallback string) string {
	if strings.TrimSpace(s.reason) != "" {
		return s.reason
	}
	return normalizeRemoteOwnerCloseReason("", fallback)
}

func (s remoteOwnerCloseSignal) websocketReason(fallback string) string {
	return s.metricReason(fallback)
}

func (s remoteOwnerCloseSignal) websocketStatus() websocket.StatusCode {
	if s.retryable {
		return websocket.StatusTryAgainLater
	}
	return websocket.StatusGoingAway
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
		provider:                      cfg.Provider,
		resolveRequest:                cfg.ResolveRequest,
		acceptOptions:                 cloneAcceptOptions(cfg.AcceptOptions),
		readLimitBytes:                readLimit,
		writeTimeout:                  writeTimeout,
		persistTimeout:                persistTimeout,
		authorityRevalidationInterval: cfg.AuthorityRevalidationInterval,
		metrics:                       normalizeMetrics(cfg.Metrics),
		onError:                       cfg.OnError,
		registry:                      newRoomRegistry(),
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

	s.serveResolvedHTTP(w, r, req)
}

func (s *Server) serveResolvedHTTP(w http.ResponseWriter, r *http.Request, req Request) {
	s.serveResolvedHTTPWithOptions(w, r, req, serverSocketSessionOptions{
		observeConnectionLifecycle: true,
	})
}

func (s *Server) serveResolvedHTTPWithOptions(
	w http.ResponseWriter,
	r *http.Request,
	req Request,
	options serverSocketSessionOptions,
) {
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
		status := statusFromOpenError(err)
		if status == http.StatusServiceUnavailable {
			w.Header().Set("Retry-After", "1")
		}
		http.Error(w, err.Error(), status)
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
	s.serveConnectedSocket(r, req, connection, socket, options)
}

func (s *Server) serveConnectedSocket(
	r *http.Request,
	req Request,
	connection *yprotocol.Connection,
	socket *websocket.Conn,
	options serverSocketSessionOptions,
) {
	if options.authorityLossHandler != nil {
		s.serveSwitchableConnectedSocket(r, req, connection, socket, options)
		return
	}

	socket.SetReadLimit(s.readLimitBytes)

	sessionCtx, cancelSession := context.WithCancel(r.Context())
	defer cancelSession()
	if options.observeConnectionLifecycle {
		s.metrics.ConnectionOpened(req)
		defer s.metrics.ConnectionClosed(req)
	}

	peer := s.registry.add(req.DocumentKey, req.ConnectionID, &websocketPeer{conn: socket})
	var onAuthorityLoss func(remoteOwnerCloseSignal)
	if options.authorityLossHandler == nil {
		onAuthorityLoss = func(signal remoteOwnerCloseSignal) {
			if closeErr := socket.Close(signal.websocketStatus(), signal.websocketReason(authorityLostCloseReason)); closeErr != nil && !isIgnorableTransportError(closeErr) {
				s.metrics.Error(req, "close_revalidate_authority", closeErr)
				s.report(nil, req, closeErr)
			}
		}
	}
	revalidateCh := s.startAuthorityRevalidator(sessionCtx, req, connection, cancelSession, onAuthorityLoss)
	defer drainRemoteOwnerCloseSignal(revalidateCh)

	if options.bootstrap {
		if err := s.bootstrapConnection(r, req, connection, peer); err != nil {
			if isAuthorityLostRetryableError(err) {
				s.cleanupConnection(r, req, connection)
				s.handleAuthorityLoss(r, req, socket, connection.AuthorityEpoch(), options.authorityLossHandler)
				return
			}
			s.metrics.Error(req, "bootstrap", err)
			s.report(r, req, err)
			closeStatus, closeReason := websocketCloseFromRuntimeError(
				err,
				websocket.StatusGoingAway,
				"falha ao assumir owner local",
			)
			if closeErr := socket.Close(closeStatus, closeReason); closeErr != nil && !isIgnorableTransportError(closeErr) {
				s.metrics.Error(req, "close_bootstrap", closeErr)
				s.report(r, req, closeErr)
			}
			s.cleanupConnection(r, req, connection)
			s.closeSocket(r, req, socket)
			return
		}
	}

	for {
		msgType, payload, err := socket.Read(sessionCtx)
		if err != nil {
			if signal, ok := drainRemoteOwnerCloseSignal(revalidateCh); ok {
				s.cleanupConnection(r, req, connection)
				s.handleAuthorityLoss(r, req, socket, connection.AuthorityEpoch(), options.authorityLossHandler, signal)
				return
			}
			if status := websocket.CloseStatus(err); status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway && !isIgnorableTransportError(err) {
				s.metrics.Error(req, "read", err)
				s.report(r, req, err)
			}
			s.cleanupConnection(r, req, connection)
			s.closeSocket(r, req, socket)
			return
		}
		s.metrics.FrameRead(req, len(payload))
		if msgType != websocket.MessageBinary {
			if closeErr := socket.Close(websocket.StatusUnsupportedData, "yjs-go-bridge aceita apenas frames binarios"); closeErr != nil {
				s.metrics.Error(req, "close_unsupported_data", closeErr)
				s.report(r, req, closeErr)
			}
			s.cleanupConnection(r, req, connection)
			s.closeSocket(r, req, socket)
			return
		}

		handleStart := time.Now()
		result, err := connection.HandleEncodedMessages(payload)
		s.metrics.Handle(req, time.Since(handleStart), err)
		if err != nil {
			if isAuthorityLostRetryableError(err) {
				s.cleanupConnection(r, req, connection)
				s.handleAuthorityLoss(
					r,
					req,
					socket,
					connection.AuthorityEpoch(),
					options.authorityLossHandler,
					remoteOwnerCloseSignalFromError(err, "handle"),
				)
				return
			}
			s.metrics.Error(req, "handle", err)
			s.report(r, req, err)
			closeStatus, closeReason := websocketCloseFromRuntimeError(
				err,
				websocket.StatusPolicyViolation,
				"payload do y-protocol invalido",
			)
			if closeErr := socket.Close(closeStatus, closeReason); closeErr != nil {
				s.metrics.Error(req, "close_policy_violation", closeErr)
				s.report(r, req, closeErr)
			}
			s.cleanupConnection(r, req, connection)
			s.closeSocket(r, req, socket)
			return
		}

		if len(result.Direct) > 0 {
			if err := s.writeBinary(peer, result.Direct); err != nil {
				if !isIgnorableTransportError(err) {
					s.metrics.Error(req, "write_direct", err)
					s.report(r, req, err)
				}
				s.cleanupConnection(r, req, connection)
				s.closeSocket(r, req, socket)
				return
			}
			s.metrics.FrameWritten(req, "direct", len(result.Direct))
		}
		if len(result.Broadcast) > 0 {
			s.fanout(r, req, result.Broadcast)
		}
	}
}

func (s *Server) serveSwitchableConnectedSocket(
	r *http.Request,
	req Request,
	connection *yprotocol.Connection,
	socket *websocket.Conn,
	options serverSocketSessionOptions,
) {
	socket.SetReadLimit(s.readLimitBytes)

	sessionCtx, cancelSession := context.WithCancel(r.Context())
	defer cancelSession()
	if options.observeConnectionLifecycle {
		s.metrics.ConnectionOpened(req)
		defer s.metrics.ConnectionClosed(req)
	}

	peer := &websocketPeer{conn: socket}
	s.registry.add(req.DocumentKey, req.ConnectionID, peer)
	revalidateCh := s.startAuthorityRevalidator(sessionCtx, req, connection, cancelSession, nil)
	defer drainRemoteOwnerCloseSignal(revalidateCh)

	if options.bootstrap {
		if err := s.bootstrapConnection(r, req, connection, peer); err != nil {
			if isAuthorityLostRetryableError(err) {
				s.cleanupConnection(r, req, connection)
				s.handleAuthorityLoss(r, req, socket, connection.AuthorityEpoch(), options.authorityLossHandler)
				return
			}
			s.metrics.Error(req, "bootstrap", err)
			s.report(r, req, err)
			closeStatus, closeReason := websocketCloseFromRuntimeError(
				err,
				websocket.StatusGoingAway,
				"falha ao assumir owner local",
			)
			if closeErr := socket.Close(closeStatus, closeReason); closeErr != nil && !isIgnorableTransportError(closeErr) {
				s.metrics.Error(req, "close_bootstrap", closeErr)
				s.report(r, req, closeErr)
			}
			s.cleanupConnection(r, req, connection)
			return
		}
	}

	upstreamPeer := newSwitchableRemoteStreamPeer(req.DocumentKey, req.ConnectionID)
	upstreamPeer.switchTarget(&localConnectionPeer{
		server:     s,
		req:        req,
		connection: connection,
		peer:       peer,
	})

	clientCtx, cancelClient := context.WithCancel(r.Context())
	defer cancelClient()
	clientErrCh := make(chan error, 1)
	go func() {
		clientErrCh <- s.pipeClientToLocal(clientCtx, req, socket, upstreamPeer)
	}()

	handoffReq := r.WithContext(context.WithValue(r.Context(), authorityLossHandoffContextKey{}, &authorityLossHandoffState{
		clientErrCh: clientErrCh,
		cancelClient: cancelClient,
		upstreamPeer: upstreamPeer,
	}))

	for {
		select {
		case err := <-clientErrCh:
			if signal, ok := drainRemoteOwnerCloseSignal(revalidateCh); ok {
				previousEpoch := connection.AuthorityEpoch()
				upstreamPeer.clearSession()
				s.cleanupConnection(r, req, connection)
				s.handleAuthorityLoss(handoffReq, req, socket, previousEpoch, options.authorityLossHandler, signal)
				return
			}
			s.cleanupConnection(r, req, connection)
			if err != nil && !isIgnorableTransportError(err) {
				s.metrics.Error(req, "read", err)
				s.report(r, req, err)
			}
			s.closeSocket(r, req, socket)
			return
		case <-sessionCtx.Done():
			if signal, ok := drainRemoteOwnerCloseSignal(revalidateCh); ok {
				previousEpoch := connection.AuthorityEpoch()
				upstreamPeer.clearSession()
				s.cleanupConnection(r, req, connection)
				s.handleAuthorityLoss(handoffReq, req, socket, previousEpoch, options.authorityLossHandler, signal)
				return
			}
			s.cleanupConnection(r, req, connection)
			cancelClient()
			s.closeSocket(r, req, socket)
			return
		}
	}
}

func (s *Server) serveAttachedSocket(r *http.Request, req Request, socket *websocket.Conn, bootstrap bool) error {
	return s.serveAttachedSocketWithOptions(r, req, socket, serverSocketSessionOptions{
		bootstrap: bootstrap,
	})
}

func (s *Server) serveAttachedSocketWithOptions(
	r *http.Request,
	req Request,
	socket *websocket.Conn,
	options serverSocketSessionOptions,
) error {
	if err := req.DocumentKey.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(req.ConnectionID) == "" {
		req.ConnectionID = s.newConnectionID()
	}

	connection, err := s.provider.Open(r.Context(), req.DocumentKey, req.ConnectionID, req.ClientID)
	if err != nil {
		return err
	}
	s.serveConnectedSocket(r, req, connection, socket, options)
	return nil
}

func (s *Server) bootstrapConnection(r *http.Request, req Request, connection *yprotocol.Connection, peer roomPeer) error {
	for _, payload := range [][]byte{
		yprotocol.EncodeProtocolSyncStep1([]byte{0x00}),
		yprotocol.EncodeProtocolQueryAwareness(),
	} {
		handleStart := time.Now()
		result, err := connection.HandleEncodedMessages(payload)
		s.metrics.Handle(req, time.Since(handleStart), err)
		if err != nil {
			return err
		}
		if len(result.Direct) > 0 {
			if err := s.writeBinary(peer, result.Direct); err != nil {
				return err
			}
			s.metrics.FrameWritten(req, "direct", len(result.Direct))
		}
		if len(result.Broadcast) > 0 {
			s.fanout(r, req, result.Broadcast)
		}
	}
	return nil
}

func (s *Server) cleanupConnection(r *http.Request, req Request, connection *yprotocol.Connection) {
	s.registry.remove(req.DocumentKey, req.ConnectionID)

	if req.PersistOnClose && !connection.AuthorityLost() {
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
}

func (s *Server) closeSocket(r *http.Request, req Request, socket *websocket.Conn) {
	if err := socket.CloseNow(); err != nil {
		if isIgnorableTransportError(err) {
			return
		}
		s.metrics.Error(req, "close_socket", err)
		s.report(r, req, err)
	}
}

func (s *Server) pipeClientToLocal(
	ctx context.Context,
	req Request,
	socket *websocket.Conn,
	peer *switchableRemoteStreamPeer,
) error {
	for {
		msgType, payload, err := socket.Read(ctx)
		if err != nil {
			if status := websocket.CloseStatus(err); status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway && !isIgnorableTransportError(err) {
				return err
			}
			return nil
		}

		s.metrics.FrameRead(req, len(payload))
		if msgType != websocket.MessageBinary {
			if closeErr := socket.Close(websocket.StatusUnsupportedData, "yjs-go-bridge aceita apenas frames binarios"); closeErr != nil && !isIgnorableTransportError(closeErr) {
				s.metrics.Error(req, "close_unsupported_data", closeErr)
				s.report(nil, req, closeErr)
			}
			return nil
		}

		if err := peer.deliver(ctx, payload); err != nil {
			if isIgnorableTransportError(err) {
				return nil
			}
			s.metrics.Error(req, "write_direct", err)
			s.report(nil, req, err)
			return err
		}
	}
}

func (s *Server) fanout(r *http.Request, req Request, payload []byte) {
	for _, peer := range s.registry.peersExcept(req.DocumentKey, req.ConnectionID) {
		if err := s.writeBinary(peer, payload); err != nil {
			if !isIgnorableTransportError(err) {
				s.metrics.Error(req, "write_broadcast", err)
				s.report(r, req, err)
			}
			if closeErr := peer.close("falha ao entregar broadcast local"); closeErr != nil {
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

func (s *Server) writeBinary(peer roomPeer, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.writeTimeout)
	defer cancel()
	return peer.deliver(ctx, payload)
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
	case isAuthorityLostRetryableError(err):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func isAuthorityLostRetryableError(err error) bool {
	return errors.Is(err, yprotocol.ErrAuthorityLost) || errors.Is(err, storage.ErrAuthorityLost)
}

func websocketCloseFromRuntimeError(
	err error,
	fallbackStatus websocket.StatusCode,
	fallbackReason string,
) (websocket.StatusCode, string) {
	if isAuthorityLostRetryableError(err) {
		return websocket.StatusTryAgainLater, authorityLostCloseReason
	}
	return fallbackStatus, fallbackReason
}

func remoteOwnerCloseSignalFromError(err error, fallback string) remoteOwnerCloseSignal {
	if isAuthorityLostRetryableError(err) {
		return remoteOwnerCloseSignal{
			reason:    authorityLostCloseReason,
			retryable: true,
		}
	}
	return remoteOwnerCloseSignal{
		reason: normalizeRemoteOwnerCloseReason(fallback, "close"),
	}
}

func remoteOwnerCloseSignalFromMessage(message *ynodeproto.Close) remoteOwnerCloseSignal {
	if message == nil {
		return remoteOwnerCloseSignal{}
	}

	fallback := "remote_close"
	if message.Retryable {
		fallback = retryableRemoteOwnerCloseReason
	}
	return remoteOwnerCloseSignal{
		reason:    normalizeRemoteOwnerCloseReason(message.Reason, fallback),
		retryable: message.Retryable,
	}
}

func (s *Server) handleAuthorityLoss(
	r *http.Request,
	req Request,
	socket *websocket.Conn,
	previousEpoch uint64,
	handler AuthorityLossHandler,
	signals ...remoteOwnerCloseSignal,
) {
	signal := remoteOwnerCloseSignal{
		reason:    authorityLostCloseReason,
		retryable: true,
	}
	if len(signals) > 0 {
		signal = signals[0]
	}

	if handler != nil {
		if err := handler(r, req, socket, previousEpoch); err == nil {
			return
		} else {
			s.metrics.Error(req, "authority_loss_handoff", err)
			s.report(r, req, err)
		}
	}

	if closeErr := socket.Close(signal.websocketStatus(), signal.websocketReason(authorityLostCloseReason)); closeErr != nil && !isIgnorableTransportError(closeErr) {
		s.metrics.Error(req, "close_authority_loss", closeErr)
		s.report(r, req, closeErr)
	}
}

func (s *Server) startAuthorityRevalidator(
	ctx context.Context,
	req Request,
	connection *yprotocol.Connection,
	cancelSession context.CancelFunc,
	onAuthorityLoss func(remoteOwnerCloseSignal),
) <-chan remoteOwnerCloseSignal {
	signals := make(chan remoteOwnerCloseSignal, 1)
	if s == nil || connection == nil || s.authorityRevalidationInterval <= 0 {
		return signals
	}

	go func() {
		ticker := time.NewTicker(s.authorityRevalidationInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkCtx, cancel := context.WithTimeout(context.Background(), s.writeTimeout)
				start := time.Now()
				err := connection.RevalidateAuthority(checkCtx)
				cancel()
				duration := time.Since(start)
				switch {
				case err == nil, errors.Is(err, yprotocol.ErrConnectionClosed):
					if err == nil {
						observeAuthorityRevalidation(s.metrics, req, authorityRevalidationRoleLocal, duration, nil)
					}
					continue
				case isAuthorityLostRetryableError(err):
					observeAuthorityRevalidation(s.metrics, req, authorityRevalidationRoleLocal, duration, err)
					s.metrics.Error(req, "revalidate_authority", err)
					s.report(nil, req, err)
					signal := remoteOwnerCloseSignalFromError(err, "revalidate_authority")
					select {
					case signals <- signal:
					default:
					}
					if onAuthorityLoss != nil {
						onAuthorityLoss(signal)
					}
					if cancelSession != nil {
						cancelSession()
					}
					return
				default:
					observeAuthorityRevalidation(s.metrics, req, authorityRevalidationRoleLocal, duration, err)
					s.metrics.Error(req, "revalidate_authority", err)
					s.report(nil, req, err)
				}
			}
		}
	}()

	return signals
}

func drainRemoteOwnerCloseSignal(ch <-chan remoteOwnerCloseSignal) (remoteOwnerCloseSignal, bool) {
	if ch == nil {
		return remoteOwnerCloseSignal{}, false
	}

	select {
	case signal := <-ch:
		return signal, true
	default:
		return remoteOwnerCloseSignal{}, false
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
