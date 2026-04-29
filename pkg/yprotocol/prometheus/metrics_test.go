package yprotocolprometheus

import (
	"errors"
	"testing"
	"time"

	prometheuslib "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

func TestMetricsRecordsProviderLifecycle(t *testing.T) {
	t.Parallel()

	registry := prometheuslib.NewRegistry()
	metrics, err := New(Config{
		Namespace:                    "testbridge",
		Subsystem:                    "yprotocol",
		Registerer:                   registry,
		PersistDurationBuckets:       []float64{0.001, 0.01, 0.1},
		AuthorityRevalidationBuckets: []float64{0.001, 0.01, 0.1},
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	key := storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-prometheus"}
	metrics.RoomOpened(key)
	metrics.Persist(key, 8*time.Millisecond, 9, 2, errors.New("boom"))
	metrics.Persist(key, 4*time.Millisecond, 9, 3, nil)
	metrics.AuthorityRevalidation(key, 5*time.Millisecond, nil)
	metrics.AuthorityRevalidation(key, 7*time.Millisecond, errors.New("boom"))
	metrics.AuthorityLost(key, "append")
	metrics.AuthorityLost(key, "revalidate")
	metrics.RoomClosed(key)

	assertCounterValue(t, registry, "testbridge_yprotocol_rooms_opened_total", nil, 1)
	assertCounterValue(t, registry, "testbridge_yprotocol_rooms_closed_total", nil, 1)
	assertGaugeValue(t, registry, "testbridge_yprotocol_rooms_active", nil, 0)
	assertCounterValue(t, registry, "testbridge_yprotocol_persist_updates_compacted_total", nil, 3)
	assertGaugeValue(t, registry, "testbridge_yprotocol_persist_through_offset", nil, 9)
	assertGaugeValue(t, registry, "testbridge_yprotocol_persist_last_compacted_updates", nil, 3)
	assertCounterValue(t, registry, "testbridge_yprotocol_authority_losses_total", map[string]string{"stage": "append"}, 1)
	assertCounterValue(t, registry, "testbridge_yprotocol_authority_losses_total", map[string]string{"stage": "revalidate"}, 1)
	assertHistogramCount(t, registry, "testbridge_yprotocol_persist_duration_seconds", map[string]string{"result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_yprotocol_persist_duration_seconds", map[string]string{"result": "error"}, 1)
	assertHistogramCount(t, registry, "testbridge_yprotocol_authority_revalidation_duration_seconds", map[string]string{"result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_yprotocol_authority_revalidation_duration_seconds", map[string]string{"result": "error"}, 1)
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
