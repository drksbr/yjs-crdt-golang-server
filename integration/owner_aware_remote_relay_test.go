package integration

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/ycluster"
	"yjs-go-bridge/pkg/yhttp"
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

	ownerServer := newRelayOwnerAwareTestServer(t, "node-b", store, resolver, nil)
	edgeServer := newRelayOwnerAwareTestServer(t, "node-a", store, resolver, map[ycluster.NodeID]string{
		"node-b": wsURL(ownerServer.URL + "/ws"),
	})

	ownerPeer := dialSmokeWS(t, ownerServer.URL+"/ws?doc=owner-aware-remote-relay&client=801&conn=owner")
	seedUpdate := buildIntegrationGCOnlyUpdate(91, 2)
	writeSmokeBinary(t, ownerPeer, yprotocol.EncodeProtocolSyncUpdate(seedUpdate))

	edgePeer := dialSmokeWS(t, edgeServer.URL+"/ws?doc=owner-aware-remote-relay&client=802&conn=edge")
	writeSmokeBinary(t, edgePeer, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	assertSyncStep2MatchesUpdate(t, readSmokeBinary(t, edgePeer), seedUpdate)

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

func newRelayOwnerAwareTestServer(
	t *testing.T,
	localNode ycluster.NodeID,
	store *memory.Store,
	resolver ycluster.ShardResolver,
	ownerRoutes map[ycluster.NodeID]string,
) *httptest.Server {
	t.Helper()

	localHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: resolveSmokeRequest,
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
	if len(ownerRoutes) > 0 {
		cfg.OnRemoteOwner = relayRemoteOwnerHandler(ownerRoutes)
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

func relayRemoteOwnerHandler(ownerRoutes map[ycluster.NodeID]string) yhttp.RemoteOwnerHandler {
	return func(w http.ResponseWriter, r *http.Request, _ yhttp.Request, resolution ycluster.OwnerResolution) bool {
		targetURL := strings.TrimSpace(ownerRoutes[resolution.Placement.NodeID])
		if targetURL == "" {
			http.Error(w, "remote owner route indisponivel", http.StatusBadGateway)
			return true
		}
		targetURL = withRawQuery(targetURL, r.URL.RawQuery)

		upstream, _, err := websocket.Dial(r.Context(), targetURL, nil)
		if err != nil {
			http.Error(w, "dial remote owner: "+err.Error(), http.StatusBadGateway)
			return true
		}
		defer upstream.CloseNow()

		downstream, err := websocket.Accept(w, r, nil)
		if err != nil {
			return true
		}
		defer downstream.CloseNow()

		relayCtx, cancel := context.WithCancel(r.Context())
		defer cancel()

		upstreamConn := websocket.NetConn(relayCtx, upstream, websocket.MessageBinary)
		downstreamConn := websocket.NetConn(relayCtx, downstream, websocket.MessageBinary)
		defer upstreamConn.Close()
		defer downstreamConn.Close()

		errCh := make(chan error, 2)
		go copyRelayTraffic(errCh, upstreamConn, downstreamConn)
		go copyRelayTraffic(errCh, downstreamConn, upstreamConn)

		firstErr := <-errCh
		cancel()
		_ = upstreamConn.Close()
		_ = downstreamConn.Close()
		secondErr := <-errCh

		if isExpectedRelayClose(firstErr) || isExpectedRelayClose(secondErr) {
			return true
		}
		return true
	}
}

func copyRelayTraffic(errCh chan<- error, dst net.Conn, src net.Conn) {
	_, err := io.Copy(dst, src)
	errCh <- err
}

func isExpectedRelayClose(err error) bool {
	switch {
	case err == nil, errors.Is(err, io.EOF), errors.Is(err, net.ErrClosed):
		return true
	default:
		var closeErr websocket.CloseError
		if errors.As(err, &closeErr) {
			return closeErr.Code == websocket.StatusNormalClosure || closeErr.Code == websocket.StatusGoingAway
		}
		return false
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

func wsURL(rawURL string) string {
	return "ws" + strings.TrimPrefix(rawURL, "http")
}

func withRawQuery(baseURL string, rawQuery string) string {
	if strings.TrimSpace(baseURL) == "" {
		return ""
	}
	if strings.TrimSpace(rawQuery) == "" {
		return baseURL
	}
	return baseURL + "?" + rawQuery
}
