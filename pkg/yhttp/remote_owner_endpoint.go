package yhttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ynodeproto"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

// RemoteOwnerEndpointConfig define o endpoint owner-side que consome streams
// tipados inter-node e compartilha o runtime/fanout do `Server` local.
type RemoteOwnerEndpointConfig struct {
	Local          *Server
	LocalNodeID    ycluster.NodeID
	Authenticate   RemoteOwnerAuthenticator
	AcceptOptions  *websocket.AcceptOptions
	ReadLimitBytes int64
}

// RemoteOwnerAuthenticator valida se um handshake inter-node pode materializar
// uma conexão no owner local.
type RemoteOwnerAuthenticator func(ctx context.Context, req RemoteOwnerAuthRequest) error

// RemoteOwnerAuthRequest descreve os campos autenticáveis do handshake
// inter-node inicial.
type RemoteOwnerAuthRequest struct {
	NodeID       ycluster.NodeID
	DocumentKey  storage.DocumentKey
	ConnectionID string
	ClientID     uint32
	Epoch        uint64
	Flags        ynodeproto.Flags
	Header       http.Header
}

// RemoteOwnerEndpoint materializa conexões roteadas vindas de outros nós
// contra o provider local do owner.
type RemoteOwnerEndpoint struct {
	local          *Server
	localNodeID    ycluster.NodeID
	authenticate   RemoteOwnerAuthenticator
	acceptOptions  *websocket.AcceptOptions
	readLimitBytes int64
}

// NewRemoteOwnerEndpoint valida a configuração e constrói o endpoint owner-side.
func NewRemoteOwnerEndpoint(cfg RemoteOwnerEndpointConfig) (*RemoteOwnerEndpoint, error) {
	if cfg.Local == nil {
		return nil, ErrNilRemoteOwnerEndpoint
	}
	if err := cfg.LocalNodeID.Validate(); err != nil {
		return nil, err
	}

	readLimit := cfg.ReadLimitBytes
	if readLimit <= 0 {
		readLimit = defaultReadLimitBytes
	}

	return &RemoteOwnerEndpoint{
		local:          cfg.Local,
		localNodeID:    cfg.LocalNodeID,
		authenticate:   cfg.Authenticate,
		acceptOptions:  cloneAcceptOptions(cfg.AcceptOptions),
		readLimitBytes: readLimit,
	}, nil
}

// ServeHTTP aceita um websocket binário de nó remoto e o materializa contra o
// provider local usando o protocolo tipado de `pkg/ynodeproto`.
func (e *RemoteOwnerEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !isWebSocketUpgrade(r) {
		http.Error(w, "upgrade websocket obrigatorio", http.StatusBadRequest)
		return
	}

	socket, err := websocket.Accept(w, r, e.acceptOptions)
	if err != nil {
		return
	}
	socket.SetReadLimit(e.readLimitBytes)

	stream := newWebSocketNodeMessageStream(socket)
	if err := e.serveStream(r.Context(), stream, cloneHeader(r.Header)); err != nil && !isIgnorableNodeStreamError(err) {
		_ = socket.Close(websocket.StatusPolicyViolation, "falha ao materializar owner remoto")
	}
}

// ServeStream consome um `NodeMessageStream` tipado e o integra ao room local
// do owner usando a mesma lógica de `Provider`/fanout do `Server`.
func (e *RemoteOwnerEndpoint) ServeStream(ctx context.Context, stream NodeMessageStream) (err error) {
	return e.serveStream(ctx, stream, nil)
}

