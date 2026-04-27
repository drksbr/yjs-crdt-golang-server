package yhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/ycluster"
	"yjs-go-bridge/pkg/ynodeproto"
	"yjs-go-bridge/pkg/yprotocol"
)

func TestOwnerAwareServerDelegatesToLocalOwner(t *testing.T) {
	t.Parallel()

	local := newLocalHTTPServer(t, nil)
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 7,
				NodeID:  "node-a",
				Version: 3,
			},
			Local: true,
		}, nil
	})

	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:       local,
		OwnerLookup: lookup,
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	left := dialWS(t, srv.URL+"/ws?doc=room-owner-local&client=701&conn=left")
	right := dialWS(t, srv.URL+"/ws?doc=room-owner-local&client=702&conn=right")

	awarenessPayload, err := yprotocol.EncodeProtocolAwarenessUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{{
			ClientID: 701,
			Clock:    1,
			State:    json.RawMessage(`{"name":"left"}`),
		}},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate() unexpected error: %v", err)
	}
	writeBinary(t, left, awarenessPayload)

	broadcast := readBinary(t, right)
	messages, err := yprotocol.DecodeProtocolMessages(broadcast)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Awareness == nil {
		t.Fatalf("messages = %#v, want single awareness message", messages)
	}
	if len(messages[0].Awareness.Clients) != 1 {
		t.Fatalf("len(messages[0].Awareness.Clients) = %d, want 1", len(messages[0].Awareness.Clients))
	}
}

func TestOwnerAwareServerReturnsRemoteOwnerMetadata(t *testing.T) {
	t.Parallel()

	local := newLocalHTTPServer(t, nil)
	expiresAt := time.Date(2026, time.April, 27, 10, 0, 0, 0, time.UTC)
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 9,
				NodeID:  "node-b",
				Version: 17,
				Lease: &ycluster.Lease{
					ShardID:   9,
					Holder:    "node-b",
					Epoch:     23,
					Token:     "opaque-token",
					ExpiresAt: expiresAt,
				},
			},
			Local: false,
		}, nil
	})

	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:       local,
		OwnerLookup: lookup,
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	resp, err := http.Get(srv.URL + "/ws?doc=room-owner-remote&client=703")
	if err != nil {
		t.Fatalf("http.Get() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("resp.StatusCode = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	if got := resp.Header.Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want %q", got, "1")
	}
	if got := resp.Header.Get("X-Yjs-Owner-Node"); got != "node-b" {
		t.Fatalf("X-Yjs-Owner-Node = %q, want %q", got, "node-b")
	}
	if got := resp.Header.Get("X-Yjs-Owner-Shard"); got != "9" {
		t.Fatalf("X-Yjs-Owner-Shard = %q, want %q", got, "9")
	}
	if got := resp.Header.Get("X-Yjs-Owner-Version"); got != "17" {
		t.Fatalf("X-Yjs-Owner-Version = %q, want %q", got, "17")
	}
	if got := resp.Header.Get("X-Yjs-Owner-Epoch"); got != "23" {
		t.Fatalf("X-Yjs-Owner-Epoch = %q, want %q", got, "23")
	}
	if got := resp.Header.Get("X-Yjs-Retryable"); got != "true" {
		t.Fatalf("X-Yjs-Retryable = %q, want %q", got, "true")
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll() unexpected error: %v", err)
	}

	var payload remoteOwnerResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() unexpected error: %v", err)
	}
	if payload.Code != "remote_owner" {
		t.Fatalf("payload.Code = %q, want %q", payload.Code, "remote_owner")
	}
	if !payload.Retryable {
		t.Fatal("payload.Retryable = false, want true")
	}
	if payload.DocumentKey != testDocumentKey("room-owner-remote") {
		t.Fatalf("payload.DocumentKey = %#v, want %#v", payload.DocumentKey, testDocumentKey("room-owner-remote"))
	}
	if payload.Owner.NodeID != "node-b" {
		t.Fatalf("payload.Owner.NodeID = %q, want %q", payload.Owner.NodeID, "node-b")
	}
	if payload.Owner.ShardID != 9 {
		t.Fatalf("payload.Owner.ShardID = %d, want %d", payload.Owner.ShardID, 9)
	}
	if payload.Owner.Version != 17 {
		t.Fatalf("payload.Owner.Version = %d, want %d", payload.Owner.Version, 17)
	}
	if payload.Owner.Epoch != 23 {
		t.Fatalf("payload.Owner.Epoch = %d, want %d", payload.Owner.Epoch, 23)
	}
	if payload.Owner.LeaseExpiresAt == nil || !payload.Owner.LeaseExpiresAt.Equal(expiresAt) {
		t.Fatalf("payload.Owner.LeaseExpiresAt = %v, want %v", payload.Owner.LeaseExpiresAt, expiresAt)
	}
	if strings.Contains(string(body), "opaque-token") {
		t.Fatal("remote owner payload leaked lease token")
	}
}

