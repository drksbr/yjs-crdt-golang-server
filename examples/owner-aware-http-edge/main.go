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

	prometheuslib "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	storageprometheus "github.com/drksbr/yjs-crdt-golang-server/pkg/storage/prometheus"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	yclusterprometheus "github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster/prometheus"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp"
	yhttpprometheus "github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp/prometheus"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
	yprotocolprometheus "github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol/prometheus"
)

const (
	address                  = "127.0.0.1:8080"
	remoteAddress            = "127.0.0.1:9090"
	wsPath                   = "/ws"
	nodePath                 = "/node"
	shardCount               = 32
	ownershipTTL             = 30 * time.Minute
	ownershipRenewWithin     = 10 * time.Minute
	ownershipCheckInterval   = 30 * time.Second
	ownershipShutdownTimeout = 5 * time.Second
)

var (
	localNodeID  = ycluster.NodeID("node-a")
	remoteNodeID = ycluster.NodeID("node-b")
)

type demoApp struct {
	localNode           ycluster.NodeID
	docs                []demoDocument
	ownershipHandles    []*ycluster.DocumentOwnershipHandle
	edgeMetrics         *demoMetrics
	remoteMetrics       *demoMetrics
	edge                *ownerAwareEdge
	remoteBrowserHandle http.Handler
	remoteNodeHandle    http.Handler
}

type demoMetrics struct {
	registry *prometheuslib.Registry
	storage  *storageprometheus.Metrics
	protocol *yprotocolprometheus.Metrics
	cluster  *yclusterprometheus.Metrics
	http     *yhttpprometheus.Metrics
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
	mux.Handle("/metrics", app.edgeMetrics.handler())
	mux.HandleFunc("/owner", app.handleOwner)
	mux.HandleFunc("/", app.handleRoot)

	remoteMux := http.NewServeMux()
	remoteMux.Handle(wsPath, app.remoteBrowserHandle)
	remoteMux.Handle(nodePath, app.remoteNodeHandle)
	remoteMux.Handle("/metrics", app.remoteMetrics.handler())
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
	edgeMetrics, err := newDemoMetrics(localNode.LocalNodeID(), "edge")
	if err != nil {
		return nil, err
	}
	remoteMetrics, err := newDemoMetrics(remoteNodeID, "owner")
	if err != nil {
		return nil, err
	}

	localOwnership, err := ycluster.NewStorageOwnershipCoordinator(ycluster.StorageOwnershipCoordinatorConfig{
		LocalNode:  localNode.LocalNodeID(),
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        ownershipTTL,
		Metrics:    edgeMetrics.cluster,
	})
	if err != nil {
		return nil, err
	}
	remoteOwnership, err := ycluster.NewStorageOwnershipCoordinator(ycluster.StorageOwnershipCoordinatorConfig{
		LocalNode:  remoteNodeID,
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        ownershipTTL,
		Metrics:    remoteMetrics.cluster,
	})
	if err != nil {
		return nil, err
	}

	docs, err := seedDemoDocuments(ctx, localOwnership, remoteOwnership, resolver)
	if err != nil {
		return nil, err
	}
	localOwnershipRuntime, err := newDemoOwnershipRuntime(localOwnership)
	if err != nil {
		return nil, err
	}
	remoteOwnershipRuntime, err := newDemoOwnershipRuntime(remoteOwnership)
	if err != nil {
		return nil, err
	}
	ownershipHandles, err := acquireDemoOwnerships(ctx, docs, localOwnershipRuntime, remoteOwnershipRuntime)
	if err != nil {
		return nil, err
	}

	ownerRoutes := map[ycluster.NodeID]string{
		localNode.LocalNodeID(): fmt.Sprintf("ws://%s%s", address, wsPath),
		remoteNodeID:            fmt.Sprintf("ws://%s%s", remoteAddress, wsPath),
	}
	nodeRoutes := map[ycluster.NodeID]string{
		remoteNodeID: fmt.Sprintf("ws://%s%s", remoteAddress, nodePath),
	}

	edgeWSHandler, ownerLookup, err := newOwnerAwareWSHandler(localNode.LocalNodeID(), store, localOwnership, localOwnershipRuntime, nodeRoutes, edgeMetrics)
	if err != nil {
		return nil, err
	}
	remoteBrowserHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider: yprotocol.NewProvider(yprotocol.ProviderConfig{
			Store:                 store,
			ResolveAuthorityFence: remoteOwnership.ResolveAuthorityFence,
			Metrics:               remoteMetrics.protocol,
			StorageMetrics:        remoteMetrics.storage,
		}),
		OwnershipRuntime: remoteOwnershipRuntime,
		ResolveRequest:   resolveWSRequest,
		Metrics:          remoteMetrics.http,
	})
	if err != nil {
		return nil, err
	}
	remoteNodeHandler, err := yhttp.NewRemoteOwnerEndpoint(yhttp.RemoteOwnerEndpointConfig{
		Local:       remoteBrowserHandler,
		LocalNodeID: remoteNodeID,
		Authenticate: func(_ context.Context, req yhttp.RemoteOwnerAuthRequest) error {
			if req.NodeID != localNode.LocalNodeID() {
				return fmt.Errorf("remote owner rejeitou node %q", req.NodeID)
			}
			return nil
		},
	})
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
		localNode:        localNode.LocalNodeID(),
		docs:             docs,
		ownershipHandles: ownershipHandles,
		edgeMetrics:      edgeMetrics,
		remoteMetrics:    remoteMetrics,
		edge: &ownerAwareEdge{
			ownerLookup: ownerLookup,
			wsHandler:   edgeWSHandler,
			ownerRoutes: ownerRoutes,
		},
		remoteBrowserHandle: remoteBrowserHandler,
		remoteNodeHandle:    remoteNodeHandler,
	}, nil
}