func (e *RemoteOwnerEndpoint) serveStream(ctx context.Context, stream NodeMessageStream, header http.Header) (err error) {
	if stream == nil {
		return ErrNilNodeMessageStream
	}
	if ctx == nil {
		ctx = context.Background()
	}

	handshakeStart := time.Now()
	handshake, err := e.receiveHandshake(ctx, stream)
	if err != nil {
		observeRemoteOwnerHandshake(e.local.metrics, Request{}, remoteOwnerMetricsRoleOwner, time.Since(handshakeStart), err)
		return err
	}

	req := requestFromHandshake(handshake)
	observeRemoteOwnerMessage(e.local.metrics, req, remoteOwnerMetricsRoleOwner, remoteOwnerMetricsDirectionIn, nodeMessageMetricKind(handshake))
	if err := e.authenticateHandshake(ctx, handshake, header); err != nil {
		observeRemoteOwnerHandshake(e.local.metrics, req, remoteOwnerMetricsRoleOwner, time.Since(handshakeStart), err)
		e.local.metrics.Error(req, "remote_owner_authenticate", err)
		e.local.report(nil, req, err)
		return err
	}
	ownership, err := e.local.acquireRequestOwnership(ctx, req)
	if err != nil {
		observeRemoteOwnerHandshake(e.local.metrics, req, remoteOwnerMetricsRoleOwner, time.Since(handshakeStart), err)
		if statusFromOwnershipError(err) == http.StatusServiceUnavailable {
			signal := remoteOwnerCloseSignal{
				reason:    authorityLostCloseReason,
				retryable: true,
			}
			e.sendRemoteOwnerClose(req, handshake.Epoch, stream, signal)
			if closeErr := stream.Close(); closeErr != nil && !isIgnorableNodeStreamError(closeErr) {
				e.local.metrics.Error(req, "remote_owner_close_stream", closeErr)
				e.local.report(nil, req, closeErr)
			}
			return nil
		}
		return err
	}
	connection, err := e.local.provider.Open(ctx, req.DocumentKey, req.ConnectionID, req.ClientID)
	if err != nil {
		e.local.releaseRequestOwnership(nil, req, ownership)
		observeRemoteOwnerHandshake(e.local.metrics, req, remoteOwnerMetricsRoleOwner, time.Since(handshakeStart), err)
		if isAuthorityLostRetryableError(err) {
			signal := remoteOwnerCloseSignalFromError(err, "open_error")
			e.sendRemoteOwnerClose(req, handshake.Epoch, stream, signal)
			if closeErr := stream.Close(); closeErr != nil && !isIgnorableNodeStreamError(closeErr) {
				e.local.metrics.Error(req, "remote_owner_close_stream", closeErr)
				e.local.report(nil, req, closeErr)
			}
			return nil
		}
		return err
	}
	if err := validateRemoteOwnerAuthorityEpoch(handshake.Epoch, connection.AuthorityEpoch()); err != nil {
		e.local.releaseRequestOwnership(nil, req, ownership)
		observeRemoteOwnerHandshake(e.local.metrics, req, remoteOwnerMetricsRoleOwner, time.Since(handshakeStart), err)
		e.local.metrics.Error(req, "remote_owner_authority_epoch", err)
		e.local.report(nil, req, err)
		if _, closeErr := connection.Close(); closeErr != nil {
			e.local.metrics.Error(req, "remote_owner_close_connection", closeErr)
			e.local.report(nil, req, closeErr)
		}
		signal := remoteOwnerCloseSignal{
			reason:    authorityLostCloseReason,
			retryable: true,
		}
		e.sendRemoteOwnerClose(req, handshake.Epoch, stream, signal)
		if closeErr := stream.Close(); closeErr != nil && !isIgnorableNodeStreamError(closeErr) {
			e.local.metrics.Error(req, "remote_owner_close_stream", closeErr)
			e.local.report(nil, req, closeErr)
		}
		return nil
	}

	closeClient := false
	closeSignal := remoteOwnerCloseSignal{}
	closeReason := "stream_closed"
	remoteConnectionOpened := false
	e.local.metrics.ConnectionOpened(req)
	defer func() {
		e.cleanupRemoteOwnerStream(req, handshake.Epoch, connection, stream, ownership, closeClient, closeSignal, remoteConnectionOpened, closeReason)
	}()

	if err := e.sendHandshakeAck(ctx, stream, handshake); err != nil {
		observeRemoteOwnerHandshake(e.local.metrics, req, remoteOwnerMetricsRoleOwner, time.Since(handshakeStart), err)
		closeClient = true
		closeSignal = remoteOwnerCloseSignalFromError(err, "handshake_error")
		closeReason = closeSignal.metricReason("handshake_error")
		return err
	}
	observeRemoteOwnerHandshake(e.local.metrics, req, remoteOwnerMetricsRoleOwner, time.Since(handshakeStart), nil)
	observeRemoteOwnerConnectionOpened(e.local.metrics, req, remoteOwnerMetricsRoleOwner)
	observeRemoteOwnerMessage(e.local.metrics, req, remoteOwnerMetricsRoleOwner, remoteOwnerMetricsDirectionOut, "handshake_ack")
	remoteConnectionOpened = true

	sessionCtx, cancelSession := context.WithCancel(ctx)
	defer cancelSession()
	revalidateCh := e.startAuthorityRevalidator(sessionCtx, req, connection, cancelSession)
	defer drainRemoteOwnerCloseSignal(revalidateCh)

	peer := e.local.registry.add(req.DocumentKey, req.ConnectionID, &remoteStreamPeer{
		stream:       stream,
		documentKey:  req.DocumentKey,
		connectionID: req.ConnectionID,
		epoch:        handshake.Epoch,
		onDeliver: func(message ynodeproto.Message) {
			observeRemoteOwnerMessage(e.local.metrics, req, remoteOwnerMetricsRoleOwner, remoteOwnerMetricsDirectionOut, nodeMessageMetricKind(message))
		},
	})
	defer e.local.registry.remove(req.DocumentKey, req.ConnectionID)

	for {
		message, recvErr := stream.Receive(sessionCtx)
		if recvErr != nil {
			if signal, ok := drainRemoteOwnerCloseSignal(revalidateCh); ok {
				closeClient = true
				closeSignal = signal
				closeReason = signal.metricReason("revalidate_authority")
				return nil
			}
			if isIgnorableNodeStreamError(recvErr) {
				closeReason = "stream_closed"
				return nil
			}
			closeClient = true
			closeSignal = remoteOwnerCloseSignalFromError(recvErr, "stream_error")
			closeReason = closeSignal.metricReason("stream_error")
			return recvErr
		}
		observeRemoteOwnerMessage(e.local.metrics, req, remoteOwnerMetricsRoleOwner, remoteOwnerMetricsDirectionIn, nodeMessageMetricKind(message))

		stop, messageCloseReason, handleErr := e.handleRemoteOwnerMessage(ctx, req, handshake.Epoch, connection, peer, stream, message)
		if handleErr != nil {
			closeClient = true
			closeSignal = remoteOwnerCloseSignalFromError(handleErr, messageCloseReason)
			closeReason = closeSignal.metricReason("handler_error")
			if isAuthorityLostRetryableError(handleErr) {
				return nil
			}
			return handleErr
		}
		if stop {
			closeReason = normalizeRemoteOwnerCloseReason(messageCloseReason, "stream_closed")
			return nil
		}
	}
}

