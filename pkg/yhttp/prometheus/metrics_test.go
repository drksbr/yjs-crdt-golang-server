package yhttpprometheus

import (
	"errors"
	"testing"
	"time"

	prometheuslib "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"yjs-go-bridge/pkg/yhttp"
)

func TestMetricsRecordsTransportLifecycle(t *testing.T) {
	t.Parallel()

	registry := prometheuslib.NewRegistry()
	metrics, err := New(Config{
		Namespace:                   "testbridge",
		Subsystem:                   "yhttp",
		Registerer:                  registry,
		HandleDurationBuckets:       []float64{0.01, 0.05, 0.1},
		PersistDurationBucket:       []float64{0.005, 0.02, 0.05},
		OwnerLookupDurationBuckets:  []float64{0.001, 0.01, 0.1},
		RemoteOwnerHandshakeBuckets: []float64{0.001, 0.01, 0.1},
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	req := yhttp.Request{}
	metrics.ConnectionOpened(req)
	metrics.FrameRead(req, 12)
	metrics.FrameWritten(req, "direct", 5)
	metrics.FrameWritten(req, "broadcast", 7)
	metrics.Handle(req, 25*time.Millisecond, nil)
	metrics.Handle(req, 75*time.Millisecond, errors.New("boom"))
	metrics.Persist(req, 10*time.Millisecond, nil)
	metrics.Persist(req, 30*time.Millisecond, errors.New("boom"))
	metrics.Error(req, "handle", errors.New("boom"))
	metrics.Error(req, "persist", errors.New("boom"))
	metrics.OwnerLookup(req, 8*time.Millisecond, "remote")
	metrics.RouteDecision(req, "remote_forward_ws")
	metrics.RemoteOwnerConnectionOpened(req, "edge")
	metrics.RemoteOwnerConnectionOpened(req, "owner")
	metrics.RemoteOwnerHandshake(req, "edge", 3*time.Millisecond, nil)
	metrics.RemoteOwnerHandshake(req, "owner", 6*time.Millisecond, errors.New("boom"))
	metrics.RemoteOwnerMessage(req, "edge", "out", "handshake")
	metrics.RemoteOwnerMessage(req, "edge", "out", "document_update")
	metrics.RemoteOwnerMessage(req, "owner", "in", "query_awareness_request")
	metrics.RemoteOwnerClose(req, "edge", "client_closed")
	metrics.RemoteOwnerConnectionClosed(req, "edge")
	metrics.RemoteOwnerConnectionClosed(req, "owner")
	metrics.ConnectionClosed(req)

	assertCounterValue(t, registry, "testbridge_yhttp_connections_opened_total", nil, 1)
	assertGaugeValue(t, registry, "testbridge_yhttp_connections_active", nil, 0)
	assertCounterValue(t, registry, "testbridge_yhttp_frames_read_total", nil, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_bytes_read_total", nil, 12)
	assertCounterValue(t, registry, "testbridge_yhttp_frames_written_total", map[string]string{"kind": "direct"}, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_frames_written_total", map[string]string{"kind": "broadcast"}, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_bytes_written_total", map[string]string{"kind": "direct"}, 5)
	assertCounterValue(t, registry, "testbridge_yhttp_bytes_written_total", map[string]string{"kind": "broadcast"}, 7)
	assertCounterValue(t, registry, "testbridge_yhttp_errors_total", map[string]string{"stage": "handle"}, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_errors_total", map[string]string{"stage": "persist"}, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_route_decisions_total", map[string]string{"decision": "remote_forward_ws"}, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_remote_owner_connections_opened_total", map[string]string{"role": "edge"}, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_remote_owner_connections_opened_total", map[string]string{"role": "owner"}, 1)
	assertGaugeValue(t, registry, "testbridge_yhttp_remote_owner_connections_active", map[string]string{"role": "edge"}, 0)
	assertGaugeValue(t, registry, "testbridge_yhttp_remote_owner_connections_active", map[string]string{"role": "owner"}, 0)
	assertCounterValue(t, registry, "testbridge_yhttp_remote_owner_messages_total", map[string]string{"role": "edge", "direction": "out", "kind": "handshake"}, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_remote_owner_messages_total", map[string]string{"role": "edge", "direction": "out", "kind": "document_update"}, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_remote_owner_messages_total", map[string]string{"role": "owner", "direction": "in", "kind": "query_awareness_request"}, 1)
	assertCounterValue(t, registry, "testbridge_yhttp_remote_owner_closes_total", map[string]string{"role": "edge", "reason": "client_closed"}, 1)

	assertHistogramCount(t, registry, "testbridge_yhttp_handle_duration_seconds", map[string]string{"result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_yhttp_handle_duration_seconds", map[string]string{"result": "error"}, 1)
	assertHistogramCount(t, registry, "testbridge_yhttp_persist_duration_seconds", map[string]string{"result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_yhttp_persist_duration_seconds", map[string]string{"result": "error"}, 1)
	assertHistogramCount(t, registry, "testbridge_yhttp_owner_lookup_duration_seconds", map[string]string{"result": "remote"}, 1)
	assertHistogramCount(t, registry, "testbridge_yhttp_remote_owner_handshake_duration_seconds", map[string]string{"role": "edge", "result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_yhttp_remote_owner_handshake_duration_seconds", map[string]string{"role": "owner", "result": "error"}, 1)
}

func assertCounterValue(t *testing.T, registry *prometheuslib.Registry, name string, labels map[string]string, want float64) {
	t.Helper()

	metric := findMetric(t, registry, name, labels)
	got := metric.GetCounter().GetValue()
	if got != want {
		t.Fatalf("%s counter = %v, want %v", name, got, want)
	}
}

func assertGaugeValue(t *testing.T, registry *prometheuslib.Registry, name string, labels map[string]string, want float64) {
	t.Helper()

	metric := findMetric(t, registry, name, labels)
	got := metric.GetGauge().GetValue()
	if got != want {
		t.Fatalf("%s gauge = %v, want %v", name, got, want)
	}
}

func assertHistogramCount(t *testing.T, registry *prometheuslib.Registry, name string, labels map[string]string, want uint64) {
	t.Helper()

	metric := findMetric(t, registry, name, labels)
	got := metric.GetHistogram().GetSampleCount()
	if got != want {
		t.Fatalf("%s histogram count = %d, want %d", name, got, want)
	}
}

func findMetric(t *testing.T, registry *prometheuslib.Registry, name string, labels map[string]string) *dto.Metric {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("registry.Gather() unexpected error: %v", err)
	}

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if labelsMatch(metric.GetLabel(), labels) {
				return metric
			}
		}
	}

	t.Fatalf("metric %s with labels %v not found", name, labels)
	return nil
}

func labelsMatch(pairs []*dto.LabelPair, expected map[string]string) bool {
	if len(expected) == 0 {
		return len(pairs) == 0
	}
	if len(pairs) != len(expected) {
		return false
	}
	for _, pair := range pairs {
		value, ok := expected[pair.GetName()]
		if !ok || value != pair.GetValue() {
			return false
		}
	}
	return true
}
