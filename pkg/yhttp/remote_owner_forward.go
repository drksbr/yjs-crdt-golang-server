package yhttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"

	"yjs-go-bridge/pkg/ycluster"
	"yjs-go-bridge/pkg/ynodeproto"
)

const remoteOwnerDialErrorStatusCode = http.StatusBadGateway

var errRemoteOwnerClosed = errors.New("yhttp: owner remoto encerrou a conexao")

// RemoteOwnerDialRequest descreve o contexto necessário para abrir um stream
// com o owner remoto resolvido.
type RemoteOwnerDialRequest struct {
	Request    Request
	Resolution ycluster.OwnerResolution
	Header     http.Header
}

// RemoteOwnerDialer abre um stream bidirecional para o owner remoto.
type RemoteOwnerDialer interface {
	DialRemoteOwner(ctx context.Context, req RemoteOwnerDialRequest) (NodeMessageStream, error)
}

// NodeMessageStream representa um stream bidirecional de mensagens tipadas
// entre edge e owner remoto, independente do transporte subjacente.
type NodeMessageStream interface {
	Send(ctx context.Context, message ynodeproto.Message) error
	Receive(ctx context.Context) (ynodeproto.Message, error)
	Close() error
}

// RemoteOwnerForwardConfig define o wiring do handler de forwarding remoto.
//
// O handler resultante deve ser usado em `OwnerAwareServerConfig.OnRemoteOwner`.
// Quando a request não for um upgrade WebSocket, o handler retorna `false` para
// permitir que `OwnerAwareServer` preserve o fallback HTTP com metadados do
// owner remoto.
type RemoteOwnerForwardConfig struct {
	LocalNodeID    ycluster.NodeID
	Dialer         RemoteOwnerDialer
	AcceptOptions  *websocket.AcceptOptions
	ReadLimitBytes int64
	WriteTimeout   time.Duration
	Metrics        Metrics
	OnError        ErrorHandler
}

type remoteOwnerForwarder struct {
	localNodeID    ycluster.NodeID
	dialer         RemoteOwnerDialer
	acceptOptions  *websocket.AcceptOptions
	readLimitBytes int64
	writeTimeout   time.Duration
	metrics        Metrics
	onError        ErrorHandler
}

// NewRemoteOwnerForwardHandler constrói um `RemoteOwnerHandler` que aceita a
// conexão WebSocket do cliente e faz bridge de frames binários com um owner
// remoto via `RemoteOwnerDialer`.
func NewRemoteOwnerForwardHandler(cfg RemoteOwnerForwardConfig) (RemoteOwnerHandler, error) {
	if cfg.Dialer == nil {
		return nil, ErrNilRemoteOwnerDialer
	}
	if err := cfg.LocalNodeID.Validate(); err != nil {
		return nil, err
	}

	readLimit := cfg.ReadLimitBytes
	if readLimit <= 0 {
		readLimit = defaultReadLimitBytes
	}

	writeTimeout := cfg.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = defaultWriteTimeout
	}

	forwarder := &remoteOwnerForwarder{
		localNodeID:    cfg.LocalNodeID,
		dialer:         cfg.Dialer,
		acceptOptions:  cloneAcceptOptions(cfg.AcceptOptions),
		readLimitBytes: readLimit,
		writeTimeout:   writeTimeout,
		metrics:        normalizeMetrics(cfg.Metrics),
		onError:        cfg.OnError,
	}
	return forwarder.handle, nil
}

