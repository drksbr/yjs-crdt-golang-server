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
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yawareness"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ynodeproto"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
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

	writeBinary(t, left, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, left)
	writeBinary(t, right, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, right)

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

func TestOwnerAwareServerPromotesLocalOwnerWhenLookupIsUnavailable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	recorder := newRecordingMetrics()
	local, coordinator, _ := newOwnershipHTTPServer(t, store, "node-a")
	local.metrics = recorder

	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:                          local,
		OwnerLookup:                    coordinator,
		PromoteLocalOnOwnerUnavailable: true,
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	key := testDocumentKey("room-owner-promote")
	conn := dialWS(t, srv.URL+"/ws?doc=room-owner-promote&client=720&conn=promote")
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, conn)

	waitForCondition(t, 2*time.Second, func() bool {
		resolution, err := coordinator.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
		return err == nil && resolution.Local && resolution.Placement.Lease != nil
	})
	waitForCondition(t, 2*time.Second, func() bool {
		snapshot := recorder.snapshot()
		return snapshot.ownerLookupResults[ownerLookupResultNotFound] == 1 &&
			snapshot.routeDecisions[routeDecisionLocalPromote] == 1
	})

	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("conn.Close() unexpected error: %v", err)
	}
	waitForCondition(t, 2*time.Second, func() bool {
		_, err := coordinator.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
		return errors.Is(err, ycluster.ErrOwnerNotFound)
	})
}

func TestOwnerAwareServerDoesNotPromoteUnavailableOwnerByDefault(t *testing.T) {
	t.Parallel()

	store := memory.New()
	local, coordinator, _ := newOwnershipHTTPServer(t, store, "node-a")

	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:       local,
		OwnerLookup: coordinator,
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	resp, err := http.Get(srv.URL + "/ws?doc=room-owner-no-promote&client=721")
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
}

