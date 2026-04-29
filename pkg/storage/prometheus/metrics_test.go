package storageprometheus

import (
	"errors"
	"testing"
	"time"

	prometheuslib "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

func TestMetricsRecordsReplayRecoveryAndCompaction(t *testing.T) {
	t.Parallel()

	registry := prometheuslib.NewRegistry()
	metrics, err := New(Config{
		Namespace:               "testbridge",
		Subsystem:               "storage",
		Registerer:              registry,
		ReplayDurationBuckets:   []float64{0.001, 0.01, 0.1},
		RecoveryDurationBuckets: []float64{0.001, 0.01, 0.1},
		CompactionDurationBucket: []float64{
			0.001, 0.01, 0.1,
		},
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	key := storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-prometheus"}
	metrics.ReplayUpdateLog(key, 6*time.Millisecond, 0, 9, 7, errors.New("boom"))
	metrics.ReplayUpdateLog(key, 3*time.Millisecond, 4, 9, 7, nil)
	metrics.RecoverSnapshot(key, 9*time.Millisecond, 0, 7, 7, 7, errors.New("boom"))
	metrics.RecoverSnapshot(key, 5*time.Millisecond, 2, 7, 9, 8, nil)
	metrics.CompactUpdateLog(key, 11*time.Millisecond, 1, 9, 8, errors.New("boom"))
	metrics.CompactUpdateLog(key, 7*time.Millisecond, 3, 9, 8, nil)

	assertCounterValue(t, registry, "testbridge_storage_replay_updates_applied_total", nil, 4)
	assertCounterValue(t, registry, "testbridge_storage_recovery_updates_applied_total", nil, 2)
	assertCounterValue(t, registry, "testbridge_storage_compaction_updates_applied_total", nil, 4)

	assertGaugeValue(t, registry, "testbridge_storage_replay_through_offset", nil, 9)
	assertGaugeValue(t, registry, "testbridge_storage_replay_last_epoch", nil, 7)
	assertGaugeValue(t, registry, "testbridge_storage_recovery_checkpoint_through_offset", nil, 7)
	assertGaugeValue(t, registry, "testbridge_storage_recovery_last_offset", nil, 9)
	assertGaugeValue(t, registry, "testbridge_storage_recovery_tail_lag_updates", nil, 2)
	assertGaugeValue(t, registry, "testbridge_storage_recovery_last_epoch", nil, 8)
	assertGaugeValue(t, registry, "testbridge_storage_compaction_through_offset", nil, 9)
	assertGaugeValue(t, registry, "testbridge_storage_compaction_last_epoch", nil, 8)

	assertHistogramCount(t, registry, "testbridge_storage_replay_update_log_duration_seconds", map[string]string{"result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_storage_replay_update_log_duration_seconds", map[string]string{"result": "error"}, 1)
	assertHistogramCount(t, registry, "testbridge_storage_recovery_duration_seconds", map[string]string{"result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_storage_recovery_duration_seconds", map[string]string{"result": "error"}, 1)
	assertHistogramCount(t, registry, "testbridge_storage_compaction_duration_seconds", map[string]string{"result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_storage_compaction_duration_seconds", map[string]string{"result": "error"}, 1)
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
