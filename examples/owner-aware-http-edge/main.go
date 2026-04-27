package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/ycluster"
	"yjs-go-bridge/pkg/yhttp"
	"yjs-go-bridge/pkg/yprotocol"
)

const (
	address       = "127.0.0.1:8080"
	remoteAddress = "127.0.0.1:9090"
	wsPath        = "/ws"
	shardCount    = 32
)

var (
	localNodeID  = ycluster.NodeID("node-a")
	remoteNodeID = ycluster.NodeID("node-b")
)

type demoApp struct {
	localNode       ycluster.NodeID
	docs            []demoDocument
	edge            *ownerAwareEdge
	remoteWSHandler http.Handler
}

type demoDocument struct {
	Key   storage.DocumentKey
	Shard ycluster.ShardID
	Owner ycluster.NodeID
	Local bool
}

type ownerAwareEdge struct {
	ownerLookup ycluster.OwnerLookup
	wsHandler   http.Handler
	ownerRoutes map[ycluster.NodeID]string
}

type ownerRouteResponse struct {
	Namespace      string    `json:"namespace"`
	DocumentID     string    `json:"document_id"`
	Shard          string    `json:"shard"`
	OwnerNode      string    `json:"owner_node"`
	OwnerEpoch     uint64    `json:"owner_epoch,omitempty"`
	Local          bool      `json:"local"`
	LeaseToken     string    `json:"lease_token,omitempty"`
	LeaseExpiresAt time.Time `json:"lease_expires_at,omitempty"`
	WebSocketURL   string    `json:"websocket_url"`
	Note           string    `json:"note,omitempty"`
}

