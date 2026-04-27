package yhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
	"yjs-go-bridge/internal/yupdate"
	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/ycluster"
	"yjs-go-bridge/pkg/yjsbridge"
	"yjs-go-bridge/pkg/yprotocol"
)

const testIOTimeout = 15 * time.Second

func TestHTTPServerBroadcastsLocalSyncAndAwareness(t *testing.T) {
	t.Parallel()

	srv := newHTTPTestServer(t, nil)
	left := dialWS(t, srv.URL+"/ws?doc=room-a&client=401&conn=left")
	right := dialWS(t, srv.URL+"/ws?doc=room-a&client=402&conn=right")

	writeBinary(t, left, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, left)
	writeBinary(t, right, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, right)

	update := buildGCOnlyUpdate(19, 2)
	writeBinary(t, left, yprotocol.EncodeProtocolSyncUpdate(update))

	broadcast := readBinary(t, right)
	messages, err := yprotocol.DecodeProtocolMessages(broadcast)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(sync broadcast) unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil {
		t.Fatalf("sync broadcast messages = %#v, want single sync message", messages)
	}
	if messages[0].Sync.Type != yprotocol.SyncMessageTypeUpdate {
		t.Fatalf("sync broadcast type = %v, want %v", messages[0].Sync.Type, yprotocol.SyncMessageTypeUpdate)
	}
	if !bytes.Equal(messages[0].Sync.Payload, update) {
		t.Fatalf("sync broadcast payload = %v, want %v", messages[0].Sync.Payload, update)
	}

	awarenessPayload, err := yprotocol.EncodeProtocolAwarenessUpdate(&yawareness.Update{
		Clients: []yawareness.ClientState{{
			ClientID: 401,
			Clock:    1,
			State:    json.RawMessage(`{"name":"left"}`),
		}},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate() unexpected error: %v", err)
	}
	writeBinary(t, left, awarenessPayload)

	awarenessBroadcast := readBinary(t, right)
	awarenessMessages, err := yprotocol.DecodeProtocolMessages(awarenessBroadcast)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(awareness broadcast) unexpected error: %v", err)
	}
	if len(awarenessMessages) != 1 || awarenessMessages[0].Awareness == nil {
		t.Fatalf("awareness broadcast messages = %#v, want single awareness message", awarenessMessages)
	}
	if len(awarenessMessages[0].Awareness.Clients) != 1 {
		t.Fatalf("len(awareness clients) = %d, want 1", len(awarenessMessages[0].Awareness.Clients))
	}
	client := awarenessMessages[0].Awareness.Clients[0]
	if client.ClientID != 401 || client.Clock != 1 {
		t.Fatalf("awareness client = %#v, want clientID=401 clock=1", client)
	}
	if !bytes.Equal(client.State, []byte(`{"name":"left"}`)) {
		t.Fatalf("awareness state = %s, want %s", client.State, `{"name":"left"}`)
	}
}

func TestHTTPServerPersistsSnapshotOnClose(t *testing.T) {
	t.Parallel()

	store := memory.New()
	firstServer := newHTTPTestServer(t, store)
	first := dialWS(t, firstServer.URL+"/ws?doc=doc-persist&client=501&persist=1")

	update := buildGCOnlyUpdate(29, 4)
	writeBinary(t, first, yprotocol.EncodeProtocolSyncUpdate(update))
	if err := first.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("first.Close() unexpected error: %v", err)
	}

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "doc-persist"}
	waitForSnapshot(t, store, key)

	firstServer.Close()
	secondServer := newHTTPTestServer(t, store)
	probe := dialWS(t, secondServer.URL+"/ws?doc=doc-persist&client=502")

	writeBinary(t, probe, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	reply := readBinary(t, probe)

	messages, err := yprotocol.DecodeProtocolMessages(reply)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(step2 reply) unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil {
		t.Fatalf("step2 reply messages = %#v, want single sync message", messages)
	}
	if messages[0].Sync.Type != yprotocol.SyncMessageTypeStep2 {
		t.Fatalf("step2 reply type = %v, want %v", messages[0].Sync.Type, yprotocol.SyncMessageTypeStep2)
	}

	expected, err := yjsbridge.DiffUpdate(update, []byte{0x00})
	if err != nil {
		t.Fatalf("DiffUpdate() unexpected error: %v", err)
	}
	if !bytes.Equal(messages[0].Sync.Payload, expected) {
		t.Fatalf("step2 reply payload = %v, want %v", messages[0].Sync.Payload, expected)
	}
}

