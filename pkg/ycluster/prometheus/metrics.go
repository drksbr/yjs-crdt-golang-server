package yclusterprometheus

import (
	"fmt"
	"strings"
	"time"

	prometheuslib "github.com/prometheus/client_golang/prometheus"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
)

const (
	defaultNamespace = "yjsbridge"
	defaultSubsystem = "ycluster"
)

// Config define o namespace e o registro Prometheus usados pelo adapter.
type Config struct {
	Namespace                 string
	Subsystem                 string
	Registerer                prometheuslib.Registerer
	ConstLabels               prometheuslib.Labels
	OwnerLookupBuckets        []float64
	LeaseOperationBuckets     []float64
	LeaseManagerActionBuckets []float64
}

// Metrics implementa `ycluster.Metrics` com coletores Prometheus.
type Metrics struct {
	ownerLookupDuration *prometheuslib.HistogramVec
	leaseOperations     *prometheuslib.CounterVec
	leaseOperationDur   *prometheuslib.HistogramVec
	leaseManagerActions *prometheuslib.CounterVec
	leaseManagerDur     *prometheuslib.HistogramVec
}

var _ ycluster.Metrics = (*Metrics)(nil)

// New constrói e registra um conjunto de métricas para `pkg/ycluster`.
func New(cfg Config) (*Metrics, error) {
	namespace := strings.TrimSpace(cfg.Namespace)
	if namespace == "" {
		namespace = defaultNamespace
	}

	subsystem := strings.TrimSpace(cfg.Subsystem)
	if subsystem == "" {
		subsystem = defaultSubsystem
	}
	constLabels := cloneConstLabels(cfg.ConstLabels)

	lookupBuckets := cfg.OwnerLookupBuckets
	if len(lookupBuckets) == 0 {
		lookupBuckets = prometheuslib.DefBuckets
	}
	leaseOpBuckets := cfg.LeaseOperationBuckets
	if len(leaseOpBuckets) == 0 {
		leaseOpBuckets = prometheuslib.DefBuckets
	}
	managerBuckets := cfg.LeaseManagerActionBuckets
	if len(managerBuckets) == 0 {
		managerBuckets = prometheuslib.DefBuckets
	}

	metrics := &Metrics{
		ownerLookupDuration: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "owner_lookup_duration_seconds",
			Help:        "Duracao das resolucoes de owner do control plane.",
			ConstLabels: constLabels,
			Buckets:     lookupBuckets,
		}, []string{"result"}),
		leaseOperations: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "lease_operations_total",
			Help:        "Total de operacoes de lease no control plane storage-backed.",
			ConstLabels: constLabels,
		}, []string{"operation", "result"}),
		leaseOperationDur: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "lease_operation_duration_seconds",
			Help:        "Duracao das operacoes de lease no control plane storage-backed.",
			ConstLabels: constLabels,
			Buckets:     leaseOpBuckets,
		}, []string{"operation", "result"}),
		leaseManagerActions: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "lease_manager_actions_total",
			Help:        "Total de acoes do LeaseManager local.",
			ConstLabels: constLabels,
		}, []string{"action", "result"}),
		leaseManagerDur: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "lease_manager_action_duration_seconds",
			Help:        "Duracao das acoes do LeaseManager local.",
			ConstLabels: constLabels,
			Buckets:     managerBuckets,
		}, []string{"action", "result"}),
	}

	registerer := cfg.Registerer
	if registerer == nil {
		registerer = prometheuslib.DefaultRegisterer
	}

	collectors := []prometheuslib.Collector{
		metrics.ownerLookupDuration,
		metrics.leaseOperations,
		metrics.leaseOperationDur,
		metrics.leaseManagerActions,
		metrics.leaseManagerDur,
	}
	for _, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("registrar coletor prometheus: %w", err)
		}
	}

	return metrics, nil
}

// OwnerLookup observa a duracao e o resultado de uma resolucao de owner.
func (m *Metrics) OwnerLookup(duration time.Duration, result string) {
	m.ownerLookupDuration.WithLabelValues(normalizeLabel(result, "unknown")).Observe(duration.Seconds())
}

// LeaseOperation observa a operacao storage-backed de lease.
func (m *Metrics) LeaseOperation(_ ycluster.ShardID, operation string, duration time.Duration, result string) {
	op := normalizeLabel(operation, "unknown")
	res := normalizeLabel(result, "unknown")
	m.leaseOperations.WithLabelValues(op, res).Inc()
	m.leaseOperationDur.WithLabelValues(op, res).Observe(duration.Seconds())
}

// LeaseManagerAction observa a ação de autonomia local do LeaseManager.
func (m *Metrics) LeaseManagerAction(_ ycluster.ShardID, action string, duration time.Duration, result string) {
	act := normalizeLabel(action, "unknown")
	res := normalizeLabel(result, "unknown")
	m.leaseManagerActions.WithLabelValues(act, res).Inc()
	m.leaseManagerDur.WithLabelValues(act, res).Observe(duration.Seconds())
}

func normalizeLabel(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func cloneConstLabels(labels prometheuslib.Labels) prometheuslib.Labels {
	if len(labels) == 0 {
		return nil
	}
	cloned := make(prometheuslib.Labels, len(labels))
	for key, value := range labels {
		cloned[key] = value
	}
	return cloned
}