func main() {
	ctx := context.Background()

	app, err := newDemoApp(ctx)
	if err != nil {
		log.Fatalf("owner-aware-http-edge: init: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(wsPath, app.edge)
	mux.HandleFunc("/owner", app.handleOwner)
	mux.HandleFunc("/", app.handleRoot)

	remoteMux := http.NewServeMux()
	remoteMux.Handle(wsPath, app.remoteWSHandler)
	remoteMux.HandleFunc("/", app.handleRemoteRoot)

	go func() {
		log.Printf("owner-aware-http-edge: owner remoto ouvindo em http://%s\n", remoteAddress)
		if err := http.ListenAndServe(":9090", remoteMux); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("owner-aware-http-edge: servidor http remoto: %v", err)
		}
	}()

	log.Printf("owner-aware-http-edge: edge ouvindo em http://%s\n", address)
	for _, doc := range app.docs {
		scope := "remote"
		if doc.Local {
			scope = "local"
		}
		ownerRoute := withRawQuery(app.edge.ownerRoutes[doc.Owner], fmt.Sprintf("doc=%s&client=101&persist=1", doc.Key.DocumentID))
		log.Printf(
			"owner-aware-http-edge: %s doc=%s shard=%s owner=%s edge=ws://%s%s?doc=%s&client=101&persist=1 owner-route=%s\n",
			scope,
			doc.Key.DocumentID,
			doc.Shard,
			doc.Owner,
			address,
			wsPath,
			doc.Key.DocumentID,
			ownerRoute,
		)
	}

	if err := http.ListenAndServe(":8080", mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("owner-aware-http-edge: servidor http: %v", err)
	}
}

func newDemoApp(ctx context.Context) (*demoApp, error) {
	localNode := ycluster.StaticLocalNode{ID: localNodeID}
	if err := localNode.Validate(); err != nil {
		return nil, err
	}

	resolver, err := ycluster.NewDeterministicShardResolver(shardCount)
	if err != nil {
		return nil, err
	}

	store := memory.New()

	leaseStore, err := ycluster.NewStorageLeaseStore(store)
	if err != nil {
		return nil, err
	}

	docs, err := seedDemoDocuments(ctx, store, leaseStore, resolver)
	if err != nil {
		return nil, err
	}

	ownerRoutes := map[ycluster.NodeID]string{
		localNode.LocalNodeID(): fmt.Sprintf("ws://%s%s", address, wsPath),
		remoteNodeID:            fmt.Sprintf("ws://%s%s", remoteAddress, wsPath),
	}

	edgeWSHandler, ownerLookup, err := newOwnerAwareWSHandler(localNode.LocalNodeID(), store, resolver, ownerRoutes)
	if err != nil {
		return nil, err
	}
	remoteWSHandler, _, err := newOwnerAwareWSHandler(remoteNodeID, store, resolver, nil)
	if err != nil {
		return nil, err
	}

	sort.Slice(docs, func(i, j int) bool {
		if docs[i].Local != docs[j].Local {
			return docs[i].Local
		}
		return docs[i].Key.DocumentID < docs[j].Key.DocumentID
	})

	return &demoApp{
		localNode: localNode.LocalNodeID(),
		docs:      docs,
		edge: &ownerAwareEdge{
			ownerLookup: ownerLookup,
			wsHandler:   edgeWSHandler,
			ownerRoutes: ownerRoutes,
		},
		remoteWSHandler: remoteWSHandler,
	}, nil
}

func seedDemoDocuments(
	ctx context.Context,
	store *memory.Store,
	leaseStore ycluster.LeaseStore,
	resolver ycluster.ShardResolver,
) ([]demoDocument, error) {
	localKey, remoteKey, err := selectDemoKeys(resolver)
	if err != nil {
		return nil, err
	}

	localDoc, err := seedDocument(ctx, store, leaseStore, resolver, localKey, localNodeID, "lease-node-a", 1)
	if err != nil {
		return nil, err
	}
	remoteDoc, err := seedDocument(ctx, store, leaseStore, resolver, remoteKey, remoteNodeID, "lease-node-b", 2)
	if err != nil {
		return nil, err
	}

	return []demoDocument{localDoc, remoteDoc}, nil
}

func seedDocument(
	ctx context.Context,
	store *memory.Store,
	leaseStore ycluster.LeaseStore,
	resolver ycluster.ShardResolver,
	key storage.DocumentKey,
	owner ycluster.NodeID,
	token string,
	version uint64,
) (demoDocument, error) {
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		return demoDocument{}, err
	}

	if _, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:       key,
		ShardID:   ycluster.StorageShardID(shardID),
		Version:   version,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return demoDocument{}, err
	}

	if _, err := leaseStore.AcquireLease(ctx, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  owner,
		TTL:     30 * time.Minute,
		Token:   token,
	}); err != nil {
		return demoDocument{}, err
	}

	return demoDocument{
		Key:   key,
		Shard: shardID,
		Owner: owner,
		Local: owner == localNodeID,
	}, nil
}

func selectDemoKeys(resolver ycluster.ShardResolver) (storage.DocumentKey, storage.DocumentKey, error) {
	localKey := storage.DocumentKey{
		Namespace:  "examples",
		DocumentID: "notes-local",
	}
	localShard, err := resolver.ResolveShard(localKey)
	if err != nil {
		return storage.DocumentKey{}, storage.DocumentKey{}, err
	}

	remoteCandidates := []string{
		"notes-remote",
		"notes-remote-1",
		"notes-remote-2",
		"notes-remote-3",
	}
	for _, candidate := range remoteCandidates {
		remoteKey := storage.DocumentKey{
			Namespace:  "examples",
			DocumentID: candidate,
		}
		remoteShard, err := resolver.ResolveShard(remoteKey)
		if err != nil {
			return storage.DocumentKey{}, storage.DocumentKey{}, err
		}
		if remoteShard != localShard {
			return localKey, remoteKey, nil
		}
	}

	return storage.DocumentKey{}, storage.DocumentKey{}, fmt.Errorf("nao foi possivel selecionar documentos em shards distintos")
}

