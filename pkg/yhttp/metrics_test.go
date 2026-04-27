package yhttp

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/yprotocol"
)

func TestHTTPServerInvokesMetricsHooks(t *testing.T) {
	t.Parallel()

	recorder := newRecordingMetrics()
	store := memory.New()

	provider := yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store})
	handler, err := NewServer(ServerConfig{
		Provider:       provider,
		ResolveRequest: resolveTestRequest,
		Metrics:        recorder,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}

	srv := newHTTPTestServerWithHandler(t, handler)
	left := dialWS(t, srv.URL+"/ws?doc=metrics-room&client=601&conn=left&persist=1")
	right := dialWS(t, srv.URL+"/ws?doc=metrics-room&client=602&conn=right&persist=1")

	writeBinary(t, left, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, left)
	writeBinary(t, right, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	_ = readBinary(t, right)

	update := buildGCOnlyUpdate(77, 3)
	writeBinary(t, left, yprotocol.EncodeProtocolSyncUpdate(update))
	_ = readBinary(t, right)

	if err := left.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("left.Close() unexpected error: %v", err)
	}
	if err := right.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("right.Close() unexpected error: %v", err)
	}

	waitForSnapshot(t, store, testDocumentKey("metrics-room"))
	waitForCondition(t, 2*time.Second, func() bool {
		snapshot := recorder.snapshot()
		return snapshot.opened == 2 && snapshot.closed == 2 && snapshot.persistCalls == 2
	})

	snapshot := recorder.snapshot()
	if snapshot.frameReads != 3 {
		t.Fatalf("frameReads = %d, want 3", snapshot.frameReads)
	}
	if snapshot.frameWrites["direct"] != 2 {
		t.Fatalf("frameWrites[direct] = %d, want 2", snapshot.frameWrites["direct"])
	}
	if snapshot.frameWrites["broadcast"] != 1 {
		t.Fatalf("frameWrites[broadcast] = %d, want 1", snapshot.frameWrites["broadcast"])
	}
	if snapshot.handleCalls != 3 {
		t.Fatalf("handleCalls = %d, want 3", snapshot.handleCalls)
	}
	if len(snapshot.errorStages) != 0 {
		t.Fatalf("errorStages = %v, want none", snapshot.errorStages)
	}
}

type recordingMetrics struct {
	mu                          sync.Mutex
	opened                      int
	closed                      int
	frameReads                  int
	frameWrites                 map[string]int
	handleCalls                 int
	persistCalls                int
	errorStages                 []string
	ownerLookupResults          map[string]int
	routeDecisions              map[string]int
	remoteOwnerConnectionsOpen  map[string]int
	remoteOwnerConnectionsClose map[string]int
	remoteOwnerHandshakes       map[recordingRemoteOwnerHandshakeKey]int
	remoteOwnerMessages         map[recordingRemoteOwnerMessageKey]int
	remoteOwnerCloses           map[recordingRemoteOwnerCloseKey]int
	authorityRevalidations      map[recordingAuthorityRevalidationKey]int
	ownershipTransitions        map[recordingOwnershipTransitionKey]int
}

type recordingMetricsSnapshot struct {
	opened                      int
	closed                      int
	frameReads                  int
	frameWrites                 map[string]int
	handleCalls                 int
	persistCalls                int
	errorStages                 []string
	ownerLookupResults          map[string]int
	routeDecisions              map[string]int
	remoteOwnerConnectionsOpen  map[string]int
	remoteOwnerConnectionsClose map[string]int
	remoteOwnerHandshakes       map[recordingRemoteOwnerHandshakeKey]int
	remoteOwnerMessages         map[recordingRemoteOwnerMessageKey]int
	remoteOwnerCloses           map[recordingRemoteOwnerCloseKey]int
	authorityRevalidations      map[recordingAuthorityRevalidationKey]int
	ownershipTransitions        map[recordingOwnershipTransitionKey]int
}

func newRecordingMetrics() *recordingMetrics {
	return &recordingMetrics{
		frameWrites:                 make(map[string]int),
		ownerLookupResults:          make(map[string]int),
		routeDecisions:              make(map[string]int),
		remoteOwnerConnectionsOpen:  make(map[string]int),
		remoteOwnerConnectionsClose: make(map[string]int),
		remoteOwnerHandshakes:       make(map[recordingRemoteOwnerHandshakeKey]int),
		remoteOwnerMessages:         make(map[recordingRemoteOwnerMessageKey]int),
		remoteOwnerCloses:           make(map[recordingRemoteOwnerCloseKey]int),
		authorityRevalidations:      make(map[recordingAuthorityRevalidationKey]int),
		ownershipTransitions:        make(map[recordingOwnershipTransitionKey]int),
	}
}

func (r *recordingMetrics) ConnectionOpened(Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.opened++
}

func (r *recordingMetrics) ConnectionClosed(Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed++
}

func (r *recordingMetrics) FrameRead(Request, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frameReads++
}

func (r *recordingMetrics) FrameWritten(_ Request, kind string, _ int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frameWrites[kind]++
}

func (r *recordingMetrics) Handle(Request, time.Duration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handleCalls++
}

func (r *recordingMetrics) Persist(Request, time.Duration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.persistCalls++
}

func (r *recordingMetrics) Error(_ Request, stage string, _ error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorStages = append(r.errorStages, stage)
}

func (r *recordingMetrics) OwnerLookup(_ Request, _ time.Duration, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ownerLookupResults[result]++
}

func (r *recordingMetrics) RouteDecision(_ Request, decision string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routeDecisions[decision]++
}