func (f *remoteOwnerForwarder) handle(w http.ResponseWriter, r *http.Request, req Request, resolution ycluster.OwnerResolution) bool {
	if !isWebSocketUpgrade(r) {
		return false
	}
	epoch, err := remoteOwnerEpoch(resolution)
	if err != nil {
		f.metrics.Error(req, "remote_owner_epoch", err)
		f.report(r, req, err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return true
	}

	stream, err := f.dialer.DialRemoteOwner(r.Context(), RemoteOwnerDialRequest{
		Request:    req,
		Resolution: resolution,
		Header:     cloneHeader(r.Header),
	})
	if err != nil {
		f.metrics.Error(req, "remote_owner_dial", err)
		f.report(r, req, err)
		http.Error(w, err.Error(), remoteOwnerDialErrorStatusCode)
		return true
	}

	socket, err := websocket.Accept(w, r, f.acceptOptions)
	if err != nil {
		f.closeStream(r, req, stream)
		return true
	}
	socket.SetReadLimit(f.readLimitBytes)
	handshakeStart := time.Now()
	if err := f.sendHandshake(r.Context(), req, stream, epoch); err != nil {
		observeRemoteOwnerHandshake(f.metrics, req, remoteOwnerMetricsRoleEdge, time.Since(handshakeStart), err)
		f.metrics.Error(req, "remote_owner_handshake", err)
		f.report(r, req, err)
		_ = socket.Close(websocket.StatusGoingAway, "falha ao inicializar owner remoto")
		f.closeStream(r, req, stream)
		return true
	}
	observeRemoteOwnerHandshake(f.metrics, req, remoteOwnerMetricsRoleEdge, time.Since(handshakeStart), nil)
	observeRemoteOwnerMessage(f.metrics, req, remoteOwnerMetricsRoleEdge, remoteOwnerMetricsDirectionOut, "handshake")

	f.metrics.ConnectionOpened(req)
	observeRemoteOwnerConnectionOpened(f.metrics, req, remoteOwnerMetricsRoleEdge)
	closeReason := "client_closed"
	defer func() {
		f.cleanup(r, req, socket, stream, epoch, closeReason)
	}()

	closeReason = f.bridge(r, req, socket, stream, epoch)
	return true
}

func (f *remoteOwnerForwarder) bridge(r *http.Request, req Request, socket *websocket.Conn, stream NodeMessageStream, epoch uint64) string {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- f.pipeClientToRemote(ctx, req, socket, stream, epoch)
	}()
	go func() {
		errCh <- f.pipeRemoteToClient(ctx, req, socket, stream)
	}()

	firstErr := <-errCh
	cancel()

	if firstErr != nil {
		closeReason := "bridge_error"
		closeMessage := "falha ao encaminhar para owner remoto"
		if errors.Is(firstErr, errRemoteOwnerClosed) {
			closeReason = "remote_close"
			closeMessage = "owner remoto encerrou a conexao"
		}
		if closeErr := socket.Close(websocket.StatusGoingAway, closeMessage); closeErr != nil && !isIgnorableTransportError(closeErr) {
			f.metrics.Error(req, "remote_owner_close_client", closeErr)
			f.report(r, req, closeErr)
		}
		<-errCh
		return closeReason
	}

	<-errCh
	return "client_closed"
}

func (f *remoteOwnerForwarder) pipeClientToRemote(ctx context.Context, req Request, socket *websocket.Conn, stream NodeMessageStream, epoch uint64) error {
	peer := &remoteStreamPeer{
		stream:       stream,
		documentKey:  req.DocumentKey,
		connectionID: req.ConnectionID,
		epoch:        epoch,
	}
	for {
		msgType, payload, err := socket.Read(ctx)
		if err != nil {
			if status := websocket.CloseStatus(err); status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway && !isIgnorableTransportError(err) {
				f.metrics.Error(req, "remote_owner_read_client", err)
				f.report(nil, req, err)
				return err
			}
			return nil
		}

		f.metrics.FrameRead(req, len(payload))
		if msgType != websocket.MessageBinary {
			if closeErr := socket.Close(websocket.StatusUnsupportedData, "yjs-go-bridge aceita apenas frames binarios"); closeErr != nil && !isIgnorableTransportError(closeErr) {
				f.metrics.Error(req, "remote_owner_reject_non_binary", closeErr)
				f.report(nil, req, closeErr)
			}
			return nil
		}

		writeCtx, cancel := context.WithTimeout(ctx, f.writeTimeout)
		err = peer.deliver(writeCtx, payload)
		cancel()
		if err != nil {
			if isIgnorableRemoteOwnerStreamError(err) {
				return nil
			}
			f.metrics.Error(req, "remote_owner_write_upstream", err)
			f.report(nil, req, err)
			return err
		}
		if kinds, metricErr := protocolPayloadMetricKindsForOwner(payload); metricErr == nil {
			for _, kind := range kinds {
				observeRemoteOwnerMessage(f.metrics, req, remoteOwnerMetricsRoleEdge, remoteOwnerMetricsDirectionOut, kind)
			}
		}
		f.metrics.FrameWritten(req, "remote_owner_upstream", len(payload))
	}
}