func newOwnerAwareWSHandler(
	localNode ycluster.NodeID,
	store *memory.Store,
	resolver ycluster.ShardResolver,
	ownerRoutes map[ycluster.NodeID]string,
) (http.Handler, ycluster.OwnerLookup, error) {
	localWSHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: resolveWSRequest,
	})
	if err != nil {
		return nil, nil, err
	}

	ownerLookup, err := ycluster.NewStorageOwnerLookup(localNode, resolver, store, store)
	if err != nil {
		return nil, nil, err
	}

	cfg := yhttp.OwnerAwareServerConfig{
		Local:       localWSHandler,
		OwnerLookup: ownerLookup,
	}
	if len(ownerRoutes) > 0 {
		cfg.OnRemoteOwner = relayRemoteOwnerHandler(ownerRoutes)
	}

	handler, err := yhttp.NewOwnerAwareServer(cfg)
	if err != nil {
		return nil, nil, err
	}
	return handler, ownerLookup, nil
}

func (a *demoApp) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	lines := []string{
		"owner-aware-http-edge",
		"",
		fmt.Sprintf("local node: %s", a.localNode),
		fmt.Sprintf("remote owner server: ws://%s%s", remoteAddress, wsPath),
		"",
		"owner resolution samples:",
	}
	for _, doc := range a.docs {
		label := "remote owner relay"
		if doc.Local {
			label = "local owner"
		}
		ownerRoute := withRawQuery(a.edge.ownerRoutes[doc.Owner], fmt.Sprintf("doc=%s&client=101&persist=1", doc.Key.DocumentID))
		lines = append(lines,
			fmt.Sprintf("- %s: doc=%s shard=%s owner=%s", label, doc.Key.DocumentID, doc.Shard, doc.Owner),
			fmt.Sprintf("  http://%s/owner?doc=%s&client=101&persist=1", address, doc.Key.DocumentID),
			fmt.Sprintf("  edge ws://%s%s?doc=%s&client=101&persist=1", address, wsPath, doc.Key.DocumentID),
			fmt.Sprintf("  owner %s", ownerRoute),
		)
	}
	lines = append(lines,
		"",
		"Edge /ws uses yhttp.OwnerAwareServer plus OnRemoteOwner relay.",
		"Remote-owner documents are proxied to node-b instead of stopping at route metadata.",
	)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(strings.Join(lines, "\n") + "\n"))
}

func (a *demoApp) handleRemoteRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("owner-aware-http-edge remote owner node-b\n"))
}

func (a *demoApp) handleOwner(w http.ResponseWriter, r *http.Request) {
	key, err := documentKeyFromQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, err := a.edge.routeForRequest(r.Context(), key, r.URL.RawQuery)
	if err != nil {
		http.Error(w, err.Error(), ownerLookupStatus(err))
		return
	}
	if response.Local {
		response.Note = "este no ja pode materializar o room localmente"
	} else {
		response.Note = "o edge encaminha o websocket ao no owner remoto via relay"
	}

	if err := writeJSON(w, http.StatusOK, response); err != nil {
		log.Printf("owner-aware-http-edge: write owner response: %v", err)
	}
}

func (h *ownerAwareEdge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.wsHandler.ServeHTTP(w, r)
}

func (h *ownerAwareEdge) routeForRequest(
	ctx context.Context,
	key storage.DocumentKey,
	rawQuery string,
) (*ownerRouteResponse, error) {
	resolution, err := h.ownerLookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		return nil, err
	}

	response := &ownerRouteResponse{
		Namespace:    key.Namespace,
		DocumentID:   key.DocumentID,
		Shard:        resolution.Placement.ShardID.String(),
		OwnerNode:    resolution.Placement.NodeID.String(),
		Local:        resolution.Local,
		WebSocketURL: withRawQuery(h.ownerRoutes[resolution.Placement.NodeID], rawQuery),
	}
	if resolution.Placement.Lease != nil {
		response.OwnerEpoch = resolution.Placement.Lease.Epoch
		response.LeaseToken = resolution.Placement.Lease.Token
		response.LeaseExpiresAt = resolution.Placement.Lease.ExpiresAt
	}
	return response, nil
}