func newDemoMetrics(nodeID ycluster.NodeID, deploymentRole string) (*demoMetrics, error) {
	registry := prometheuslib.NewRegistry()
	constLabels := prometheuslib.Labels{
		"deployment_role": strings.TrimSpace(deploymentRole),
		"env":             "demo",
		"node_id":         string(nodeID),
	}

	storageMetrics, err := storageprometheus.New(storageprometheus.Config{
		Registerer:  registry,
		ConstLabels: constLabels,
	})
	if err != nil {
		return nil, fmt.Errorf("storage metrics: %w", err)
	}
	protocolMetrics, err := yprotocolprometheus.New(yprotocolprometheus.Config{
		Registerer:  registry,
		ConstLabels: constLabels,
	})
	if err != nil {
		return nil, fmt.Errorf("protocol metrics: %w", err)
	}
	clusterMetrics, err := yclusterprometheus.New(yclusterprometheus.Config{
		Registerer:  registry,
		ConstLabels: constLabels,
	})
	if err != nil {
		return nil, fmt.Errorf("cluster metrics: %w", err)
	}
	httpMetrics, err := yhttpprometheus.New(yhttpprometheus.Config{
		Registerer:  registry,
		ConstLabels: constLabels,
	})
	if err != nil {
		return nil, fmt.Errorf("http metrics: %w", err)
	}

	return &demoMetrics{
		registry: registry,
		storage:  storageMetrics,
		protocol: protocolMetrics,
		cluster:  clusterMetrics,
		http:     httpMetrics,
	}, nil
}