func (f *remoteOwnerForwarder) pipeRemoteToClient(ctx context.Context, req Request, socket *websocket.Conn, stream NodeMessageStream) error {
	for {
		message, err := stream.Receive(ctx)
		if err != nil {
			if isIgnorableRemoteOwnerStreamError(err) {
				return nil
			}
			f.metrics.Error(req, "remote_owner_read_upstream", err)
			f.report(nil, req, err)
			return err
		}
		observeRemoteOwnerMessage(f.metrics, req, remoteOwnerMetricsRoleEdge, remoteOwnerMetricsDirectionIn, nodeMessageMetricKind(message))
		payload, closeClient, err := remoteMessageToProtocolPayload(message)
		if err != nil {
			f.metrics.Error(req, "remote_owner_decode_upstream", err)
			f.report(nil, req, err)
			return err
		}
		if closeClient {
			return errRemoteOwnerClosed
		}
		if len(payload) == 0 {
			continue
		}

		writeCtx, cancel := context.WithTimeout(ctx, f.writeTimeout)
		err = socket.Write(writeCtx, websocket.MessageBinary, payload)
		cancel()
		if err != nil {
			if isIgnorableTransportError(err) {
				return nil
			}
			f.metrics.Error(req, "remote_owner_write_client", err)
			f.report(nil, req, err)
			return err
		}
		f.metrics.FrameWritten(req, "remote_owner_downstream", len(payload))
	}
}

func (f *remoteOwnerForwarder) cleanup(r *http.Request, req Request, socket *websocket.Conn, stream NodeMessageStream, epoch uint64, closeReason string) {
	defer f.metrics.ConnectionClosed(req)
	defer observeRemoteOwnerConnectionClosed(f.metrics, req, remoteOwnerMetricsRoleEdge)
	defer observeRemoteOwnerClose(f.metrics, req, remoteOwnerMetricsRoleEdge, closeReason)

	f.sendDisconnect(req, stream, epoch)
	f.closeStream(r, req, stream)
	if err := socket.CloseNow(); err != nil && !isIgnorableTransportError(err) {
		f.metrics.Error(req, "remote_owner_close_socket", err)
		f.report(r, req, err)
	}
}

func (f *remoteOwnerForwarder) sendHandshake(ctx context.Context, req Request, stream NodeMessageStream, epoch uint64) error {
	flags := ynodeproto.FlagNone
	if req.PersistOnClose {
		flags |= ynodeproto.FlagPersistOnClose
	}
	return stream.Send(ctx, &ynodeproto.Handshake{
		Flags:        flags,
		NodeID:       f.localNodeID.String(),
		DocumentKey:  req.DocumentKey,
		ConnectionID: req.ConnectionID,
		ClientID:     req.ClientID,
		Epoch:        epoch,
	})
}

func (f *remoteOwnerForwarder) sendDisconnect(req Request, stream NodeMessageStream, epoch uint64) {
	if stream == nil || epoch == 0 || req.DocumentKey.DocumentID == "" || strings.TrimSpace(req.ConnectionID) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.writeTimeout)
	defer cancel()
	if err := stream.Send(ctx, &ynodeproto.Disconnect{
		DocumentKey:  req.DocumentKey,
		ConnectionID: req.ConnectionID,
		Epoch:        epoch,
	}); err != nil && !isIgnorableRemoteOwnerStreamError(err) {
		f.metrics.Error(req, "remote_owner_disconnect", err)
		f.report(nil, req, err)
	} else if err == nil {
		observeRemoteOwnerMessage(f.metrics, req, remoteOwnerMetricsRoleEdge, remoteOwnerMetricsDirectionOut, "disconnect")
	}
}

func (f *remoteOwnerForwarder) closeStream(r *http.Request, req Request, stream NodeMessageStream) {
	if err := stream.Close(); err != nil && !isIgnorableRemoteOwnerStreamError(err) {
		f.metrics.Error(req, "remote_owner_close_stream", err)
		f.report(r, req, err)
	}
}

func (f *remoteOwnerForwarder) report(r *http.Request, req Request, err error) {
	if f.onError != nil && err != nil {
		f.onError(r, req, err)
	}
}

func cloneHeader(src http.Header) http.Header {
	if src == nil {
		return nil
	}

	cloned := make(http.Header, len(src))
	for key, values := range src {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func isWebSocketUpgrade(r *http.Request) bool {
	if r == nil {
		return false
	}
	if !headerContainsToken(r.Header.Values("Connection"), "upgrade") {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")
}

func headerContainsToken(values []string, want string) bool {
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), want) {
				return true
			}
		}
	}
	return false
}

func isIgnorableRemoteOwnerStreamError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, io.EOF)
}

func remoteOwnerEpoch(resolution ycluster.OwnerResolution) (uint64, error) {
	if resolution.Placement.Lease == nil || resolution.Placement.Lease.Epoch == 0 {
		return 0, fmt.Errorf("%w: owner remoto sem epoch ativo", ycluster.ErrInvalidLease)
	}
	return resolution.Placement.Lease.Epoch, nil
}
