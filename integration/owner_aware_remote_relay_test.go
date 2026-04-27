package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/ycluster"
	"yjs-go-bridge/pkg/yhttp"
	"yjs-go-bridge/pkg/ynodeproto"
	"yjs-go-bridge/pkg/yprotocol"
)

func TestOwnerAwareEdgeRelaysRemoteOwnerTraffic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()

	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	leases, err := ycluster.NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "owner-aware-remote-relay",
	}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}
	if _, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     key,
		ShardID: ycluster.StorageShardID(shardID),
		Version: 11,
	}); err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}
	if _, err := leases.AcquireLease(ctx, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  "node-b",
		TTL:     2 * time.Minute,
		Token:   "lease-node-b",
	}); err != nil {
		t.Fatalf("AcquireLease(node-b) unexpected error: %v", err)
	}

	ownerServer := newTypedOwnerTestServer(t, "node-b", store, resolver, 0)
	edgeServer := newTypedOwnerAwareTestServer(t, "node-a", store, resolver, &memoryRemoteOwnerDialer{
		t:        t,
		endpoint: ownerServer.endpoint,
	})

	ownerPeer := dialSmokeWS(t, ownerServer.URL+"/ws?doc=owner-aware-remote-relay&client=801&conn=owner")
	edgePeer := dialSmokeWS(t, edgeServer.URL+"/ws?doc=owner-aware-remote-relay&client=802&conn=edge")

	seedUpdate := buildIntegrationGCOnlyUpdate(91, 2)
	writeSmokeBinary(t, ownerPeer, yprotocol.EncodeProtocolSyncUpdate(seedUpdate))
	assertSyncUpdateMatchesUpdate(t, readSmokeBinary(t, edgePeer), seedUpdate)

	edgeAwareness, err := yprotocol.EncodeProtocolAwarenessUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{{
			ClientID: 802,
			Clock:    1,
			State:    json.RawMessage(`{"name":"edge"}`),
		}},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate(edge) unexpected error: %v", err)
	}
	writeSmokeBinary(t, edgePeer, edgeAwareness)
	assertAwarenessClientState(t, readSmokeBinary(t, ownerPeer), 802, 1, `{"name":"edge"}`)

	ownerAwareness, err := yprotocol.EncodeProtocolAwarenessUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{{
			ClientID: 801,
			Clock:    2,
			State:    json.RawMessage(`{"name":"owner"}`),
		}},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate(owner) unexpected error: %v", err)
	}
	writeSmokeBinary(t, ownerPeer, ownerAwareness)
	assertAwarenessClientState(t, readSmokeBinary(t, edgePeer), 801, 2, `{"name":"owner"}`)
}

func TestOwnerAwareEdgePropagatesRetryableRemoteClose(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()

	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	leases, err := ycluster.NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "owner-aware-remote-retry",
	}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}
	if _, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     key,
		ShardID: ycluster.StorageShardID(shardID),
		Version: 12,
	}); err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}
	if _, err := leases.AcquireLease(ctx, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  "node-b",
		TTL:     2 * time.Minute,
		Token:   "lease-node-b-retry",
	}); err != nil {
		t.Fatalf("AcquireLease(node-b) unexpected error: %v", err)
	}

	edgeServer := newTypedOwnerAwareTestServer(t, "node-a", store, resolver, &scriptedMemoryRemoteOwnerDialer{
		t: t,
		script: func(ctx context.Context, stream yhttp.NodeMessageStream) error {
			message, err := stream.Receive(ctx)
			if err != nil {
				return err
			}
			handshake, ok := message.(*ynodeproto.Handshake)
			if !ok {
				return errors.New("handshake inicial obrigatorio")
			}
			if err := stream.Send(ctx, &ynodeproto.HandshakeAck{
				NodeID:       "node-b",
				DocumentKey:  handshake.DocumentKey,
				ConnectionID: handshake.ConnectionID,
				ClientID:     handshake.ClientID,
				Epoch:        handshake.Epoch,
			}); err != nil {
				return err
			}
			if err := stream.Send(ctx, &ynodeproto.Close{
				DocumentKey:  handshake.DocumentKey,
				ConnectionID: handshake.ConnectionID,
				Epoch:        handshake.Epoch,
				Retryable:    true,
				Reason:       "authority_lost",
			}); err != nil {
				return err
			}
			return stream.Close()
		},
	})

	edgePeer := dialSmokeWS(t, edgeServer.URL+"/ws?doc=owner-aware-remote-retry&client=803&conn=edge")
	closeErr := readSmokeCloseError(t, edgePeer)
	if closeErr.Code != websocket.StatusTryAgainLater {
		t.Fatalf("closeErr.Code = %d, want %d", closeErr.Code, websocket.StatusTryAgainLater)
	}
	if closeErr.Reason != "authority_lost" {
		t.Fatalf("closeErr.Reason = %q, want %q", closeErr.Reason, "authority_lost")
	}
}

