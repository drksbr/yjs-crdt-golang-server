package yhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/ycluster"
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