// ServeNodeStream é um alias explícito para o consumo owner-side de um stream
// tipado inter-node.
func (e *RemoteOwnerEndpoint) ServeNodeStream(ctx context.Context, stream NodeMessageStream) error {
	return e.ServeStream(ctx, stream)
}

func (e *RemoteOwnerEndpoint) receiveHandshake(ctx context.Context, stream NodeMessageStream) (*ynodeproto.Handshake, error) {
	message, err := stream.Receive(ctx)
	if err != nil {
		return nil, err
	}

	handshake, ok := message.(*ynodeproto.Handshake)
	if !ok {
		return nil, fmt.Errorf("yhttp: handshake inicial obrigatorio, recebido %T", message)
	}
	if err := handshake.Validate(); err != nil {
		return nil, err
	}
	return handshake, nil
}

func (e *RemoteOwnerEndpoint) authenticateHandshake(ctx context.Context, handshake *ynodeproto.Handshake, header http.Header) error {
	if e == nil || e.authenticate == nil {
		return nil
	}
	return e.authenticate(ctx, RemoteOwnerAuthRequest{
		NodeID:       ycluster.NodeID(handshake.NodeID),
		DocumentKey:  handshake.DocumentKey,
		ConnectionID: handshake.ConnectionID,
		ClientID:     handshake.ClientID,
		Epoch:        handshake.Epoch,
		Flags:        handshake.Flags,
		Header:       cloneHeader(header),
	})
}