func TestOwnerAwareEdgeRebindsRemoteOwnerWithoutClientReconnect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()

	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	leases, err := ycluster.NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "owner-aware-remote-rebind",
	}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}
	if _, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     key,
		ShardID: ycluster.StorageShardID(shardID),
		Version: 13,
	}); err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}

	initialLease, err := leases.AcquireLease(ctx, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  "node-b",
		TTL:     2 * time.Minute,
		Token:   "lease-node-b-rebind",
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-b) unexpected error: %v", err)
	}

	ownerB := newTypedOwnerTestServer(t, "node-b", store, resolver, 20*time.Millisecond)
	ownerC := newTypedOwnerTestServer(t, "node-c", store, resolver, 20*time.Millisecond)
	dialer := &memoryRemoteOwnerDialer{
		t:        t,
		requests: make(chan yhttp.RemoteOwnerDialRequest, 2),
		endpoints: map[ycluster.NodeID]*yhttp.RemoteOwnerEndpoint{
			"node-b": ownerB.endpoint,
			"node-c": ownerC.endpoint,
		},
	}
	edgeServer := newTypedOwnerAwareTestServer(t, "node-a", store, resolver, dialer)

	edgePeer := dialSmokeWS(t, edgeServer.URL+"/ws?doc=owner-aware-remote-rebind&client=804&conn=edge")
	select {
	case req := <-dialer.requests:
		if req.Resolution.Placement.NodeID != "node-b" {
			t.Fatalf("initial remote dial node = %q, want %q", req.Resolution.Placement.NodeID, "node-b")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("initial remote dial was not observed")
	}

	if err := leases.ReleaseLease(ctx, *initialLease); err != nil {
		t.Fatalf("ReleaseLease(node-b) unexpected error: %v", err)
	}
	handoffAt := time.Now().UTC()
	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID: ycluster.StorageShardID(shardID),
		Owner: storage.OwnerInfo{
			NodeID: ycluster.StorageNodeID("node-c"),
			Epoch:  initialLease.Epoch + 1,
		},
		Token:      "lease-node-c-rebind",
		AcquiredAt: handoffAt,
		ExpiresAt:  handoffAt.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("SaveLease(node-c handoff) unexpected error: %v", err)
	}
	select {
	case req := <-dialer.requests:
		if req.Resolution.Placement.NodeID != "node-c" {
			t.Fatalf("rebind remote dial node = %q, want %q", req.Resolution.Placement.NodeID, "node-c")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("rebind remote dial was not observed")
	}

	ownerPeer := dialSmokeWS(t, ownerC.URL+"/ws?doc=owner-aware-remote-rebind&client=805&conn=owner-c")
	ownerUpdate := buildIntegrationGCOnlyUpdate(111, 2)
	writeSmokeBinary(t, ownerPeer, yprotocol.EncodeProtocolSyncUpdate(ownerUpdate))
	assertSyncUpdateMatchesUpdate(t, readSmokeSyncUpdate(t, edgePeer, ownerUpdate), ownerUpdate)

	edgeUpdate := buildIntegrationGCOnlyUpdate(222, 2)
	writeSmokeBinary(t, edgePeer, yprotocol.EncodeProtocolSyncUpdate(edgeUpdate))
	assertSyncUpdateMatchesUpdate(t, readSmokeSyncUpdate(t, ownerPeer, edgeUpdate), edgeUpdate)
}

