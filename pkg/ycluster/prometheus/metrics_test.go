package yclusterprometheus

import (
	"testing"
	"time"

	prometheuslib "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMetricsRecordsControlPlaneLifecycle(t *testing.T) {
	t.Parallel()

	registry := prometheuslib.NewRegistry()
	metrics, err := New(Config{
		Namespace:                 "testbridge",
		Subsystem:                 "ycluster",
		Registerer:                registry,
		OwnerLookupBuckets:        []float64{0.001, 0.01, 0.1},
		LeaseOperationBuckets:     []float64{0.001, 0.01, 0.1},
		LeaseManagerActionBuckets: []float64{0.001, 0.01, 0.1},
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	metrics.OwnerLookup(3*time.Millisecond, "local")
	metrics.OwnerLookup(8*time.Millisecond, "lease_expired")
	metrics.LeaseOperation(7, "acquire", 5*time.Millisecond, "ok")
	metrics.LeaseOperation(7, "renew", 7*time.Millisecond, "held")
	metrics.LeaseManagerAction(7, "acquire", 6*time.Millisecond, "ok")
	metrics.LeaseManagerAction(7, "reacquire", 9*time.Millisecond, "error")

	assertCounterValue(t, registry, "testbridge_ycluster_lease_operations_total", map[string]string{"operation": "acquire", "result": "ok"}, 1)
	assertCounterValue(t, registry, "testbridge_ycluster_lease_operations_total", map[string]string{"operation": "renew", "result": "held"}, 1)
	assertCounterValue(t, registry, "testbridge_ycluster_lease_manager_actions_total", map[string]string{"action": "acquire", "result": "ok"}, 1)
	assertCounterValue(t, registry, "testbridge_ycluster_lease_manager_actions_total", map[string]string{"action": "reacquire", "result": "error"}, 1)

	assertHistogramCount(t, registry, "testbridge_ycluster_owner_lookup_duration_seconds", map[string]string{"result": "local"}, 1)
	assertHistogramCount(t, registry, "testbridge_ycluster_owner_lookup_duration_seconds", map[string]string{"result": "lease_expired"}, 1)
	assertHistogramCount(t, registry, "testbridge_ycluster_lease_operation_duration_seconds", map[string]string{"operation": "acquire", "result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_ycluster_lease_operation_duration_seconds", map[string]string{"operation": "renew", "result": "held"}, 1)
	assertHistogramCount(t, registry, "testbridge_ycluster_lease_manager_action_duration_seconds", map[string]string{"action": "acquire", "result": "ok"}, 1)
	assertHistogramCount(t, registry, "testbridge_ycluster_lease_manager_action_duration_seconds", map[string]string{"action": "reacquire", "result": "error"}, 1)
}

func assertCounterValue(t *testing.T, registry *prometheuslib.Registry, name string, labels map[string]string, want float64) {
	t.Helper()

	metric := findMetric(t, registry, name, labels)
	got := metric.GetCounter().GetValue()
	if got != want {
		t.Fatalf("%s counter = %v, want %v", name, got, want)
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