func (m *demoMetrics) handler() http.Handler {
	if m == nil || m.registry == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func seedDemoDocuments(
	ctx context.Context,
	localOwnership *ycluster.StorageOwnershipCoordinator,
	remoteOwnership *ycluster.StorageOwnershipCoordinator,
	resolver ycluster.ShardResolver,
) ([]demoDocument, error) {
	localKey, remoteKey, err := selectDemoKeys(resolver)
	if err != nil {
		return nil, err
	}

	localDoc, err := seedDocument(ctx, localOwnership, localKey, localNodeID, "lease-node-a", 1)
	if err != nil {
		return nil, err
	}
	remoteDoc, err := seedDocument(ctx, remoteOwnership, remoteKey, remoteNodeID, "lease-node-b", 2)
	if err != nil {
		return nil, err
	}

	return []demoDocument{localDoc, remoteDoc}, nil
}

func newDemoOwnershipRuntime(ownership *ycluster.StorageOwnershipCoordinator) (*ycluster.DocumentOwnershipRuntime, error) {
	return ycluster.NewDocumentOwnershipRuntime(ycluster.DocumentOwnershipRuntimeConfig{
		Coordinator: ownership,
		Lease: ycluster.LeaseManagerRunConfig{
			RenewWithin: ownershipRenewWithin,
			Interval:    ownershipCheckInterval,
		},
		ReleaseTimeout: ownershipShutdownTimeout,
	})
}

func acquireDemoOwnerships(
	ctx context.Context,
	docs []demoDocument,
	localRuntime *ycluster.DocumentOwnershipRuntime,
	remoteRuntime *ycluster.DocumentOwnershipRuntime,
) ([]*ycluster.DocumentOwnershipHandle, error) {
	handles := make([]*ycluster.DocumentOwnershipHandle, 0, len(docs))
	for _, doc := range docs {
		runtime := remoteRuntime
		if doc.Local {
			runtime = localRuntime
		}
		handle, err := runtime.AcquireDocumentOwnership(ctx, ycluster.ClaimDocumentRequest{
			DocumentKey: doc.Key,
		})
		if err != nil {
			releaseDemoOwnerships(handles)
			return nil, err
		}
		handles = append(handles, handle)
	}
	return handles, nil
}

func releaseDemoOwnerships(handles []*ycluster.DocumentOwnershipHandle) {
	ctx, cancel := context.WithTimeout(context.Background(), ownershipShutdownTimeout)
	defer cancel()
	for _, handle := range handles {
		if err := handle.Release(ctx); err != nil {
			log.Printf("owner-aware-http-edge: release demo ownership: %v", err)
		}
	}
}

func seedDocument(
	ctx context.Context,
	ownership *ycluster.StorageOwnershipCoordinator,
	key storage.DocumentKey,
	owner ycluster.NodeID,
	token string,
	version uint64,
) (demoDocument, error) {
	claimed, err := ownership.ClaimDocument(ctx, ycluster.ClaimDocumentRequest{
		DocumentKey:      key,
		Holder:           owner,
		Token:            token,
		PlacementVersion: version,
	})
	if err != nil {
		return demoDocument{}, err
	}

	return demoDocument{
		Key:   key,
		Shard: claimed.ShardID,
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
	ownership *ycluster.StorageOwnershipCoordinator,
	ownershipRuntime *ycluster.DocumentOwnershipRuntime,
	nodeRoutes map[ycluster.NodeID]string,
	metrics *demoMetrics,
) (http.Handler, ycluster.OwnerLookup, error) {
	localWSHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider: yprotocol.NewProvider(yprotocol.ProviderConfig{
			Store:                 store,
			ResolveAuthorityFence: ownership.ResolveAuthorityFence,
			Metrics:               metrics.protocol,
			StorageMetrics:        metrics.storage,
		}),
		OwnershipRuntime: ownershipRuntime,
		ResolveRequest:   resolveWSRequest,
		Metrics:          metrics.http,
	})
	if err != nil {
		return nil, nil, err
	}

	cfg := yhttp.OwnerAwareServerConfig{
		Local:                          localWSHandler,
		OwnerLookup:                    ownership,
		PromoteLocalOnOwnerUnavailable: true,
	}
	if len(nodeRoutes) > 0 {
		dialer, err := yhttp.NewWebSocketRemoteOwnerDialer(yhttp.WebSocketRemoteOwnerDialerConfig{
			ResolveURL: func(_ context.Context, req yhttp.RemoteOwnerDialRequest) (string, error) {
				targetURL := strings.TrimSpace(nodeRoutes[req.Resolution.Placement.NodeID])
				if targetURL == "" {
					return "", errors.New("typed remote owner route indisponivel")
				}
				return targetURL, nil
			},
		})
		if err != nil {
			return nil, nil, err
		}
		cfg.OnRemoteOwner, cfg.OnLocalAuthorityLost, err = yhttp.NewRemoteOwnerForwardHandlers(yhttp.RemoteOwnerForwardConfig{
			LocalNodeID: localNode,
			Local:       localWSHandler,
			Dialer:      dialer,
			OwnerLookup: ownership,
			Metrics:     metrics.http,
		})
		if err != nil {
			return nil, nil, err
		}
	}

	handler, err := yhttp.NewOwnerAwareServer(cfg)
	if err != nil {
		return nil, nil, err
	}
	return handler, ownership, nil
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
		fmt.Sprintf("active demo ownership handles: %d", len(a.ownershipHandles)),
		fmt.Sprintf("remote owner browser ws: ws://%s%s", remoteAddress, wsPath),
		fmt.Sprintf("remote owner node endpoint: ws://%s%s", remoteAddress, nodePath),
		fmt.Sprintf("edge metrics: http://%s/metrics", address),
		fmt.Sprintf("remote metrics: http://%s/metrics", remoteAddress),
		"",
		"owner resolution samples:",
	}
	for _, doc := range a.docs {
		label := "remote owner proxy"
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
		"Edge /ws uses yhttp.OwnerAwareServer plus RemoteOwnerForwardHandler.",
		"Unknown documents can be promoted locally when no active owner exists.",
		"Remote-owner documents are proxied to node-b through the typed /node endpoint.",
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
	_, _ = w.Write([]byte("owner-aware-http-edge remote owner node-b (/ws browser, /node inter-node)\n"))
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
		response.Note = "o edge encaminha o websocket ao no owner remoto via endpoint tipado /node"
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