func TestOwnerAwareEdgeTakesOverLocalOwnerWithoutClientReconnect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()

	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	leases, err := ycluster.NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "owner-aware-local-takeover",
	}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}
	if _, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     key,
		ShardID: ycluster.StorageShardID(shardID),
		Version: 14,
	}); err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}
	initialLease, err := leases.AcquireLease(ctx, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  "node-b",
		TTL:     2 * time.Minute,
		Token:   "lease-node-b-local",
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-b) unexpected error: %v", err)
	}

	ownerB := newTypedOwnerTestServer(t, "node-b", store, resolver, 20*time.Millisecond)
	edgeServer := newTypedOwnerAwareTestServer(t, "node-a", store, resolver, &memoryRemoteOwnerDialer{
		t:        t,
		endpoint: ownerB.endpoint,
	})

	ownerPeer := dialSmokeWS(t, ownerB.URL+"/ws?doc=owner-aware-local-takeover&client=806&conn=owner")
	edgePeer := dialSmokeWS(t, edgeServer.URL+"/ws?doc=owner-aware-local-takeover&client=807&conn=edge")

	seedUpdate := buildIntegrationGCOnlyUpdate(131, 2)
	writeSmokeBinary(t, ownerPeer, yprotocol.EncodeProtocolSyncUpdate(seedUpdate))
	assertSyncUpdateMatchesUpdate(t, readSmokeBinary(t, edgePeer), seedUpdate)

	if err := leases.ReleaseLease(ctx, *initialLease); err != nil {
		t.Fatalf("ReleaseLease(node-b) unexpected error: %v", err)
	}
	handoffAt := time.Now().UTC()
	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID: ycluster.StorageShardID(shardID),
		Owner: storage.OwnerInfo{
			NodeID: ycluster.StorageNodeID("node-a"),
			Epoch:  initialLease.Epoch + 1,
		},
		Token:      "lease-node-a-local",
		AcquiredAt: handoffAt,
		ExpiresAt:  handoffAt.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("SaveLease(node-a handoff) unexpected error: %v", err)
	}

	assertOwnerAwareSyncStep2MatchesUpdate(t, readSmokeBinary(t, edgePeer), seedUpdate)

	probe := dialSmokeWS(t, edgeServer.URL+"/ws?doc=owner-aware-local-takeover&client=808&conn=probe")
	writeSmokeBinary(t, probe, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	assertOwnerAwareSyncStep2MatchesUpdate(t, readSmokeBinary(t, probe), seedUpdate)

	edgeUpdate := buildIntegrationGCOnlyUpdate(232, 2)
	writeSmokeBinary(t, edgePeer, yprotocol.EncodeProtocolSyncUpdate(edgeUpdate))
	assertSyncUpdateMatchesUpdate(t, readSmokeSyncUpdate(t, probe, edgeUpdate), edgeUpdate)
}