func TestHTTPServerRevalidatesAuthorityAndClosesIdleConnection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := testDocumentKey("doc-authority-revalidate")
	store, resolver, provider := newAuthoritativeHTTPProvider(t, "node-a")
	seedAuthoritativeHTTPDocument(t, ctx, store, resolver, key, "node-a", 1, "lease-node-a")
	recorder := newRecordingMetrics()

	handler, err := NewServer(ServerConfig{
		Provider:                      provider,
		ResolveRequest:                resolveTestRequest,
		AuthorityRevalidationInterval: 10 * time.Millisecond,
		Metrics:                       recorder,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	conn := dialWS(t, srv.URL+"/ws?doc=doc-authority-revalidate&client=611&conn=idle")
	writeBinary(t, conn, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, conn)
	handoffAuthoritativeHTTPDocument(t, ctx, store, resolver, key, "lease-node-a", "node-b", 2, "lease-node-b")

	closeErr := readCloseError(t, conn)
	if closeErr.Code != websocket.StatusTryAgainLater {
		t.Fatalf("closeErr.Code = %d, want %d", closeErr.Code, websocket.StatusTryAgainLater)
	}
	if closeErr.Reason != authorityLostCloseReason {
		t.Fatalf("closeErr.Reason = %q, want %q", closeErr.Reason, authorityLostCloseReason)
	}

	waitForCondition(t, 2*time.Second, func() bool {
		snapshot := recorder.snapshot()
		return snapshot.authorityRevalidations[recordingAuthorityRevalidationKey{
			role:   authorityRevalidationRoleLocal,
			result: "error",
		}] == 1
	})
}

func newHTTPTestServer(t *testing.T, store storage.SnapshotStore) *httptest.Server {
	t.Helper()

	provider := yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store})
	handler, err := NewServer(ServerConfig{
		Provider:       provider,
		ResolveRequest: resolveTestRequest,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func resolveTestRequest(r *http.Request) (Request, error) {
	query := r.URL.Query()
	documentID := strings.TrimSpace(query.Get("doc"))
	if documentID == "" {
		return Request{}, errors.New("doc obrigatorio")
	}

	clientRaw := strings.TrimSpace(query.Get("client"))
	if clientRaw == "" {
		return Request{}, errors.New("client obrigatorio")
	}

	clientValue, err := strconv.ParseUint(clientRaw, 10, 32)
	if err != nil {
		return Request{}, err
	}

	return Request{
		DocumentKey: storage.DocumentKey{
			Namespace:  "tests",
			DocumentID: documentID,
		},
		ConnectionID:   strings.TrimSpace(query.Get("conn")),
		ClientID:       uint32(clientValue),
		PersistOnClose: query.Get("persist") == "1",
	}, nil
}

func dialWS(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testIOTimeout)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(rawURL, "http")
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial(%q) unexpected error: %v", wsURL, err)
	}
	t.Cleanup(func() {
		_ = conn.CloseNow()
	})
	return conn
}

func writeBinary(t *testing.T, conn *websocket.Conn, payload []byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testIOTimeout)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, payload); err != nil {
		t.Fatalf("conn.Write() unexpected error: %v", err)
	}
}

func readBinary(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testIOTimeout)
	defer cancel()

	msgType, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() unexpected error: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Fatalf("conn.Read() type = %v, want %v", msgType, websocket.MessageBinary)
	}
	return payload
}

func waitForSnapshot(t *testing.T, store storage.SnapshotStore, key storage.DocumentKey) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		record, err := store.LoadSnapshot(context.Background(), key)
		if err == nil && record != nil && record.Snapshot != nil && !record.Snapshot.IsEmpty() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("snapshot for %v was not persisted before timeout", key)
}