func requestFromHandshake(handshake *ynodeproto.Handshake) Request {
	req := Request{
		DocumentKey:  handshake.DocumentKey,
		ConnectionID: handshake.ConnectionID,
		ClientID:     handshake.ClientID,
	}
	if handshake.Flags&ynodeproto.FlagPersistOnClose != 0 {
		req.PersistOnClose = true
	}
	return req
}

func (e *RemoteOwnerEndpoint) sendHandshakeAck(ctx context.Context, stream NodeMessageStream, handshake *ynodeproto.Handshake) error {
	writeCtx, cancel := context.WithTimeout(ctx, e.local.writeTimeout)
	defer cancel()

	return stream.Send(writeCtx, &ynodeproto.HandshakeAck{
		NodeID:       e.localNodeID.String(),
		DocumentKey:  handshake.DocumentKey,
		ConnectionID: handshake.ConnectionID,
		ClientID:     handshake.ClientID,
		Epoch:        handshake.Epoch,
	})
}

func (e *RemoteOwnerEndpoint) handleRemoteOwnerMessage(
	ctx context.Context,
	req Request,
	epoch uint64,
	connection *yprotocol.Connection,
	peer roomPeer,
	stream NodeMessageStream,
	message ynodeproto.Message,
) (bool, string, error) {
	switch message := message.(type) {
	case *ynodeproto.Ping:
		writeCtx, cancel := context.WithTimeout(ctx, e.local.writeTimeout)
		defer cancel()
		if err := stream.Send(writeCtx, &ynodeproto.Pong{Nonce: message.Nonce}); err != nil {
			return false, "pong_error", err
		}
		observeRemoteOwnerMessage(e.local.metrics, req, remoteOwnerMetricsRoleOwner, remoteOwnerMetricsDirectionOut, "pong")
		return false, "", nil
	case *ynodeproto.Pong:
		return false, "", nil
	case *ynodeproto.Disconnect:
		if err := validateRemoteOwnerRoute(req, epoch, message); err != nil {
			return false, "route_mismatch", err
		}
		return true, "disconnect", nil
	case *ynodeproto.Close:
		if err := validateRemoteOwnerRoute(req, epoch, message); err != nil {
			return false, "route_mismatch", err
		}
		return true, "close", nil
	case *ynodeproto.Handshake, *ynodeproto.HandshakeAck:
		return false, "unexpected_handshake", fmt.Errorf("yhttp: mensagem de handshake inesperada apos inicializacao: %T", message)
	}

	if err := validateRemoteOwnerRoute(req, epoch, message); err != nil {
		return false, "route_mismatch", err
	}

	payload, err := ownerMessageToProtocolPayload(message)
	if err != nil {
		return false, "decode_error", err
	}
	e.local.metrics.FrameRead(req, len(payload))

	handleStart := time.Now()
	result, err := connection.HandleEncodedMessagesContext(ctx, payload)
	e.local.metrics.Handle(req, time.Since(handleStart), err)
	if err != nil {
		if !isAuthorityLostRetryableError(err) {
			e.local.metrics.Error(req, "remote_owner_handle", err)
			e.local.report(nil, req, err)
		}
		return false, remoteOwnerCloseSignalFromError(err, "handle_error").metricReason("handle_error"), err
	}

	if len(result.Direct) > 0 {
		if err := e.writeRemoteDirect(ctx, req, epoch, peer, stream, message, result.Direct); err != nil {
			e.local.metrics.Error(req, "remote_owner_write_direct", err)
			e.local.report(nil, req, err)
			return false, "write_direct_error", err
		}
		e.local.metrics.FrameWritten(req, "remote_owner_direct", len(result.Direct))
	}
	if len(result.Broadcast) > 0 {
		e.local.fanout(nil, req, result.Broadcast)
	}
	return false, "", nil
}