func TestOwnerAwareEdgeHandsOffLocalOwnerToRemoteWithoutClientReconnect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()

	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	leases, err := ycluster.NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "owner-aware-local-to-remote",
	}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}
	if _, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     key,
		ShardID: ycluster.StorageShardID(shardID),
		Version: 15,
	}); err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}
	initialLease, err := leases.AcquireLease(ctx, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  "node-a",
		TTL:     2 * time.Minute,
		Token:   "lease-node-a-local-to-remote",
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-a) unexpected error: %v", err)
	}

	ownerB := newTypedOwnerTestServer(t, "node-b", store, resolver, 20*time.Millisecond)
	dialer := &memoryRemoteOwnerDialer{
		t:        t,
		requests: make(chan yhttp.RemoteOwnerDialRequest, 1),
		endpoints: map[ycluster.NodeID]*yhttp.RemoteOwnerEndpoint{
			"node-b": ownerB.endpoint,
		},
	}
	edgeServer := newTypedOwnerAwareTestServer(t, "node-a", store, resolver, dialer)

	edgePeer := dialSmokeWS(t, edgeServer.URL+"/ws?doc=owner-aware-local-to-remote&client=809&conn=edge")
	seedUpdate := buildIntegrationGCOnlyUpdate(333, 2)
	writeSmokeBinary(t, edgePeer, yprotocol.EncodeProtocolSyncUpdate(seedUpdate))
	deadline := time.Now().Add(5 * time.Second)
	for {
		records, listErr := store.ListUpdates(ctx, key, 0, 0)
		if listErr != nil {
			t.Fatalf("ListUpdates() unexpected error: %v", listErr)
		}
		if len(records) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("seed update was not persisted before owner handoff")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := leases.ReleaseLease(ctx, *initialLease); err != nil {
		t.Fatalf("ReleaseLease(node-a) unexpected error: %v", err)
	}
	handoffAt := time.Now().UTC()
	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID: ycluster.StorageShardID(shardID),
		Owner: storage.OwnerInfo{
			NodeID: ycluster.StorageNodeID("node-b"),
			Epoch:  initialLease.Epoch + 1,
		},
		Token:      "lease-node-b-local-to-remote",
		AcquiredAt: handoffAt,
		ExpiresAt:  handoffAt.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("SaveLease(node-b handoff) unexpected error: %v", err)
	}

	select {
	case dialReq := <-dialer.requests:
		if dialReq.Resolution.Placement.NodeID != "node-b" {
			t.Fatalf("handoff remote dial node = %q, want %q", dialReq.Resolution.Placement.NodeID, "node-b")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("local->remote handoff was not observed")
	}

	assertOwnerAwareSyncStep2MatchesUpdate(t, readSmokeBinary(t, edgePeer), seedUpdate)

	ownerPeer := dialSmokeWS(t, ownerB.URL+"/ws?doc=owner-aware-local-to-remote&client=810&conn=owner")
	ownerUpdate := buildIntegrationGCOnlyUpdate(334, 2)
	writeSmokeBinary(t, ownerPeer, yprotocol.EncodeProtocolSyncUpdate(ownerUpdate))
	assertSyncUpdateMatchesUpdate(t, readSmokeSyncUpdate(t, edgePeer, ownerUpdate), ownerUpdate)

	edgeUpdate := buildIntegrationGCOnlyUpdate(335, 2)
	writeSmokeBinary(t, edgePeer, yprotocol.EncodeProtocolSyncUpdate(edgeUpdate))
	assertSyncUpdateMatchesUpdate(t, readSmokeSyncUpdate(t, ownerPeer, edgeUpdate), edgeUpdate)
}

func newTypedOwnerAwareTestServer(
	t *testing.T,
	localNode ycluster.NodeID,
	store *memory.Store,
	resolver ycluster.ShardResolver,
	dialer yhttp.RemoteOwnerDialer,
) *httptest.Server {
	t.Helper()

	lookup, err := ycluster.NewStorageOwnerLookup(localNode, resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup(%s) unexpected error: %v", localNode, err)
	}

	localHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider: yprotocol.NewProvider(yprotocol.ProviderConfig{
			Store: store,
			ResolveAuthorityFence: func(ctx context.Context, key storage.DocumentKey) (*storage.AuthorityFence, error) {
				return ycluster.ResolveStorageAuthorityFence(ctx, lookup, key)
			},
		}),
		ResolveRequest:                resolveSmokeRequest,
		AuthorityRevalidationInterval: 20 * time.Millisecond,
		OnError: func(_ *http.Request, req yhttp.Request, err error) {
			t.Logf("edge[%s] doc=%s/%s conn=%s: %v", localNode, req.DocumentKey.Namespace, req.DocumentKey.DocumentID, req.ConnectionID, err)
		},
	})
	if err != nil {
		t.Fatalf("yhttp.NewServer() unexpected error: %v", err)
	}

	cfg := yhttp.OwnerAwareServerConfig{
		Local:       localHandler,
		OwnerLookup: lookup,
	}
	if dialer != nil {
		forwardRemoteOwner, handoffRemoteOwner, err := yhttp.NewRemoteOwnerForwardHandlers(yhttp.RemoteOwnerForwardConfig{
			LocalNodeID: localNode,
			Local:       localHandler,
			Dialer:      dialer,
			OwnerLookup: lookup,
			OnError: func(_ *http.Request, req yhttp.Request, err error) {
				t.Logf("forward[%s] doc=%s/%s conn=%s: %v", localNode, req.DocumentKey.Namespace, req.DocumentKey.DocumentID, req.ConnectionID, err)
			},
		})
		if err != nil {
			t.Fatalf("yhttp.NewRemoteOwnerForwardHandlers() unexpected error: %v", err)
		}
		cfg.OnRemoteOwner = forwardRemoteOwner
		cfg.OnLocalAuthorityLost = handoffRemoteOwner
	}

	handler, err := yhttp.NewOwnerAwareServer(cfg)
	if err != nil {
		t.Fatalf("yhttp.NewOwnerAwareServer() unexpected error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type typedOwnerTestServer struct {
	*httptest.Server
	endpoint *yhttp.RemoteOwnerEndpoint
}

func newTypedOwnerTestServer(
	t *testing.T,
	localNode ycluster.NodeID,
	store *memory.Store,
	resolver ycluster.ShardResolver,
	revalidationInterval time.Duration,
) *typedOwnerTestServer {
	t.Helper()

	ownerLookup, err := ycluster.NewStorageOwnerLookup(localNode, resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup(%s) unexpected error: %v", localNode, err)
	}

	localHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider: yprotocol.NewProvider(yprotocol.ProviderConfig{
			Store: store,
			ResolveAuthorityFence: func(ctx context.Context, key storage.DocumentKey) (*storage.AuthorityFence, error) {
				return ycluster.ResolveStorageAuthorityFence(ctx, ownerLookup, key)
			},
		}),
		ResolveRequest:                resolveSmokeRequest,
		AuthorityRevalidationInterval: revalidationInterval,
		OnError: func(_ *http.Request, req yhttp.Request, err error) {
			t.Logf("owner[%s] doc=%s/%s conn=%s: %v", localNode, req.DocumentKey.Namespace, req.DocumentKey.DocumentID, req.ConnectionID, err)
		},
	})
	if err != nil {
		t.Fatalf("yhttp.NewServer() unexpected error: %v", err)
	}
	ownerEndpoint, err := yhttp.NewRemoteOwnerEndpoint(yhttp.RemoteOwnerEndpointConfig{
		Local:       localHandler,
		LocalNodeID: localNode,
	})
	if err != nil {
		t.Fatalf("yhttp.NewRemoteOwnerEndpoint() unexpected error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", localHandler)
	mux.Handle("/node", ownerEndpoint)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &typedOwnerTestServer{
		Server:   srv,
		endpoint: ownerEndpoint,
	}
}