func TestOwnerAwareServerPromoteRequiresOwnershipRuntime(t *testing.T) {
	t.Parallel()

	recorder := newRecordingMetrics()
	local := newLocalHTTPServerWithMetrics(t, memory.New(), recorder)
	lookup := ownerLookupFunc(func(context.Context, ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		return nil, ycluster.ErrOwnerNotFound
	})

	handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
		Local:                          local,
		OwnerLookup:                    lookup,
		PromoteLocalOnOwnerUnavailable: true,
	})
	if err != nil {
		t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	resp, err := http.Get(srv.URL + "/ws?doc=room-owner-no-runtime&client=722")
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

	snapshot := recorder.snapshot()
	if snapshot.routeDecisions[routeDecisionLocalPromote] != 0 {
		t.Fatalf("routeDecisions[local_promote] = %d, want 0", snapshot.routeDecisions[routeDecisionLocalPromote])
	}
	if len(snapshot.errorStages) != 1 || snapshot.errorStages[0] != "lookup_owner" {
		t.Fatalf("errorStages = %v, want [lookup_owner]", snapshot.errorStages)
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

func TestOwnerAwareServerRecordsLookupAndRouteMetrics(t *testing.T) {
	t.Parallel()

	t.Run("local", func(t *testing.T) {
		recorder := newRecordingMetrics()
		local := newLocalHTTPServerWithMetrics(t, nil, recorder)
		lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
			return &ycluster.OwnerResolution{
				DocumentKey: req.DocumentKey,
				Placement: ycluster.Placement{
					ShardID: 1,
					NodeID:  "node-local-metrics",
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
		conn := dialWS(t, srv.URL+"/ws?doc=room-owner-local-metrics&client=711&conn=left")
		if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
			t.Fatalf("conn.Close() unexpected error: %v", err)
		}

		waitForCondition(t, 2*time.Second, func() bool {
			snapshot := recorder.snapshot()
			return snapshot.ownerLookupResults[ownerLookupResultLocal] == 1 &&
				snapshot.routeDecisions[routeDecisionLocal] == 1
		})
	})

	t.Run("remote metadata", func(t *testing.T) {
		recorder := newRecordingMetrics()
		local := newLocalHTTPServerWithMetrics(t, nil, recorder)
		lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
			return &ycluster.OwnerResolution{
				DocumentKey: req.DocumentKey,
				Placement: ycluster.Placement{
					ShardID: 2,
					NodeID:  "node-remote-metadata",
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
		resp, err := http.Get(srv.URL + "/ws?doc=room-owner-remote-metrics&client=712")
		if err != nil {
			t.Fatalf("http.Get() unexpected error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("resp.StatusCode = %d, want %d", resp.StatusCode, http.StatusConflict)
		}

		waitForCondition(t, 2*time.Second, func() bool {
			snapshot := recorder.snapshot()
			return snapshot.ownerLookupResults[ownerLookupResultRemote] == 1 &&
				snapshot.routeDecisions[routeDecisionRemoteHTTPMetadata] == 1
		})
	})

	t.Run("remote hook http", func(t *testing.T) {
		recorder := newRecordingMetrics()
		local := newLocalHTTPServerWithMetrics(t, nil, recorder)
		lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
			return &ycluster.OwnerResolution{
				DocumentKey: req.DocumentKey,
				Placement: ycluster.Placement{
					ShardID: 3,
					NodeID:  "node-remote-hook-metrics",
				},
				Local: false,
			}, nil
		})

		handler, err := NewOwnerAwareServer(OwnerAwareServerConfig{
			Local:       local,
			OwnerLookup: lookup,
			OnRemoteOwner: func(w http.ResponseWriter, _ *http.Request, _ Request, _ ycluster.OwnerResolution) bool {
				w.WriteHeader(http.StatusAccepted)
				return true
			},
		})
		if err != nil {
			t.Fatalf("NewOwnerAwareServer() unexpected error: %v", err)
		}

		srv := newHTTPTestServerWithHandler(t, handler)
		resp, err := http.Get(srv.URL + "/ws?doc=room-owner-hook-metrics&client=713")
		if err != nil {
			t.Fatalf("http.Get() unexpected error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("resp.StatusCode = %d, want %d", resp.StatusCode, http.StatusAccepted)
		}

		waitForCondition(t, 2*time.Second, func() bool {
			snapshot := recorder.snapshot()
			return snapshot.ownerLookupResults[ownerLookupResultRemote] == 1 &&
				snapshot.routeDecisions[routeDecisionRemoteHTTPHandler] == 1
		})
	})
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
		Local:       local,
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
		Local:       local,
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
	var handshake *ynodeproto.Handshake

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

	clientUpdate := buildGCOnlyUpdate(707, 2)
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncUpdate(clientUpdate))

	select {
	case got := <-stream.sends:
		var ok bool
		handshake, ok = got.(*ynodeproto.Handshake)
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

	remoteUpdate := buildGCOnlyUpdate(708, 1)
	stream.pushReceive(&ynodeproto.DocumentUpdate{
		DocumentKey:  testDocumentKey("room-owner-forward-ws"),
		ConnectionID: handshake.ConnectionID,
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

func TestOwnerAwareServerRemoteForwarderRecordsMetrics(t *testing.T) {
	t.Parallel()

	recorder := newRecordingMetrics()
	local := newLocalHTTPServerWithMetrics(t, nil, recorder)
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 12,
				NodeID:  "node-remote-metrics",
				Lease: &ycluster.Lease{
					ShardID: 12,
					Holder:  "node-remote-metrics",
					Epoch:   33,
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
		LocalNodeID: "node-edge-metrics",
		Local:       local,
		Dialer:      dialer,
		Metrics:     recorder,
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
	conn := dialWS(t, srv.URL+"/ws?doc=room-owner-forward-metrics&client=714&conn=edge-metrics")

	select {
	case <-dialer.requests:
	case <-time.After(testIOTimeout):
		t.Fatal("DialRemoteOwner() was not called")
	}
	handshakeMessage := readRemoteStreamMessage(t, stream)
	handshake, ok := handshakeMessage.(*ynodeproto.Handshake)
	if !ok {
		t.Fatalf("expected handshake to be forwarded to remote owner, got %T", handshakeMessage)
	}

	update := buildGCOnlyUpdate(714, 1)
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncUpdate(update))
	if _, ok := readRemoteStreamMessage(t, stream).(*ynodeproto.DocumentUpdate); !ok {
		t.Fatal("expected document update to be forwarded to remote owner")
	}

	stream.pushReceive(&ynodeproto.DocumentUpdate{
		DocumentKey:  testDocumentKey("room-owner-forward-metrics"),
		ConnectionID: handshake.ConnectionID,
		Epoch:        33,
		UpdateV1:     buildGCOnlyUpdate(715, 1),
	})
	_ = readBinary(t, conn)

	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("conn.Close() unexpected error: %v", err)
	}
	if _, ok := readRemoteStreamMessage(t, stream).(*ynodeproto.Disconnect); !ok {
		t.Fatal("expected disconnect to be forwarded to remote owner")
	}
	select {
	case <-stream.closeCh:
	case <-time.After(testIOTimeout):
		t.Fatal("remote stream was not closed after disconnect")
	}

	waitForCondition(t, 2*time.Second, func() bool {
		snapshot := recorder.snapshot()
		return snapshot.routeDecisions[routeDecisionRemoteForwardWS] == 1 &&
			snapshot.remoteOwnerConnectionsOpen[remoteOwnerMetricsRoleEdge] == 1 &&
			snapshot.remoteOwnerConnectionsClose[remoteOwnerMetricsRoleEdge] == 1 &&
			snapshot.remoteOwnerCloses[recordingRemoteOwnerCloseKey{
				role:   remoteOwnerMetricsRoleEdge,
				reason: "client_closed",
			}] == 1
	})

	snapshot := recorder.snapshot()
	if snapshot.ownerLookupResults[ownerLookupResultRemote] != 1 {
		t.Fatalf("ownerLookupResults[remote] = %d, want 1", snapshot.ownerLookupResults[ownerLookupResultRemote])
	}
	if snapshot.remoteOwnerHandshakes[recordingRemoteOwnerHandshakeKey{
		role:   remoteOwnerMetricsRoleEdge,
		result: "ok",
	}] != 1 {
		t.Fatalf("remoteOwnerHandshakes[edge ok] = %d, want 1", snapshot.remoteOwnerHandshakes[recordingRemoteOwnerHandshakeKey{
			role:   remoteOwnerMetricsRoleEdge,
			result: "ok",
		}])
	}
	if snapshot.remoteOwnerMessages[recordingRemoteOwnerMessageKey{
		role:      remoteOwnerMetricsRoleEdge,
		direction: remoteOwnerMetricsDirectionOut,
		kind:      "handshake",
	}] != 1 {
		t.Fatal("missing edge handshake metric")
	}
	if snapshot.remoteOwnerMessages[recordingRemoteOwnerMessageKey{
		role:      remoteOwnerMetricsRoleEdge,
		direction: remoteOwnerMetricsDirectionOut,
		kind:      "document_update",
	}] != 1 {
		t.Fatal("missing edge outbound document_update metric")
	}
	if snapshot.remoteOwnerMessages[recordingRemoteOwnerMessageKey{
		role:      remoteOwnerMetricsRoleEdge,
		direction: remoteOwnerMetricsDirectionIn,
		kind:      "document_update",
	}] != 1 {
		t.Fatal("missing edge inbound document_update metric")
	}
	if snapshot.remoteOwnerMessages[recordingRemoteOwnerMessageKey{
		role:      remoteOwnerMetricsRoleEdge,
		direction: remoteOwnerMetricsDirectionOut,
		kind:      "disconnect",
	}] != 1 {
		t.Fatal("missing edge disconnect metric")
	}
}

func TestOwnerAwareServerRemoteForwarderMapsRetryableCloseTo1013(t *testing.T) {
	t.Parallel()

	recorder := newRecordingMetrics()
	local := newLocalHTTPServerWithMetrics(t, nil, recorder)
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 14,
				NodeID:  "node-remote-retry",
				Lease: &ycluster.Lease{
					ShardID: 14,
					Holder:  "node-remote-retry",
					Epoch:   37,
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
		LocalNodeID: "node-edge-retry",
		Local:       local,
		Dialer:      dialer,
		Metrics:     recorder,
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
	conn := dialWS(t, srv.URL+"/ws?doc=room-owner-forward-retry&client=715&conn=edge-retry")

	select {
	case <-dialer.requests:
	case <-time.After(testIOTimeout):
		t.Fatal("DialRemoteOwner() was not called")
	}

	handshakeMessage := readRemoteStreamMessage(t, stream)
	handshake, ok := handshakeMessage.(*ynodeproto.Handshake)
	if !ok {
		t.Fatalf("handshake message = %T, want *ynodeproto.Handshake", handshakeMessage)
	}

	stream.pushReceive(&ynodeproto.Close{
		DocumentKey:  handshake.DocumentKey,
		ConnectionID: handshake.ConnectionID,
		Epoch:        handshake.Epoch,
		Retryable:    true,
		Reason:       authorityLostCloseReason,
	})

	closeErr := readCloseError(t, conn)
	if closeErr.Code != websocket.StatusTryAgainLater {
		t.Fatalf("closeErr.Code = %d, want %d", closeErr.Code, websocket.StatusTryAgainLater)
	}
	if closeErr.Reason != authorityLostCloseReason {
		t.Fatalf("closeErr.Reason = %q, want %q", closeErr.Reason, authorityLostCloseReason)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		snapshot := recorder.snapshot()
		return snapshot.remoteOwnerCloses[recordingRemoteOwnerCloseKey{
			role:   remoteOwnerMetricsRoleEdge,
			reason: authorityLostCloseReason,
		}] == 1
	})
}

func TestOwnerAwareServerRemoteForwarderRebindsAfterRetryableClose(t *testing.T) {
	t.Parallel()

	recorder := newRecordingMetrics()
	local := newLocalHTTPServerWithMetrics(t, nil, recorder)

	var (
		lookupMu    sync.Mutex
		lookupCount int
	)
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		lookupMu.Lock()
		defer lookupMu.Unlock()
		lookupCount++
		if lookupCount == 1 {
			return &ycluster.OwnerResolution{
				DocumentKey: req.DocumentKey,
				Placement: ycluster.Placement{
					ShardID: 18,
					NodeID:  "node-remote-old",
					Lease: &ycluster.Lease{
						ShardID: 18,
						Holder:  "node-remote-old",
						Epoch:   41,
						Token:   "lease-old",
					},
				},
				Local: false,
			}, nil
		}
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 18,
				NodeID:  "node-remote-new",
				Lease: &ycluster.Lease{
					ShardID: 18,
					Holder:  "node-remote-new",
					Epoch:   42,
					Token:   "lease-new",
				},
			},
			Local: false,
		}, nil
	})

	firstStream := newFakeRemoteOwnerStream()
	secondStream := newFakeRemoteOwnerStream()
	dialer := &fakeRemoteOwnerDialer{
		requests: make(chan RemoteOwnerDialRequest, 2),
		streams:  []NodeMessageStream{firstStream, secondStream},
	}
	forwardRemoteOwner, err := NewRemoteOwnerForwardHandler(RemoteOwnerForwardConfig{
		LocalNodeID: "node-edge-rebind",
		Local:       local,
		Dialer:      dialer,
		OwnerLookup: lookup,
		Metrics:     recorder,
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
	conn := dialWS(t, srv.URL+"/ws?doc=room-owner-forward-rebind&client=716&conn=edge-rebind")

	select {
	case <-dialer.requests:
	case <-time.After(testIOTimeout):
		t.Fatal("initial DialRemoteOwner() was not called")
	}

	firstHandshakeMessage := readRemoteStreamMessage(t, firstStream)
	firstHandshake, ok := firstHandshakeMessage.(*ynodeproto.Handshake)
	if !ok {
		t.Fatalf("first stream first message = %T, want *ynodeproto.Handshake", firstHandshakeMessage)
	}

	firstStream.pushReceive(&ynodeproto.Close{
		DocumentKey:  firstHandshake.DocumentKey,
		ConnectionID: firstHandshake.ConnectionID,
		Epoch:        firstHandshake.Epoch,
		Retryable:    true,
		Reason:       authorityLostCloseReason,
	})

	select {
	case dialReq := <-dialer.requests:
		if dialReq.Resolution.Placement.NodeID != "node-remote-new" {
			t.Fatalf("rebind dial node = %q, want %q", dialReq.Resolution.Placement.NodeID, "node-remote-new")
		}
	case <-time.After(testIOTimeout):
		t.Fatal("rebind DialRemoteOwner() was not called")
	}

	secondHandshakeMessage := readRemoteStreamMessage(t, secondStream)
	secondHandshake, ok := secondHandshakeMessage.(*ynodeproto.Handshake)
	if !ok {
		t.Fatalf("second stream first message = %T, want *ynodeproto.Handshake", secondHandshakeMessage)
	}
	if secondHandshake.ConnectionID != firstHandshake.ConnectionID {
		t.Fatalf("second handshake connectionID = %q, want %q", secondHandshake.ConnectionID, firstHandshake.ConnectionID)
	}
	if secondHandshake.ClientID != firstHandshake.ClientID {
		t.Fatalf("second handshake clientID = %d, want %d", secondHandshake.ClientID, firstHandshake.ClientID)
	}
	if secondHandshake.Epoch != 42 {
		t.Fatalf("second handshake epoch = %d, want %d", secondHandshake.Epoch, 42)
	}

	bootstrapSyncMessage := readRemoteStreamMessage(t, secondStream)
	bootstrapSync, ok := bootstrapSyncMessage.(*ynodeproto.DocumentSyncRequest)
	if !ok {
		t.Fatalf("second stream bootstrap sync = %T, want *ynodeproto.DocumentSyncRequest", bootstrapSyncMessage)
	}
	if bootstrapSync.ConnectionID != firstHandshake.ConnectionID {
		t.Fatalf("bootstrap sync connectionID = %q, want %q", bootstrapSync.ConnectionID, firstHandshake.ConnectionID)
	}
	if bootstrapSync.Epoch != 42 {
		t.Fatalf("bootstrap sync epoch = %d, want %d", bootstrapSync.Epoch, 42)
	}

	bootstrapAwarenessMessage := readRemoteStreamMessage(t, secondStream)
	bootstrapAwareness, ok := bootstrapAwarenessMessage.(*ynodeproto.QueryAwarenessRequest)
	if !ok {
		t.Fatalf("second stream bootstrap awareness = %T, want *ynodeproto.QueryAwarenessRequest", bootstrapAwarenessMessage)
	}
	if bootstrapAwareness.ConnectionID != firstHandshake.ConnectionID {
		t.Fatalf("bootstrap awareness connectionID = %q, want %q", bootstrapAwareness.ConnectionID, firstHandshake.ConnectionID)
	}
	if bootstrapAwareness.Epoch != 42 {
		t.Fatalf("bootstrap awareness epoch = %d, want %d", bootstrapAwareness.Epoch, 42)
	}

	remoteUpdate := buildGCOnlyUpdate(716, 1)
	secondStream.pushReceive(&ynodeproto.DocumentUpdate{
		DocumentKey:  testDocumentKey("room-owner-forward-rebind"),
		ConnectionID: secondHandshake.ConnectionID,
		Epoch:        42,
		UpdateV1:     remoteUpdate,
	})
	reply := readBinary(t, conn)
	messages, err := yprotocol.DecodeProtocolMessages(reply)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil || messages[0].Sync.Type != yprotocol.SyncMessageTypeUpdate {
		t.Fatalf("messages = %#v, want single sync update", messages)
	}
	if !bytes.Equal(messages[0].Sync.Payload, remoteUpdate) {
		t.Fatalf("sync payload = %v, want %v", messages[0].Sync.Payload, remoteUpdate)
	}

	clientUpdate := buildGCOnlyUpdate(717, 1)
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncUpdate(clientUpdate))
	forwardedUpdateMessage := readRemoteStreamMessage(t, secondStream)
	forwardedUpdate, ok := forwardedUpdateMessage.(*ynodeproto.DocumentUpdate)
	if !ok {
		t.Fatalf("second stream forwarded update = %T, want *ynodeproto.DocumentUpdate", forwardedUpdateMessage)
	}
	if !bytes.Equal(forwardedUpdate.UpdateV1, clientUpdate) {
		t.Fatalf("forwarded update payload = %v, want %v", forwardedUpdate.UpdateV1, clientUpdate)
	}
	if forwardedUpdate.Epoch != 42 {
		t.Fatalf("forwarded update epoch = %d, want %d", forwardedUpdate.Epoch, 42)
	}

	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("conn.Close() unexpected error: %v", err)
	}
	disconnectMessage := readRemoteStreamMessage(t, secondStream)
	disconnect, ok := disconnectMessage.(*ynodeproto.Disconnect)
	if !ok {
		t.Fatalf("second stream disconnect = %T, want *ynodeproto.Disconnect", disconnectMessage)
	}
	if disconnect.Epoch != 42 {
		t.Fatalf("disconnect epoch = %d, want %d", disconnect.Epoch, 42)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		snapshot := recorder.snapshot()
		return snapshot.ownershipTransitions[recordingOwnershipTransitionKey{
			from:   ownershipStateRemote,
			to:     ownershipStateRemote,
			result: "ok",
		}] == 1
	})
}

func TestOwnerAwareServerRemoteForwarderTakesOverLocalOwnerWithOwnershipRuntime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	recorder := newRecordingMetrics()
	local, coordinator, resolver := newOwnershipHTTPServer(t, store, "node-edge-local")
	local.metrics = recorder

	key := testDocumentKey("room-owner-forward-local-takeover")
	var (
		lookupMu    sync.Mutex
		lookupCount int
	)
	lookup := ownerLookupFunc(func(ctx context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		lookupMu.Lock()
		lookupCount++
		current := lookupCount
		lookupMu.Unlock()
		if current == 1 {
			return &ycluster.OwnerResolution{
				DocumentKey: req.DocumentKey,
				Placement: ycluster.Placement{
					ShardID: 22,
					NodeID:  "node-remote-old",
					Lease: &ycluster.Lease{
						ShardID: 22,
						Holder:  "node-remote-old",
						Epoch:   41,
						Token:   "lease-old",
					},
				},
				Local: false,
			}, nil
		}
		return coordinator.LookupOwner(ctx, req)
	})

	firstStream := newFakeRemoteOwnerStream()
	dialer := &fakeRemoteOwnerDialer{
		requests: make(chan RemoteOwnerDialRequest, 1),
		streams:  []NodeMessageStream{firstStream},
	}
	forwardRemoteOwner, err := NewRemoteOwnerForwardHandler(RemoteOwnerForwardConfig{
		LocalNodeID:    "node-edge-local",
		Local:          local,
		Dialer:         dialer,
		OwnerLookup:    lookup,
		Metrics:        recorder,
		RebindTimeout:  time.Second,
		RebindInterval: 10 * time.Millisecond,
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
	conn := dialWS(t, srv.URL+"/ws?doc=room-owner-forward-local-takeover&client=717&conn=edge-local")

	select {
	case <-dialer.requests:
	case <-time.After(testIOTimeout):
		t.Fatal("initial DialRemoteOwner() was not called")
	}
	firstHandshakeMessage := readRemoteStreamMessage(t, firstStream)
	firstHandshake, ok := firstHandshakeMessage.(*ynodeproto.Handshake)
	if !ok {
		t.Fatalf("first stream first message = %T, want *ynodeproto.Handshake", firstHandshakeMessage)
	}

	seedAuthoritativeHTTPDocument(t, ctx, store, resolver, key, "node-edge-local", 42, "lease-local")
	firstStream.pushReceive(&ynodeproto.Close{
		DocumentKey:  firstHandshake.DocumentKey,
		ConnectionID: firstHandshake.ConnectionID,
		Epoch:        firstHandshake.Epoch,
		Retryable:    true,
		Reason:       authorityLostCloseReason,
	})

	_ = readBinary(t, conn)
	waitForCondition(t, 2*time.Second, func() bool {
		snapshot := recorder.snapshot()
		return snapshot.ownershipTransitions[recordingOwnershipTransitionKey{
			from:   ownershipStateRemote,
			to:     ownershipStateLocal,
			result: "ok",
		}] == 1
	})

	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("conn.Close() unexpected error: %v", err)
	}
	waitForCondition(t, 2*time.Second, func() bool {
		_, err := coordinator.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
		return errors.Is(err, ycluster.ErrOwnerNotFound)
	})
}

func TestRemoteOwnerForwarderReceiveHandshakeAckHandlesInitialClose(t *testing.T) {
	t.Parallel()

	key := testDocumentKey("room-owner-forward-initial-close")
	req := Request{
		DocumentKey:  key,
		ConnectionID: "edge-initial-close",
		ClientID:     718,
	}
	resolution := ycluster.OwnerResolution{
		DocumentKey: key,
		Placement: ycluster.Placement{
			ShardID: 23,
			NodeID:  "node-remote-close",
			Lease: &ycluster.Lease{
				ShardID: 23,
				Holder:  "node-remote-close",
				Epoch:   43,
				Token:   "lease-close",
			},
		},
	}

	stream := newFakeRemoteOwnerStream()
	stream.pushReceive(&ynodeproto.Close{
		DocumentKey:  key,
		ConnectionID: req.ConnectionID,
		Epoch:        43,
		Retryable:    true,
		Reason:       authorityLostCloseReason,
	})

	forwarder := &remoteOwnerForwarder{
		writeTimeout: time.Second,
		metrics:      normalizeMetrics(nil),
	}
	err := forwarder.receiveHandshakeAck(context.Background(), req, resolution, stream, 43)

	var closeErr *remoteOwnerClosedError
	if !errors.As(err, &closeErr) {
		t.Fatalf("receiveHandshakeAck() error = %v, want remoteOwnerClosedError", err)
	}
	if closeErr.signal.metricReason("") != authorityLostCloseReason || !closeErr.signal.retryable {
		t.Fatalf("close signal = %#v, want retryable authority loss", closeErr.signal)
	}
}

func TestOwnerAwareServerRemoteForwarderMapsRetryableHandshakeCloseToUnavailable(t *testing.T) {
	t.Parallel()

	key := testDocumentKey("room-owner-forward-handshake-close")
	local := newLocalHTTPServer(t, nil)
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 24,
				NodeID:  "node-remote-close",
				Lease: &ycluster.Lease{
					ShardID: 24,
					Holder:  "node-remote-close",
					Epoch:   44,
					Token:   "lease-close",
				},
			},
			Local: false,
		}, nil
	})

	stream := newFakeRemoteOwnerStream()
	stream.handshakeAckFactory = func(handshake *ynodeproto.Handshake) ynodeproto.Message {
		return &ynodeproto.Close{
			DocumentKey:  handshake.DocumentKey,
			ConnectionID: handshake.ConnectionID,
			Epoch:        handshake.Epoch,
			Retryable:    true,
			Reason:       authorityLostCloseReason,
		}
	}
	dialer := &fakeRemoteOwnerDialer{
		requests: make(chan RemoteOwnerDialRequest, 1),
		streams:  []NodeMessageStream{stream},
	}
	forwardRemoteOwner, err := NewRemoteOwnerForwardHandler(RemoteOwnerForwardConfig{
		LocalNodeID: "node-edge-close",
		Local:       local,
		Dialer:      dialer,
		OwnerLookup: lookup,
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
	dialCtx, cancel := context.WithTimeout(context.Background(), testIOTimeout)
	defer cancel()
	conn, response, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(srv.URL+"/ws?doc="+key.DocumentID+"&client=719&conn=edge-close", "http"), nil)
	if err == nil {
		_ = conn.CloseNow()
		t.Fatal("websocket.Dial() succeeded, want retryable handshake close")
	}
	if response == nil {
		t.Fatalf("websocket.Dial() response = nil, error = %v", err)
	}
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("response.StatusCode = %d, want %d", response.StatusCode, http.StatusServiceUnavailable)
	}
	if response.Header.Get("Retry-After") != "1" {
		t.Fatalf("Retry-After = %q, want 1", response.Header.Get("Retry-After"))
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

func TestRemoteOwnerForwarderRebindTimesOutWhenEpochDoesNotAdvance(t *testing.T) {
	t.Parallel()

	key := testDocumentKey("room-owner-rebind-stale-epoch")
	var lookups atomic.Int32
	lookup := ownerLookupFunc(func(_ context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
		lookups.Add(1)
		return &ycluster.OwnerResolution{
			DocumentKey: req.DocumentKey,
			Placement: ycluster.Placement{
				ShardID: 25,
				NodeID:  "node-b",
				Lease: &ycluster.Lease{
					ShardID: 25,
					Holder:  "node-b",
					Epoch:   7,
					Token:   "lease-stale",
				},
			},
		}, nil
	})
	forwarder := &remoteOwnerForwarder{
		ownerLookup:    lookup,
		rebindTimeout:  20 * time.Millisecond,
		rebindInterval: time.Millisecond,
		writeTimeout:   time.Second,
		metrics:        normalizeMetrics(nil),
	}

	start := time.Now()
	_, err := forwarder.rebindRemoteOwnerSession(context.Background(), Request{
		DocumentKey:  key,
		ConnectionID: "conn-stale",
		ClientID:     901,
	}, remoteOwnerSession{epoch: 7}, nil, false)
	elapsed := time.Since(start)

	if !errors.Is(err, ycluster.ErrLeaseExpired) {
		t.Fatalf("rebindRemoteOwnerSession() error = %v, want %v", err, ycluster.ErrLeaseExpired)
	}
	if lookups.Load() < 2 {
		t.Fatalf("lookups = %d, want retry polling before timeout", lookups.Load())
	}
	if elapsed > time.Second {
		t.Fatalf("rebindRemoteOwnerSession() took %s, want bounded timeout", elapsed)
	}
}

type ownerLookupFunc func(ctx context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error)

func (f ownerLookupFunc) LookupOwner(ctx context.Context, req ycluster.OwnerLookupRequest) (*ycluster.OwnerResolution, error) {
	return f(ctx, req)
}

type fakeRemoteOwnerDialer struct {
	requests chan RemoteOwnerDialRequest
	stream   NodeMessageStream
	streams  []NodeMessageStream
	err      error
	mu       sync.Mutex
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
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.streams) > 0 {
		stream := d.streams[0]
		d.streams = d.streams[1:]
		if fakeStream, ok := stream.(*fakeRemoteOwnerStream); ok && fakeStream.handshakeAckFactory == nil {
			fakeStream.handshakeAckNodeID = req.Resolution.Placement.NodeID.String()
		}
		return stream, nil
	}
	if fakeStream, ok := d.stream.(*fakeRemoteOwnerStream); ok && fakeStream.handshakeAckFactory == nil {
		fakeStream.handshakeAckNodeID = req.Resolution.Placement.NodeID.String()
	}
	return d.stream, nil
}