func TestOwnerAwareServerInvokesRemoteOwnerHook(t *testing.T) {
	t.Parallel()

	local := newLocalHTTPServer(t, nil)
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 3,
				NodeID:  "node-hook",
			},
			Local: false,
		}, nil
	})

	type hookInvocation struct {
		documentKey storage.DocumentKey
		nodeID      ycluster.NodeID
	}
	hookCalls := make(chan hookInvocation, 1)

	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:       local,
		OwnerLookup: lookup,
		OnRemoteOwner: func(w http.ResponseWriter, _ *http.Request, req Request, resolution ycluster.OwnerResolution) bool {
			hookCalls <- hookInvocation{
				documentKey: req.DocumentKey,
				nodeID:      resolution.Placement.NodeID,
			}
			w.Header().Set("X-Owner-Hook", "1")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("proxy pending"))
			return true
		},
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	resp, err := http.Get(srv.URL + "/ws?doc=room-owner-hook&client=704")
	if err != nil {
		t.Fatalf("http.Get() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("resp.StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	if got := resp.Header.Get("X-Owner-Hook"); got != "1" {
		t.Fatalf("X-Owner-Hook = %q, want %q", got, "1")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll() unexpected error: %v", err)
	}
	if string(body) != "proxy pending" {
		t.Fatalf("body = %q, want %q", string(body), "proxy pending")
	}

	select {
	case call := <-hookCalls:
		if call.documentKey != testDocumentKey("room-owner-hook") {
			t.Fatalf("hook DocumentKey = %#v, want %#v", call.documentKey, testDocumentKey("room-owner-hook"))
		}
		if call.nodeID != "node-hook" {
			t.Fatalf("hook NodeID = %q, want %q", call.nodeID, "node-hook")
		}
	default:
		t.Fatal("remote owner hook was not called")
	}
}

func TestOwnerAwareServerRemoteForwarderFallsBackToHTTPMetadata(t *testing.T) {
	t.Parallel()

	local := newLocalHTTPServer(t, nil)
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 5,
				NodeID:  "node-remote-http",
			},
			Local: false,
		}, nil
	})

	dialer := &fakeRemoteOwnerDialer{
		requests: make(chan RemoteOwnerDialRequest, 1),
		stream:   newFakeRemoteOwnerStream(),
	}
	forwardRemoteOwner, err := NewRemoteOwnerForwardHandler(RemoteOwnerForwardConfig{
		LocalNodeID: "node-edge-http",
		Dialer:      dialer,
	})
	if err != nil {
		t.Fatalf("NewRemoteOwnerForwardHandler() unexpected error: %v", err)
	}

	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:         local,
		OwnerLookup:   lookup,
		OnRemoteOwner: forwardRemoteOwner,
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	resp, err := http.Get(srv.URL + "/ws?doc=room-owner-forward-http&client=706")
	if err != nil {
		t.Fatalf("http.Get() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("resp.StatusCode = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	select {
	case req := <-dialer.requests:
		t.Fatalf("DialRemoteOwner() called for plain HTTP request: %#v", req)
	default:
	}
}

func TestOwnerAwareServerRemoteForwarderBridgesWebSocketFrames(t *testing.T) {
	t.Parallel()

	local := newLocalHTTPServer(t, nil)
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 11,
				NodeID:  "node-remote-ws",
				Version: 7,
				Lease: &ycluster.Lease{
					ShardID: 11,
					Holder:  "node-remote-ws",
					Epoch:   29,
					Token:   "lease-remote-ws",
				},
			},
			Local: false,
		}, nil
	})

	stream := newFakeRemoteOwnerStream()
	dialer := &fakeRemoteOwnerDialer{
		requests: make(chan RemoteOwnerDialRequest, 1),
		stream:   stream,
	}
	forwardRemoteOwner, err := NewRemoteOwnerForwardHandler(RemoteOwnerForwardConfig{
		LocalNodeID: "node-edge-ws",
		Dialer:      dialer,
	})
	if err != nil {
		t.Fatalf("NewRemoteOwnerForwardHandler() unexpected error: %v", err)
	}

	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:         local,
		OwnerLookup:   lookup,
		OnRemoteOwner: forwardRemoteOwner,
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	conn := dialWS(t, srv.URL+"/ws?doc=room-owner-forward-ws&client=707")

	select {
	case dialReq := <-dialer.requests:
		if dialReq.Request.DocumentKey != testDocumentKey("room-owner-forward-ws") {
			t.Fatalf("dialReq.Request.DocumentKey = %#v, want %#v", dialReq.Request.DocumentKey, testDocumentKey("room-owner-forward-ws"))
		}
		if dialReq.Request.ConnectionID == "" {
			t.Fatal("dialReq.Request.ConnectionID = empty, want generated connection id")
		}
		if dialReq.Resolution.Placement.NodeID != "node-remote-ws" {
			t.Fatalf("dialReq.Resolution.Placement.NodeID = %q, want %q", dialReq.Resolution.Placement.NodeID, "node-remote-ws")
		}
		if got := strings.ToLower(strings.TrimSpace(dialReq.Header.Get("Upgrade"))); got != "websocket" {
			t.Fatalf("dialReq.Header.Get(\"Upgrade\") = %q, want %q", got, "websocket")
		}
	case <-time.After(testIOTimeout):
		t.Fatal("DialRemoteOwner() was not called")
	}

	clientUpdate := []byte{0x01, 0x02, 0x03, 0x04}
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncUpdate(clientUpdate))

	select {
	case got := <-stream.sends:
		handshake, ok := got.(*ynodeproto.Handshake)
		if !ok {
			t.Fatalf("stream.Send() first message = %T, want *ynodeproto.Handshake", got)
		}
		if handshake.NodeID != "node-edge-ws" {
			t.Fatalf("handshake.NodeID = %q, want %q", handshake.NodeID, "node-edge-ws")
		}
		if handshake.DocumentKey != testDocumentKey("room-owner-forward-ws") {
			t.Fatalf("handshake.DocumentKey = %#v, want %#v", handshake.DocumentKey, testDocumentKey("room-owner-forward-ws"))
		}
		if handshake.ConnectionID == "" {
			t.Fatal("handshake.ConnectionID = empty, want generated connection id")
		}
		if handshake.ClientID != 707 {
			t.Fatalf("handshake.ClientID = %d, want %d", handshake.ClientID, 707)
		}
		if handshake.Epoch != 29 {
			t.Fatalf("handshake.Epoch = %d, want %d", handshake.Epoch, 29)
		}
	case <-time.After(testIOTimeout):
		t.Fatal("remote stream did not receive handshake")
	}

	select {
	case got := <-stream.sends:
		update, ok := got.(*ynodeproto.DocumentUpdate)
		if !ok {
			t.Fatalf("stream.Send() second message = %T, want *ynodeproto.DocumentUpdate", got)
		}
		if !bytes.Equal(update.UpdateV1, clientUpdate) {
			t.Fatalf("stream.Send().UpdateV1 = %v, want %v", update.UpdateV1, clientUpdate)
		}
		if update.DocumentKey != testDocumentKey("room-owner-forward-ws") {
			t.Fatalf("stream.Send().DocumentKey = %#v, want %#v", update.DocumentKey, testDocumentKey("room-owner-forward-ws"))
		}
		if update.ConnectionID == "" {
			t.Fatal("stream.Send().ConnectionID = empty, want generated connection id")
		}
		if update.Epoch != 29 {
			t.Fatalf("stream.Send().Epoch = %d, want %d", update.Epoch, 29)
		}
	case <-time.After(testIOTimeout):
		t.Fatal("remote stream did not receive forwarded client frame")
	}

	remoteUpdate := []byte{0x05, 0x06, 0x07}
	stream.pushReceive(&ynodeproto.DocumentUpdate{
		DocumentKey:  testDocumentKey("room-owner-forward-ws"),
		ConnectionID: "owner-conn",
		Epoch:        29,
		UpdateV1:     remoteUpdate,
	})
	reply := readBinary(t, conn)
	messages, err := yprotocol.DecodeProtocolMessages(reply)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil {
		t.Fatalf("messages = %#v, want single sync message", messages)
	}
	if messages[0].Sync.Type != yprotocol.SyncMessageTypeUpdate {
		t.Fatalf("messages[0].Sync.Type = %v, want %v", messages[0].Sync.Type, yprotocol.SyncMessageTypeUpdate)
	}
	if !bytes.Equal(messages[0].Sync.Payload, remoteUpdate) {
		t.Fatalf("messages[0].Sync.Payload = %v, want %v", messages[0].Sync.Payload, remoteUpdate)
	}

	if err := conn.Close(websocket.StatusNormalClosure, "bye"); err != nil {
		t.Fatalf("conn.Close() unexpected error: %v", err)
	}
	select {
	case got := <-stream.sends:
		disconnect, ok := got.(*ynodeproto.Disconnect)
		if !ok {
			t.Fatalf("stream.Send() close message = %T, want *ynodeproto.Disconnect", got)
		}
		if disconnect.DocumentKey != testDocumentKey("room-owner-forward-ws") {
			t.Fatalf("disconnect.DocumentKey = %#v, want %#v", disconnect.DocumentKey, testDocumentKey("room-owner-forward-ws"))
		}
		if disconnect.Epoch != 29 {
			t.Fatalf("disconnect.Epoch = %d, want %d", disconnect.Epoch, 29)
		}
	case <-time.After(testIOTimeout):
		t.Fatal("remote stream did not receive disconnect")
	}
	select {
	case <-stream.closeCh:
	case <-time.After(testIOTimeout):
		t.Fatal("remote stream was not closed after client disconnect")
	}
}