func assertAwarenessClientState(t *testing.T, encoded []byte, wantClientID uint32, wantClock uint32, wantState string) {
	t.Helper()

	messages, err := yprotocol.DecodeProtocolMessages(encoded)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Awareness == nil {
		t.Fatalf("messages = %#v, want single awareness message", messages)
	}
	if len(messages[0].Awareness.Clients) != 1 {
		t.Fatalf("len(messages[0].Awareness.Clients) = %d, want 1", len(messages[0].Awareness.Clients))
	}

	client := messages[0].Awareness.Clients[0]
	if client.ClientID != wantClientID {
		t.Fatalf("client.ClientID = %d, want %d", client.ClientID, wantClientID)
	}
	if client.Clock != wantClock {
		t.Fatalf("client.Clock = %d, want %d", client.Clock, wantClock)
	}
	if string(client.State) != wantState {
		t.Fatalf("client.State = %s, want %s", client.State, wantState)
	}
}

func assertSyncUpdateMatchesUpdate(t *testing.T, encoded []byte, expectedUpdate []byte) {
	t.Helper()

	messages, err := yprotocol.DecodeProtocolMessages(encoded)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil {
		t.Fatalf("messages = %#v, want single sync message", messages)
	}
	if messages[0].Sync.Type != yprotocol.SyncMessageTypeUpdate {
		t.Fatalf("messages[0].Sync.Type = %v, want %v", messages[0].Sync.Type, yprotocol.SyncMessageTypeUpdate)
	}
	if !bytes.Equal(messages[0].Sync.Payload, expectedUpdate) {
		t.Fatalf("messages[0].Sync.Payload = %v, want %v", messages[0].Sync.Payload, expectedUpdate)
	}
}

func assertOwnerAwareSyncStep2MatchesUpdate(t *testing.T, encoded []byte, expectedUpdate []byte) {
	t.Helper()

	messages, err := yprotocol.DecodeProtocolMessages(encoded)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil {
		t.Fatalf("messages = %#v, want single sync message", messages)
	}
	if messages[0].Sync.Type != yprotocol.SyncMessageTypeStep2 {
		t.Fatalf("messages[0].Sync.Type = %v, want %v", messages[0].Sync.Type, yprotocol.SyncMessageTypeStep2)
	}
	if !bytes.Equal(messages[0].Sync.Payload, expectedUpdate) {
		t.Fatalf("messages[0].Sync.Payload = %v, want %v", messages[0].Sync.Payload, expectedUpdate)
	}
}