func resolveWSRequest(r *http.Request) (yhttp.Request, error) {
	key, err := documentKeyFromQuery(r)
	if err != nil {
		return yhttp.Request{}, err
	}

	clientRaw := strings.TrimSpace(r.URL.Query().Get("client"))
	if clientRaw == "" {
		return yhttp.Request{}, errors.New("client obrigatorio")
	}

	clientValue, err := strconv.ParseUint(clientRaw, 10, 32)
	if err != nil {
		return yhttp.Request{}, fmt.Errorf("client invalido: %w", err)
	}

	return yhttp.Request{
		DocumentKey:    key,
		ConnectionID:   strings.TrimSpace(r.URL.Query().Get("conn")),
		ClientID:       uint32(clientValue),
		PersistOnClose: r.URL.Query().Get("persist") == "1",
	}, nil
}

func documentKeyFromQuery(r *http.Request) (storage.DocumentKey, error) {
	documentID := strings.TrimSpace(r.URL.Query().Get("doc"))
	if documentID == "" {
		return storage.DocumentKey{}, errors.New("doc obrigatorio")
	}

	return storage.DocumentKey{
		Namespace:  "examples",
		DocumentID: documentID,
	}, nil
}

func ownerLookupStatus(err error) int {
	switch {
	case errors.Is(err, ycluster.ErrPlacementNotFound), errors.Is(err, ycluster.ErrOwnerNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func relayRemoteOwnerHandler(ownerRoutes map[ycluster.NodeID]string) yhttp.RemoteOwnerHandler {
	return func(w http.ResponseWriter, r *http.Request, _ yhttp.Request, resolution ycluster.OwnerResolution) bool {
		targetURL := withRawQuery(ownerRoutes[resolution.Placement.NodeID], r.URL.RawQuery)
		if err := relayWebSocketBinary(w, r, targetURL); err != nil && !isExpectedRelayClose(err) {
			log.Printf("owner-aware-http-edge: relay remote owner %s: %v", resolution.Placement.NodeID, err)
		}
		return true
	}
}

func relayWebSocketBinary(w http.ResponseWriter, r *http.Request, targetURL string) error {
	if strings.TrimSpace(targetURL) == "" {
		http.Error(w, "owner route indisponivel", http.StatusBadGateway)
		return nil
	}

	upstream, _, err := websocket.Dial(r.Context(), targetURL, nil)
	if err != nil {
		http.Error(w, "relay dial remoto: "+err.Error(), http.StatusBadGateway)
		return err
	}
	defer upstream.CloseNow()

	downstream, err := websocket.Accept(w, r, nil)
	if err != nil {
		return err
	}
	defer downstream.CloseNow()

	relayCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

	upstreamConn := websocket.NetConn(relayCtx, upstream, websocket.MessageBinary)
	downstreamConn := websocket.NetConn(relayCtx, downstream, websocket.MessageBinary)
	defer upstreamConn.Close()
	defer downstreamConn.Close()

	errCh := make(chan error, 2)
	go relayCopy(errCh, upstreamConn, downstreamConn)
	go relayCopy(errCh, downstreamConn, upstreamConn)

	firstErr := <-errCh
	cancel()
	_ = upstreamConn.Close()
	_ = downstreamConn.Close()
	secondErr := <-errCh
	if isExpectedRelayClose(firstErr) {
		return secondErr
	}
	return firstErr
}

func relayCopy(errCh chan<- error, dst net.Conn, src net.Conn) {
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

func withRawQuery(baseURL string, rawQuery string) string {
	if strings.TrimSpace(baseURL) == "" {
		return ""
	}
	if strings.TrimSpace(rawQuery) == "" {
		return baseURL
	}
	return baseURL + "?" + rawQuery
}

func writeJSON(w http.ResponseWriter, status int, payload any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