func newAuthoritativeHTTPProvider(t *testing.T, localNode ycluster.NodeID) (*memory.Store, ycluster.ShardResolver, *yprotocol.Provider) {
	t.Helper()

	store := memory.New()
	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	lookup, err := ycluster.NewStorageOwnerLookup(localNode, resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup(%s) unexpected error: %v", localNode, err)
	}

	provider := yprotocol.NewProvider(yprotocol.ProviderConfig{
		Store: store,
		ResolveAuthorityFence: func(ctx context.Context, key storage.DocumentKey) (*storage.AuthorityFence, error) {
			return ycluster.ResolveStorageAuthorityFence(ctx, lookup, key)
		},
	})
	return store, resolver, provider
}

func newAuthoritativeLocalHTTPServer(t *testing.T, localNode ycluster.NodeID, store *memory.Store) (*Server, ycluster.ShardResolver) {
	return newAuthoritativeLocalHTTPServerWithMetrics(t, localNode, store, nil)
}

func newAuthoritativeLocalHTTPServerWithMetrics(t *testing.T, localNode ycluster.NodeID, store *memory.Store, metrics Metrics) (*Server, ycluster.ShardResolver) {
	t.Helper()

	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	lookup, err := ycluster.NewStorageOwnerLookup(localNode, resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup(%s) unexpected error: %v", localNode, err)
	}
	handler, err := NewServer(ServerConfig{
		Provider: yprotocol.NewProvider(yprotocol.ProviderConfig{
			Store: store,
			ResolveAuthorityFence: func(ctx context.Context, key storage.DocumentKey) (*storage.AuthorityFence, error) {
				return ycluster.ResolveStorageAuthorityFence(ctx, lookup, key)
			},
		}),
		ResolveRequest:                resolveTestRequest,
		AuthorityRevalidationInterval: 10 * time.Millisecond,
		Metrics:                       metrics,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	return handler, resolver
}

func seedAuthoritativeHTTPDocument(
	t *testing.T,
	ctx context.Context,
	store *memory.Store,
	resolver ycluster.ShardResolver,
	key storage.DocumentKey,
	node ycluster.NodeID,
	epoch uint64,
	token string,
) {
	t.Helper()

	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard(%#v) unexpected error: %v", key, err)
	}
	if _, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     key,
		ShardID: ycluster.StorageShardID(shardID),
		Version: 1,
	}); err != nil {
		t.Fatalf("store.SavePlacement() unexpected error: %v", err)
	}
	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID: ycluster.StorageShardID(shardID),
		Owner: storage.OwnerInfo{
			NodeID: ycluster.StorageNodeID(node),
			Epoch:  epoch,
		},
		Token:      token,
		AcquiredAt: time.Now().UTC().Add(-time.Minute),
		ExpiresAt:  time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatalf("store.SaveLease() unexpected error: %v", err)
	}
}

func handoffAuthoritativeHTTPDocument(
	t *testing.T,
	ctx context.Context,
	store *memory.Store,
	resolver ycluster.ShardResolver,
	key storage.DocumentKey,
	oldToken string,
	nextNode ycluster.NodeID,
	nextEpoch uint64,
	nextToken string,
) {
	t.Helper()

	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard(%#v) unexpected error: %v", key, err)
	}
	if err := store.ReleaseLease(ctx, ycluster.StorageShardID(shardID), oldToken); err != nil {
		t.Fatalf("store.ReleaseLease() unexpected error: %v", err)
	}
	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID: ycluster.StorageShardID(shardID),
		Owner: storage.OwnerInfo{
			NodeID: ycluster.StorageNodeID(nextNode),
			Epoch:  nextEpoch,
		},
		Token:      nextToken,
		AcquiredAt: time.Now().UTC(),
		ExpiresAt:  time.Now().UTC().Add(2 * time.Hour),
	}); err != nil {
		t.Fatalf("store.SaveLease() unexpected error: %v", err)
	}
}

func buildGCOnlyUpdate(client, length uint32) []byte {
	update := varint.Append(nil, 1)
	update = varint.Append(update, 1)
	update = varint.Append(update, client)
	update = varint.Append(update, 0)
	update = append(update, 0)
	update = varint.Append(update, length)
	return append(update, yupdate.EncodeDeleteSetBlockV1(ytypes.NewDeleteSet())...)
}