func (r *recordingMetrics) RemoteOwnerConnectionOpened(_ Request, role string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remoteOwnerConnectionsOpen[role]++
}

func (r *recordingMetrics) RemoteOwnerConnectionClosed(_ Request, role string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remoteOwnerConnectionsClose[role]++
}

func (r *recordingMetrics) RemoteOwnerHandshake(_ Request, role string, _ time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remoteOwnerHandshakes[recordingRemoteOwnerHandshakeKey{
		role:   role,
		result: recordingResultLabel(err),
	}]++
}

func (r *recordingMetrics) RemoteOwnerMessage(_ Request, role string, direction string, kind string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remoteOwnerMessages[recordingRemoteOwnerMessageKey{
		role:      role,
		direction: direction,
		kind:      kind,
	}]++
}

func (r *recordingMetrics) RemoteOwnerClose(_ Request, role string, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remoteOwnerCloses[recordingRemoteOwnerCloseKey{
		role:   role,
		reason: reason,
	}]++
}

func (r *recordingMetrics) AuthorityRevalidation(_ Request, role string, _ time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.authorityRevalidations[recordingAuthorityRevalidationKey{
		role:   role,
		result: recordingResultLabel(err),
	}]++
}

func (r *recordingMetrics) OwnershipTransition(_ Request, from string, to string, _ time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ownershipTransitions[recordingOwnershipTransitionKey{
		from:   from,
		to:     to,
		result: recordingResultLabel(err),
	}]++
}

func (r *recordingMetrics) snapshot() recordingMetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	frameWrites := make(map[string]int, len(r.frameWrites))
	for kind, total := range r.frameWrites {
		frameWrites[kind] = total
	}
	ownerLookupResults := make(map[string]int, len(r.ownerLookupResults))
	for result, total := range r.ownerLookupResults {
		ownerLookupResults[result] = total
	}
	routeDecisions := make(map[string]int, len(r.routeDecisions))
	for decision, total := range r.routeDecisions {
		routeDecisions[decision] = total
	}
	remoteOwnerConnectionsOpen := make(map[string]int, len(r.remoteOwnerConnectionsOpen))
	for role, total := range r.remoteOwnerConnectionsOpen {
		remoteOwnerConnectionsOpen[role] = total
	}
	remoteOwnerConnectionsClose := make(map[string]int, len(r.remoteOwnerConnectionsClose))
	for role, total := range r.remoteOwnerConnectionsClose {
		remoteOwnerConnectionsClose[role] = total
	}
	remoteOwnerHandshakes := make(map[recordingRemoteOwnerHandshakeKey]int, len(r.remoteOwnerHandshakes))
	for key, total := range r.remoteOwnerHandshakes {
		remoteOwnerHandshakes[key] = total
	}
	remoteOwnerMessages := make(map[recordingRemoteOwnerMessageKey]int, len(r.remoteOwnerMessages))
	for key, total := range r.remoteOwnerMessages {
		remoteOwnerMessages[key] = total
	}
	remoteOwnerCloses := make(map[recordingRemoteOwnerCloseKey]int, len(r.remoteOwnerCloses))
	for key, total := range r.remoteOwnerCloses {
		remoteOwnerCloses[key] = total
	}
	authorityRevalidations := make(map[recordingAuthorityRevalidationKey]int, len(r.authorityRevalidations))
	for key, total := range r.authorityRevalidations {
		authorityRevalidations[key] = total
	}
	ownershipTransitions := make(map[recordingOwnershipTransitionKey]int, len(r.ownershipTransitions))
	for key, total := range r.ownershipTransitions {
		ownershipTransitions[key] = total
	}

	errorStages := make([]string, len(r.errorStages))
	copy(errorStages, r.errorStages)

	return recordingMetricsSnapshot{
		opened:                      r.opened,
		closed:                      r.closed,
		frameReads:                  r.frameReads,
		frameWrites:                 frameWrites,
		handleCalls:                 r.handleCalls,
		persistCalls:                r.persistCalls,
		errorStages:                 errorStages,
		ownerLookupResults:          ownerLookupResults,
		routeDecisions:              routeDecisions,
		remoteOwnerConnectionsOpen:  remoteOwnerConnectionsOpen,
		remoteOwnerConnectionsClose: remoteOwnerConnectionsClose,
		remoteOwnerHandshakes:       remoteOwnerHandshakes,
		remoteOwnerMessages:         remoteOwnerMessages,
		remoteOwnerCloses:           remoteOwnerCloses,
		authorityRevalidations:      authorityRevalidations,
		ownershipTransitions:        ownershipTransitions,
	}
}

type recordingRemoteOwnerHandshakeKey struct {
	role   string
	result string
}

type recordingRemoteOwnerMessageKey struct {
	role      string
	direction string
	kind      string
}

type recordingRemoteOwnerCloseKey struct {
	role   string
	reason string
}

type recordingAuthorityRevalidationKey struct {
	role   string
	result string
}

type recordingOwnershipTransitionKey struct {
	from   string
	to     string
	result string
}

func recordingResultLabel(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func newHTTPTestServerWithHandler(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newLocalHTTPServerWithMetrics(t *testing.T, store storage.SnapshotStore, metrics Metrics) *Server {
	t.Helper()

	handler, err := NewServer(ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: resolveTestRequest,
		Metrics:        metrics,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	return handler
}

func testDocumentKey(documentID string) storage.DocumentKey {
	return storage.DocumentKey{
		Namespace:  "tests",
		DocumentID: documentID,
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("condicao nao satisfeita antes do timeout de %s", timeout)
}