func (e *RemoteOwnerEndpoint) writeRemoteDirect(
	ctx context.Context,
	req Request,
	epoch uint64,
	peer roomPeer,
	stream NodeMessageStream,
	message ynodeproto.Message,
	payload []byte,
) error {
	if _, ok := message.(*ynodeproto.QueryAwarenessRequest); ok {
		directMessages, err := protocolPayloadToQueryAwarenessMessages(req.DocumentKey, req.ConnectionID, epoch, payload)
		if err != nil {
			return err
		}
		for _, direct := range directMessages {
			writeCtx, cancel := context.WithTimeout(ctx, e.local.writeTimeout)
			err := stream.Send(writeCtx, direct)
			cancel()
			if err != nil {
				return err
			}
			observeRemoteOwnerMessage(e.local.metrics, req, remoteOwnerMetricsRoleOwner, remoteOwnerMetricsDirectionOut, nodeMessageMetricKind(direct))
		}
		return nil
	}
	return e.local.writeBinary(peer, payload)
}

func (e *RemoteOwnerEndpoint) cleanupRemoteOwnerStream(
	req Request,
	epoch uint64,
	connection *yprotocol.Connection,
	stream NodeMessageStream,
	ownership *ycluster.DocumentOwnershipHandle,
	closeClient bool,
	closeSignal remoteOwnerCloseSignal,
	remoteConnectionOpened bool,
	closeReason string,
) {
	defer e.local.metrics.ConnectionClosed(req)
	if remoteConnectionOpened {
		defer observeRemoteOwnerConnectionClosed(e.local.metrics, req, remoteOwnerMetricsRoleOwner)
		defer observeRemoteOwnerClose(e.local.metrics, req, remoteOwnerMetricsRoleOwner, closeReason)
	}

	if closeClient {
		e.sendRemoteOwnerClose(req, epoch, stream, closeSignal)
	}

	if req.PersistOnClose && !connection.AuthorityLost() {
		ctx, cancel := context.WithTimeout(context.Background(), e.local.persistTimeout)
		persistStart := time.Now()
		_, err := connection.Persist(ctx)
		e.local.metrics.Persist(req, time.Since(persistStart), err)
		if err != nil {
			e.local.metrics.Error(req, "remote_owner_persist", err)
			e.local.report(nil, req, err)
		}
		cancel()
	}

	result, err := connection.Close()
	if err != nil {
		e.local.metrics.Error(req, "remote_owner_close_connection", err)
		e.local.report(nil, req, err)
	} else if len(result.Broadcast) > 0 {
		e.local.fanout(nil, req, result.Broadcast)
	}
	e.local.releaseRequestOwnership(nil, req, ownership)

	if err := stream.Close(); err != nil && !isIgnorableNodeStreamError(err) {
		e.local.metrics.Error(req, "remote_owner_close_stream", err)
		e.local.report(nil, req, err)
	}
}

func (e *RemoteOwnerEndpoint) startAuthorityRevalidator(
	ctx context.Context,
	req Request,
	connection *yprotocol.Connection,
	cancelSession context.CancelFunc,
) <-chan remoteOwnerCloseSignal {
	signals := make(chan remoteOwnerCloseSignal, 1)
	if e == nil || e.local == nil || connection == nil || e.local.authorityRevalidationInterval <= 0 {
		return signals
	}

	go func() {
		ticker := time.NewTicker(e.local.authorityRevalidationInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkCtx, cancel := context.WithTimeout(context.Background(), e.local.writeTimeout)
				start := time.Now()
				err := connection.RevalidateAuthority(checkCtx)
				cancel()
				duration := time.Since(start)
				switch {
				case err == nil, errors.Is(err, yprotocol.ErrConnectionClosed):
					if err == nil {
						observeAuthorityRevalidation(e.local.metrics, req, authorityRevalidationRoleOwner, duration, nil)
					}
					continue
				case isAuthorityLostRetryableError(err):
					observeAuthorityRevalidation(e.local.metrics, req, authorityRevalidationRoleOwner, duration, err)
					e.local.metrics.Error(req, "remote_owner_revalidate_authority", err)
					e.local.report(nil, req, err)
					select {
					case signals <- remoteOwnerCloseSignalFromError(err, "revalidate_authority"):
					default:
					}
					if cancelSession != nil {
						cancelSession()
					}
					return
				default:
					observeAuthorityRevalidation(e.local.metrics, req, authorityRevalidationRoleOwner, duration, err)
					e.local.metrics.Error(req, "remote_owner_revalidate_authority", err)
					e.local.report(nil, req, err)
				}
			}
		}
	}()

	return signals
}