func readCloseError(t *testing.T, conn *websocket.Conn) websocket.CloseError {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testIOTimeout)
	defer cancel()

	_, _, err := conn.Read(ctx)
	if err == nil {
		t.Fatal("conn.Read() error = nil, want close error")
	}

	var closeErr websocket.CloseError
	if !errors.As(err, &closeErr) {
		t.Fatalf("conn.Read() error = %v, want websocket.CloseError", err)
	}
	return closeErr
}

type fakeRemoteOwnerStream struct {
	receives            chan ynodeproto.Message
	sends               chan ynodeproto.Message
	closeCh             chan struct{}
	closeOnce           sync.Once
	autoHandshakeAck    bool
	handshakeAckNodeID  string
	handshakeAckFactory func(*ynodeproto.Handshake) ynodeproto.Message
}

func newFakeRemoteOwnerStream() *fakeRemoteOwnerStream {
	return &fakeRemoteOwnerStream{
		receives:         make(chan ynodeproto.Message, 8),
		sends:            make(chan ynodeproto.Message, 8),
		closeCh:          make(chan struct{}),
		autoHandshakeAck: true,
	}
}

func (s *fakeRemoteOwnerStream) Send(ctx context.Context, message ynodeproto.Message) error {
	select {
	case s.sends <- message:
		if handshake, ok := message.(*ynodeproto.Handshake); ok && s.autoHandshakeAck {
			nodeID := s.handshakeAckNodeID
			if strings.TrimSpace(nodeID) == "" {
				nodeID = "node-remote-test"
			}
			ack := ynodeproto.Message(&ynodeproto.HandshakeAck{
				NodeID:       nodeID,
				DocumentKey:  handshake.DocumentKey,
				ConnectionID: handshake.ConnectionID,
				ClientID:     handshake.ClientID,
				Epoch:        handshake.Epoch,
			})
			if s.handshakeAckFactory != nil {
				ack = s.handshakeAckFactory(handshake)
			}
			select {
			case s.receives <- ack:
			case <-s.closeCh:
				return io.EOF
			case <-ctx.Done():
				return ctx.Err()
			}
		}
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