func TestOwnerAwareServerReturnsMappedLookupErrors(t *testing.T) {
	t.Parallel()

	local := newLocalHTTPServer(t, nil)
	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local: local,
		OwnerLookup: ownerLookupFunc(func(context.Context, ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
			return nil, ycluster.ErrLeaseExpired
		}),
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	resp, err := http.Get(srv.URL + "/ws?doc=room-owner-error&client=705")
	if err != nil {
		t.Fatalf("http.Get() unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("resp.StatusCode = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	if got := resp.Header.Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want %q", got, "1")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("io.ReadAll() unexpected error: %v", err)
	}
	if !strings.Contains(string(body), ycluster.ErrLeaseExpired.Error()) {
		t.Fatalf("body = %q, want substring %q", string(body), ycluster.ErrLeaseExpired.Error())
	}
}

func TestStatusFromOwnerLookupError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "invalid request",
			err:  ycluster.ErrInvalidOwnerLookupRequest,
			want: http.StatusBadRequest,
		},
		{
			name: "wrapped invalid document key",
			err:  errors.Join(errors.New("wrap"), storage.ErrInvalidDocumentKey),
			want: http.StatusBadRequest,
		},
		{
			name: "owner not found",
			err:  ycluster.ErrOwnerNotFound,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "invalid placement",
			err:  ycluster.ErrInvalidPlacement,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "lease expired",
			err:  ycluster.ErrLeaseExpired,
			want: http.StatusServiceUnavailable,
		},
		{
			name: "unknown error",
			err:  errors.New("boom"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := statusFromOwnerLookupError(tt.err); got != tt.want {
				t.Fatalf("statusFromOwnerLookupError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

type ownerLookupFunc func(ctx context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error)

func (f ownerLookupFunc) LookupOwner(ctx context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
	return f(ctx, req)
}

type fakeRemoteOwnerDialer struct {
	requests chan RemoteOwnerDialRequest
	stream   NodeMessageStream
	err      error
}

func (d *fakeRemoteOwnerDialer) DialRemoteOwner(ctx context.Context, req RemoteOwnerDialRequest) (NodeMessageStream, error) {
	if d.requests != nil {
		select {
		case d.requests <- req:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if d.err != nil {
		return nil, d.err
	}
	return d.stream, nil
}

type fakeRemoteOwnerStream struct {
	receives  chan ynodeproto.Message
	sends     chan ynodeproto.Message
	closeCh   chan struct{}
	closeOnce sync.Once
}

func newFakeRemoteOwnerStream() *fakeRemoteOwnerStream {
	return &fakeRemoteOwnerStream{
		receives: make(chan ynodeproto.Message, 4),
		sends:    make(chan ynodeproto.Message, 4),
		closeCh:  make(chan struct{}),
	}
}

func (s *fakeRemoteOwnerStream) Send(ctx context.Context, message ynodeproto.Message) error {
	select {
	case s.sends <- message:
		return nil
	case <-s.closeCh:
		return io.EOF
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *fakeRemoteOwnerStream) Receive(ctx context.Context) (ynodeproto.Message, error) {
	select {
	case message := <-s.receives:
		return message, nil
	case <-s.closeCh:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *fakeRemoteOwnerStream) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
	})
	return nil
}

func (s *fakeRemoteOwnerStream) pushReceive(message ynodeproto.Message) {
	s.receives <- message
}

func newLocalHTTPServer(t *testing.T, store storage.SnapshotStore) *Server {
	t.Helper()

	handler, err := NewServer(ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: resolveTestRequest,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	return handler
}
