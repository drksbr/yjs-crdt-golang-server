package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/ycluster"
	"yjs-go-bridge/pkg/yhttp"
	"yjs-go-bridge/pkg/yprotocol"
)

const (
	address    = "127.0.0.1:8080"
	wsPath     = "/ws"
	shardCount = 32
)

var (
	localNodeID  = ycluster.NodeID("node-a")
	remoteNodeID = ycluster.NodeID("node-b")
)

type demoApp struct {
	localNode ycluster.NodeID
	docs      []demoDocument
	edge      *ownerAwareEdge
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

	log.Printf("owner-aware-http-edge: ouvindo em http://%s\n", address)
	for _, doc := range app.docs {
		scope := "remote"
		if doc.Local {
			scope = "local"
		}
		log.Printf(
			"owner-aware-http-edge: %s doc=%s shard=%s owner=%s ws=ws://%s%s?doc=%s&client=101&persist=1\n",
			scope,
			doc.Key.DocumentID,
			doc.Shard,
			doc.Owner,
			address,
			wsPath,
			doc.Key.DocumentID,
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
	provider := yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store})
	wsHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       provider,
		ResolveRequest: resolveWSRequest,
	})
	if err != nil {
		return nil, err
	}

	leaseStore, err := ycluster.NewStorageLeaseStore(store)
	if err != nil {
		return nil, err
	}

	docs, err := seedDemoDocuments(ctx, store, leaseStore, resolver)
	if err != nil {
		return nil, err
	}

	ownerLookup, err := ycluster.NewStorageOwnerLookup(localNode.LocalNodeID(), resolver, store, store)
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
			wsHandler:   wsHandler,
			ownerRoutes: map[ycluster.NodeID]string{
				localNode.LocalNodeID(): fmt.Sprintf("ws://%s%s", address, wsPath),
				remoteNodeID:            "ws://127.0.0.1:9090/ws",
			},
		},
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

func (a *demoApp) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	lines := []string{
		"owner-aware-http-edge",
		"",
		fmt.Sprintf("local node: %s", a.localNode),
		"",
		"owner resolution samples:",
	}
	for _, doc := range a.docs {
		label := "remote route hint"
		if doc.Local {
			label = "local owner"
		}
		lines = append(lines,
			fmt.Sprintf("- %s: doc=%s shard=%s owner=%s", label, doc.Key.DocumentID, doc.Shard, doc.Owner),
			fmt.Sprintf("  http://%s/owner?doc=%s&client=101&persist=1", address, doc.Key.DocumentID),
			fmt.Sprintf("  ws://%s%s?doc=%s&client=101&persist=1", address, wsPath, doc.Key.DocumentID),
		)
	}
	lines = append(lines,
		"",
		"Only local-owner documents are handed to pkg/yhttp today.",
		"Remote-owner documents return route metadata instead of forwarding.",
	)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(strings.Join(lines, "\n") + "\n"))
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
		response.Note = "forwarding inter-node ainda nao existe; este exemplo para na resolucao de owner"
	}

	if err := writeJSON(w, http.StatusOK, response); err != nil {
		log.Printf("owner-aware-http-edge: write owner response: %v", err)
	}
}

func (h *ownerAwareEdge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := resolveWSRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, err := h.routeForRequest(r.Context(), req.DocumentKey, r.URL.RawQuery)
	if err != nil {
		http.Error(w, err.Error(), ownerLookupStatus(err))
		return
	}
	if response.Local {
		h.wsHandler.ServeHTTP(w, r)
		return
	}

	response.Note = "owner remoto resolvido; o room nao e materializado neste no"
	w.Header().Set("X-Yjs-Owner-Node", response.OwnerNode)
	if response.WebSocketURL != "" {
		w.Header().Set("X-Yjs-Owner-Websocket", response.WebSocketURL)
	}
	if err := writeJSON(w, http.StatusMisdirectedRequest, response); err != nil {
		log.Printf("owner-aware-http-edge: write remote owner response: %v", err)
	}
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
