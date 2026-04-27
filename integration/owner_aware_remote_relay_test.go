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

	ownerServer := newTypedOwnerTestServer(t, "node-b", store)
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

func newTypedOwnerAwareTestServer(
	t *testing.T,
	localNode ycluster.NodeID,
	store *memory.Store,
	resolver ycluster.ShardResolver,
	dialer yhttp.RemoteOwnerDialer,
) *httptest.Server {
	t.Helper()

	localHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: resolveSmokeRequest,
		OnError: func(_ *http.Request, req yhttp.Request, err error) {
			t.Logf("edge[%s] doc=%s/%s conn=%s: %v", localNode, req.DocumentKey.Namespace, req.DocumentKey.DocumentID, req.ConnectionID, err)
		},
	})
	if err != nil {
		t.Fatalf("yhttp.NewServer() unexpected error: %v", err)
	}

	lookup, err := ycluster.NewStorageOwnerLookup(localNode, resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup(%s) unexpected error: %v", localNode, err)
	}

	cfg := yhttp.OwnerAwareServerConfig{
		Local:       localHandler,
		OwnerLookup: lookup,
	}
	if dialer != nil {
		forwardRemoteOwner, err := yhttp.NewRemoteOwnerForwardHandler(yhttp.RemoteOwnerForwardConfig{
			LocalNodeID: localNode,
			Dialer:      dialer,
			OnError: func(_ *http.Request, req yhttp.Request, err error) {
				t.Logf("forward[%s] doc=%s/%s conn=%s: %v", localNode, req.DocumentKey.Namespace, req.DocumentKey.DocumentID, req.ConnectionID, err)
			},
		})
		if err != nil {
			t.Fatalf("yhttp.NewRemoteOwnerForwardHandler() unexpected error: %v", err)
		}
		cfg.OnRemoteOwner = forwardRemoteOwner
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

func newTypedOwnerTestServer(t *testing.T, localNode ycluster.NodeID, store *memory.Store) *typedOwnerTestServer {
	t.Helper()

	localHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: resolveSmokeRequest,
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

func wsURL(rawURL string) string {
	return "ws" + strings.TrimPrefix(rawURL, "http")
}

type memoryRemoteOwnerDialer struct {
	t        *testing.T
	endpoint *yhttp.RemoteOwnerEndpoint
}

func (d *memoryRemoteOwnerDialer) DialRemoteOwner(ctx context.Context, _ yhttp.RemoteOwnerDialRequest) (yhttp.NodeMessageStream, error) {
	if d.endpoint == nil {
		return nil, errors.New("memory remote owner endpoint obrigatorio")
	}

	client, server := newMemoryNodeStreamPair()
	go func() {
		if err := d.endpoint.ServeNodeStream(ctx, server); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
			d.t.Logf("memory remote owner serve stream: %v", err)
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