func readSmokeSyncUpdate(t *testing.T, conn *websocket.Conn, expectedUpdate []byte) []byte {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		payload := readSmokeBinary(t, conn)
		messages, err := yprotocol.DecodeProtocolMessages(payload)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
		}
		for _, message := range messages {
			if message.Sync == nil || message.Sync.Type != yprotocol.SyncMessageTypeUpdate {
				continue
			}
			if bytes.Equal(message.Sync.Payload, expectedUpdate) {
				return payload
			}
		}
	}

	t.Fatalf("did not receive expected sync update %v before timeout", expectedUpdate)
	return nil
}

func wsURL(rawURL string) string {
	return "ws" + strings.TrimPrefix(rawURL, "http")
}

type memoryRemoteOwnerDialer struct {
	t         *testing.T
	endpoint  *yhttp.RemoteOwnerEndpoint
	endpoints map[ycluster.NodeID]*yhttp.RemoteOwnerEndpoint
	requests  chan yhttp.RemoteOwnerDialRequest
}

func (d *memoryRemoteOwnerDialer) DialRemoteOwner(ctx context.Context, req yhttp.RemoteOwnerDialRequest) (yhttp.NodeMessageStream, error) {
	if d.requests != nil {
		select {
		case d.requests <- req:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	endpoint := d.endpoint
	if len(d.endpoints) > 0 {
		endpoint = d.endpoints[req.Resolution.Placement.NodeID]
	}
	if endpoint == nil {
		return nil, errors.New("memory remote owner endpoint obrigatorio")
	}

	client, server := newMemoryNodeStreamPair()
	go func() {
		if err := endpoint.ServeNodeStream(context.Background(), server); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
			d.t.Logf("memory remote owner serve stream: %v", err)
		}
	}()
	return client, nil
}

type scriptedMemoryRemoteOwnerDialer struct {
	t      *testing.T
	script func(context.Context, yhttp.NodeMessageStream) error
}

func (d *scriptedMemoryRemoteOwnerDialer) DialRemoteOwner(ctx context.Context, _ yhttp.RemoteOwnerDialRequest) (yhttp.NodeMessageStream, error) {
	client, server := newMemoryNodeStreamPair()
	go func() {
		if d.script == nil {
			_ = server.Close()
			return
		}
		if err := d.script(context.Background(), server); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
			d.t.Logf("scripted remote owner stream: %v", err)
		}
	}()
	return client, nil
}

type memoryNodeStream struct {
	incoming <-chan []byte
	outgoing chan<- []byte
	closed   chan struct{}
	once     sync.Once
	writeMu  sync.Mutex
}

func newMemoryNodeStreamPair() (*memoryNodeStream, *memoryNodeStream) {
	leftToRight := make(chan []byte, 8)
	rightToLeft := make(chan []byte, 8)

	left := &memoryNodeStream{
		incoming: rightToLeft,
		outgoing: leftToRight,
		closed:   make(chan struct{}),
	}
	right := &memoryNodeStream{
		incoming: leftToRight,
		outgoing: rightToLeft,
		closed:   make(chan struct{}),
	}
	return left, right
}

func (s *memoryNodeStream) Send(ctx context.Context, message ynodeproto.Message) (err error) {
	payload, err := ynodeproto.EncodeMessageFrame(message)
	if err != nil {
		return err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	defer func() {
		if recover() != nil && err == nil {
			err = io.EOF
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.closed:
		return io.EOF
	case s.outgoing <- append([]byte(nil), payload...):
		return nil
	}
}

func (s *memoryNodeStream) Receive(ctx context.Context) (ynodeproto.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case payload, ok := <-s.incoming:
		if !ok {
			return nil, io.EOF
		}
		return ynodeproto.DecodeMessageFrame(payload)
	}
}

func (s *memoryNodeStream) Close() error {
	s.once.Do(func() {
		close(s.closed)
		close(s.outgoing)
	})
	return nil
}

func readSmokeCloseError(t *testing.T, conn *websocket.Conn) websocket.CloseError {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
