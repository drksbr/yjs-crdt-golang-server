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
	if snapshot.frameReads != 2 {
		t.Fatalf("frameReads = %d, want 2", snapshot.frameReads)
	}
	if snapshot.frameWrites["direct"] != 1 {
		t.Fatalf("frameWrites[direct] = %d, want 1", snapshot.frameWrites["direct"])
	}
	if snapshot.frameWrites["broadcast"] != 1 {
		t.Fatalf("frameWrites[broadcast] = %d, want 1", snapshot.frameWrites["broadcast"])
	}
	if snapshot.handleCalls != 2 {
		t.Fatalf("handleCalls = %d, want 2", snapshot.handleCalls)
	}
	if len(snapshot.errorStages) != 0 {
		t.Fatalf("errorStages = %v, want none", snapshot.errorStages)
	}
}

type recordingMetrics struct {
	mu           sync.Mutex
	opened       int
	closed       int
	frameReads   int
	frameWrites  map[string]int
	handleCalls  int
	persistCalls int
	errorStages  []string
}

type recordingMetricsSnapshot struct {
	opened       int
	closed       int
	frameReads   int
	frameWrites  map[string]int
	handleCalls  int
	persistCalls int
	errorStages  []string
}

func newRecordingMetrics() *recordingMetrics {
	return &recordingMetrics{
		frameWrites: make(map[string]int),
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

func (r *recordingMetrics) snapshot() recordingMetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	frameWrites := make(map[string]int, len(r.frameWrites))
	for kind, total := range r.frameWrites {
		frameWrites[kind] = total
	}

	errorStages := make([]string, len(r.errorStages))
	copy(errorStages, r.errorStages)

	return recordingMetricsSnapshot{
		opened:       r.opened,
		closed:       r.closed,
		frameReads:   r.frameReads,
		frameWrites:  frameWrites,
		handleCalls:  r.handleCalls,
		persistCalls: r.persistCalls,
		errorStages:  errorStages,
	}
}

func newHTTPTestServerWithHandler(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
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