func (e *RemoteOwnerEndpoint) sendRemoteOwnerClose(
	req Request,
	epoch uint64,
	stream NodeMessageStream,
	signal remoteOwnerCloseSignal,
) {
	if stream == nil || epoch == 0 || req.DocumentKey.DocumentID == "" || strings.TrimSpace(req.ConnectionID) == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.local.writeTimeout)
	defer cancel()
	if err := stream.Send(ctx, &ynodeproto.Close{
		DocumentKey:  req.DocumentKey,
		ConnectionID: req.ConnectionID,
		Epoch:        epoch,
		Retryable:    signal.retryable,
		Reason:       signal.metricReason("close"),
	}); err != nil && !isIgnorableNodeStreamError(err) {
		e.local.metrics.Error(req, "remote_owner_send_close", err)
		e.local.report(nil, req, err)
	} else if err == nil {
		observeRemoteOwnerMessage(e.local.metrics, req, remoteOwnerMetricsRoleOwner, remoteOwnerMetricsDirectionOut, "close")
	}
}

func normalizeRemoteOwnerCloseReason(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func validateRemoteOwnerRoute(req Request, epoch uint64, message ynodeproto.Message) error {
	switch message := message.(type) {
	case *ynodeproto.DocumentSyncRequest:
		return validateRemoteOwnerRouteFields(req, epoch, message.DocumentKey, message.ConnectionID, message.Epoch)
	case *ynodeproto.DocumentUpdate:
		return validateRemoteOwnerRouteFields(req, epoch, message.DocumentKey, message.ConnectionID, message.Epoch)
	case *ynodeproto.AwarenessUpdate:
		return validateRemoteOwnerRouteFields(req, epoch, message.DocumentKey, message.ConnectionID, message.Epoch)
	case *ynodeproto.QueryAwarenessRequest:
		return validateRemoteOwnerRouteFields(req, epoch, message.DocumentKey, message.ConnectionID, message.Epoch)
	case *ynodeproto.Disconnect:
		return validateRemoteOwnerRouteFields(req, epoch, message.DocumentKey, message.ConnectionID, message.Epoch)
	case *ynodeproto.Close:
		return validateRemoteOwnerRouteFields(req, epoch, message.DocumentKey, message.ConnectionID, message.Epoch)
	default:
		return fmt.Errorf("yhttp: message type remoto nao suportado pelo owner endpoint: %T", message)
	}
}

func validateRemoteOwnerRouteFields(
	req Request,
	epoch uint64,
	key storage.DocumentKey,
	connectionID string,
	messageEpoch uint64,
) error {
	if req.DocumentKey != key {
		return fmt.Errorf("yhttp: remote owner route mismatch: document key got %#v want %#v", key, req.DocumentKey)
	}
	if strings.TrimSpace(connectionID) != req.ConnectionID {
		return fmt.Errorf("yhttp: remote owner route mismatch: connection id got %q want %q", connectionID, req.ConnectionID)
	}
	if messageEpoch != epoch {
		return fmt.Errorf("yhttp: remote owner route mismatch: epoch got %d want %d", messageEpoch, epoch)
	}
	return nil
}

func validateRemoteOwnerAuthorityEpoch(handshakeEpoch uint64, authorityEpoch uint64) error {
	if authorityEpoch == 0 || handshakeEpoch == authorityEpoch {
		return nil
	}
	return fmt.Errorf("yhttp: remote owner authority epoch mismatch: handshake got %d want %d", handshakeEpoch, authorityEpoch)
}

func isIgnorableNodeStreamError(err error) bool {
	if err == nil {
		return false
	}
	return isIgnorableRemoteOwnerStreamError(err) || errors.Is(err, context.Canceled)
}
