package yhttp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/ynodeproto"
)

// RemoteOwnerURLResolver resolve a URL do endpoint owner-side para um owner
// remoto já resolvido.
type RemoteOwnerURLResolver func(ctx context.Context, req RemoteOwnerDialRequest) (string, error)

// WebSocketRemoteOwnerDialerConfig define o dialer websocket tipado entre edge
// e owner remoto.
type WebSocketRemoteOwnerDialerConfig struct {
	ResolveURL     RemoteOwnerURLResolver
	DialOptions    *websocket.DialOptions
	ReadLimitBytes int64
}

type webSocketRemoteOwnerDialer struct {
	resolveURL     RemoteOwnerURLResolver
	dialOptions    *websocket.DialOptions
	readLimitBytes int64
}

// NewWebSocketRemoteOwnerDialer constrói um `RemoteOwnerDialer` sobre
// WebSocket binário, reaproveitando `NodeMessageStream` como wire tipado.
func NewWebSocketRemoteOwnerDialer(cfg WebSocketRemoteOwnerDialerConfig) (RemoteOwnerDialer, error) {
	if cfg.ResolveURL == nil {
		return nil, ErrNilRemoteOwnerURLResolver
	}

	readLimit := cfg.ReadLimitBytes
	if readLimit <= 0 {
		readLimit = defaultReadLimitBytes
	}

	return &webSocketRemoteOwnerDialer{
		resolveURL:     cfg.ResolveURL,
		dialOptions:    cloneDialOptions(cfg.DialOptions),
		readLimitBytes: readLimit,
	}, nil
}

func (d *webSocketRemoteOwnerDialer) DialRemoteOwner(ctx context.Context, req RemoteOwnerDialRequest) (NodeMessageStream, error) {
	targetURL, err := d.resolveURL(ctx, req)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(targetURL) == "" {
		return nil, fmt.Errorf("yhttp: remote owner url vazia para %s", req.Resolution.Placement.NodeID)
	}

	options := cloneDialOptions(d.dialOptions)
	if options == nil {
		options = &websocket.DialOptions{}
	}
	if req.Header != nil {
		options.HTTPHeader = cloneHeader(req.Header)
	}

	socket, _, err := websocket.Dial(ctx, targetURL, options)
	if err != nil {
		return nil, err
	}
	socket.SetReadLimit(d.readLimitBytes)
	return newWebSocketNodeMessageStream(socket), nil
}

type webSocketNodeMessageStream struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func newWebSocketNodeMessageStream(conn *websocket.Conn) NodeMessageStream {
	return &webSocketNodeMessageStream{conn: conn}
}

func (s *webSocketNodeMessageStream) Send(ctx context.Context, message ynodeproto.Message) error {
	payload, err := ynodeproto.EncodeMessageFrame(message)
	if err != nil {
		return err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.Write(ctx, websocket.MessageBinary, payload)
}

func (s *webSocketNodeMessageStream) Receive(ctx context.Context) (ynodeproto.Message, error) {
	msgType, payload, err := s.conn.Read(ctx)
	if err != nil {
		if isIgnorableTransportError(err) {
			return nil, io.EOF
		}
		return nil, err
	}
	if msgType != websocket.MessageBinary {
		_ = s.conn.Close(websocket.StatusUnsupportedData, "yjs-crdt-golang-server aceita apenas frames binarios")
		return nil, fmt.Errorf("yhttp: node stream recebeu frame nao-binario: %v", msgType)
	}
	return ynodeproto.DecodeMessageFrame(payload)
}

func (s *webSocketNodeMessageStream) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.CloseNow()
}

func cloneDialOptions(src *websocket.DialOptions) *websocket.DialOptions {
	if src == nil {
		return nil
	}

	cloned := *src
	if src.HTTPHeader != nil {
		cloned.HTTPHeader = cloneHeader(src.HTTPHeader)
	}
	if len(src.Subprotocols) > 0 {
		cloned.Subprotocols = append([]string(nil), src.Subprotocols...)
	}
	return &cloned
}
