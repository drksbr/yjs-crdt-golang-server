package integration

import (
	"context"
	"math"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

const (
	runPerfMatrixEnv     = "YJSBRIDGE_RUN_PERF_MATRIX"
	perfIterations       = 200
	perfWarmupIterations = 20
	perfIOTimeout        = 15 * time.Second
)

type perfResult struct {
	framework  string
	backend    string
	iterations int
	throughput float64
	avg        time.Duration
	p50        time.Duration
	p95        time.Duration
	p99        time.Duration
	restore    time.Duration
}

func TestWebSocketPerformanceMatrix(t *testing.T) {
	requirePerfMatrix(t)

	frameworks := []string{"net/http", "gin", "echo", "chi"}
	backends := newPerfBackends(t)
	results := make([]perfResult, 0, len(frameworks)*len(backends))

	for _, backend := range backends {
		for _, framework := range frameworks {
			framework := framework
			backend := backend

			t.Run(backend.name+"/"+framework, func(t *testing.T) {
				result := runPerfCase(t, framework, backend)
				results = append(results, result)
				t.Logf(
					"perf matrix: framework=%s backend=%s throughput=%.1f updates/s avg=%s p50=%s p95=%s p99=%s restore=%s",
					result.framework,
					result.backend,
					result.throughput,
					result.avg,
					result.p50,
					result.p95,
					result.p99,
					result.restore,
				)
			})
		}
	}

	t.Log("perf matrix resumo:")
	for _, result := range results {
		t.Logf(
			"%s/%s throughput=%.1f avg=%s p50=%s p95=%s p99=%s restore=%s",
			result.backend,
			result.framework,
			result.throughput,
			result.avg,
			result.p50,
			result.p95,
			result.p99,
			result.restore,
		)
	}
}

func runPerfCase(t *testing.T, framework string, backend perfBackend) perfResult {
	t.Helper()

	caseName := backend.name + "_" + framework
	store, cleanupStore := backend.newStore(t, caseName)
	defer cleanupStore()

	key := perfDocumentKey(framework, backend.name)
	server1, closeServer1 := newPerfServer(t, framework, store)

	left := dialSmokeWS(t, server1.URL+"/ws?doc="+key.DocumentID+"&client=701&conn=left&persist=1")
	right := dialSmokeWS(t, server1.URL+"/ws?doc="+key.DocumentID+"&client=702&conn=right&persist=1")

	runPerfWarmup(t, left, right)
	elapsed, latencies := runPerfLoop(t, left, right)

	closeSmokeWS(t, left)
	closeSmokeWS(t, right)
	waitSnapshotStored(t, store, key)
	closeServer1()

	server2, closeServer2 := newPerfServer(t, framework, store)
	defer closeServer2()

	restore := measureRestoreLatency(t, server2.URL, key.DocumentID)
	avg, p50, p95, p99 := summarizeLatencies(latencies)

	return perfResult{
		framework:  framework,
		backend:    backend.name,
		iterations: len(latencies),
		throughput: float64(len(latencies)) / elapsed.Seconds(),
		avg:        avg,
		p50:        p50,
		p95:        p95,
		p99:        p99,
		restore:    restore,
	}
}

func runPerfWarmup(t *testing.T, left, right *websocket.Conn) {
	t.Helper()

	for i := 0; i < perfWarmupIterations; i++ {
		payload := yprotocol.EncodeProtocolSyncUpdate(buildGCOnlyUpdate(uint32(5000+i), 1))
		sender, receiver := perfPeers(left, right, i)
		writePerfBinary(t, sender, payload)
		_ = readPerfBinary(t, receiver)
	}
}

func runPerfLoop(t *testing.T, left, right *websocket.Conn) (time.Duration, []time.Duration) {
	t.Helper()

	latencies := make([]time.Duration, 0, perfIterations)
	start := time.Now()
	for i := 0; i < perfIterations; i++ {
		payload := yprotocol.EncodeProtocolSyncUpdate(buildGCOnlyUpdate(uint32(9000+i), 1))
		sender, receiver := perfPeers(left, right, i)

		iterationStart := time.Now()
		writePerfBinary(t, sender, payload)
		_ = readPerfBinary(t, receiver)
		latencies = append(latencies, time.Since(iterationStart))
	}

	return time.Since(start), latencies
}

func measureRestoreLatency(t *testing.T, serverURL, documentID string) time.Duration {
	t.Helper()

	probe := dialSmokeWS(t, serverURL+"/ws?doc="+documentID+"&client=703&conn=probe")
	start := time.Now()
	writePerfBinary(t, probe, yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	reply := readPerfBinary(t, probe)
	restore := time.Since(start)

	messages, err := yprotocol.DecodeProtocolMessages(reply)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil || messages[0].Sync.Type != yprotocol.SyncMessageTypeStep2 {
		t.Fatalf("restore reply = %#v, want single sync step2", messages)
	}

	return restore
}

func summarizeLatencies(samples []time.Duration) (time.Duration, time.Duration, time.Duration, time.Duration) {
	if len(samples) == 0 {
		return 0, 0, 0, 0
	}

	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total int64
	for _, sample := range samples {
		total += int64(sample)
	}

	return time.Duration(total / int64(len(samples))),
		percentileDuration(sorted, 0.50),
		percentileDuration(sorted, 0.95),
		percentileDuration(sorted, 0.99)
}

func percentileDuration(sorted []time.Duration, percentile float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}

	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func waitSnapshotStored(t *testing.T, store storage.SnapshotStore, key storage.DocumentKey) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		record, err := store.LoadSnapshot(context.Background(), key)
		if err == nil && record != nil && record.Snapshot != nil && !record.Snapshot.IsEmpty() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("snapshot persistido nao apareceu para %s/%s", key.Namespace, key.DocumentID)
}

func requirePerfMatrix(t *testing.T) {
	t.Helper()

	if os.Getenv(runPerfMatrixEnv) == "" {
		t.Skipf("matriz de performance ignorada: defina %s=1", runPerfMatrixEnv)
	}
}

func perfPeers(left, right *websocket.Conn, iteration int) (*websocket.Conn, *websocket.Conn) {
	if iteration%2 == 0 {
		return left, right
	}
	return right, left
}

func newTransportHandler(store storage.SnapshotStore) (*yhttp.Server, error) {
	handler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: resolveSmokeRequest,
	})
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func writePerfBinary(t *testing.T, conn *websocket.Conn, payload []byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), perfIOTimeout)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, payload); err != nil {
		t.Fatalf("conn.Write() unexpected error: %v", err)
	}
}

func readPerfBinary(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), perfIOTimeout)
	defer cancel()

	msgType, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() unexpected error: %v", err)
	}
	if msgType != websocket.MessageBinary {
		t.Fatalf("msgType = %v, want %v", msgType, websocket.MessageBinary)
	}
	return payload
}
