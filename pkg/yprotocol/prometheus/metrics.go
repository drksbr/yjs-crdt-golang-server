package yprotocolprometheus

import (
	"fmt"
	"strings"
	"time"

	prometheuslib "github.com/prometheus/client_golang/prometheus"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

const (
	defaultNamespace = "yjsbridge"
	defaultSubsystem = "yprotocol"
)

// Config define o namespace e o registro Prometheus usados pelo adapter.
type Config struct {
	Namespace                    string
	Subsystem                    string
	Registerer                   prometheuslib.Registerer
	ConstLabels                  prometheuslib.Labels
	PersistDurationBuckets       []float64
	AuthorityRevalidationBuckets []float64
}

// Metrics implementa `yprotocol.Metrics` com coletores Prometheus.
type Metrics struct {
	roomsOpened           prometheuslib.Counter
	roomsClosed           prometheuslib.Counter
	roomsActive           prometheuslib.Gauge
	persistDuration       *prometheuslib.HistogramVec
	persistCompacted      prometheuslib.Counter
	persistThrough        prometheuslib.Gauge
	persistLastCompacted  prometheuslib.Gauge
	authorityRevalidation *prometheuslib.HistogramVec
	authorityLosses       *prometheuslib.CounterVec
}

var _ yprotocol.Metrics = (*Metrics)(nil)

// New constrói e registra um conjunto de métricas para `pkg/yprotocol`.
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

	persistBuckets := cfg.PersistDurationBuckets
	if len(persistBuckets) == 0 {
		persistBuckets = prometheuslib.DefBuckets
	}
	revalidationBuckets := cfg.AuthorityRevalidationBuckets
	if len(revalidationBuckets) == 0 {
		revalidationBuckets = prometheuslib.DefBuckets
	}

	metrics := &Metrics{
		roomsOpened: prometheuslib.NewCounter(prometheuslib.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "rooms_opened_total",
			Help:        "Total de rooms materializados pelo provider local.",
			ConstLabels: constLabels,
		}),
		roomsClosed: prometheuslib.NewCounter(prometheuslib.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "rooms_closed_total",
			Help:        "Total de rooms desmontados pelo provider local.",
			ConstLabels: constLabels,
		}),
		roomsActive: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "rooms_active",
			Help:        "Numero atual de rooms ativos no provider local.",
			ConstLabels: constLabels,
		}),
		persistDuration: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "persist_duration_seconds",
			Help:        "Duracao da persistencia/compaction do snapshot autoritativo local.",
			ConstLabels: constLabels,
			Buckets:     persistBuckets,
		}, []string{"result"}),
		persistCompacted: prometheuslib.NewCounter(prometheuslib.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "persist_updates_compacted_total",
			Help:        "Total de updates incrementais absorvidos por persist/compaction local.",
			ConstLabels: constLabels,
		}),
		persistThrough: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "persist_through_offset",
			Help:        "Ultimo offset checkpointado por persistencia/compaction local.",
			ConstLabels: constLabels,
		}),
		persistLastCompacted: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "persist_last_compacted_updates",
			Help:        "Quantidade de updates compactados na ultima persistencia local observada.",
			ConstLabels: constLabels,
		}),
		authorityRevalidation: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "authority_revalidation_duration_seconds",
			Help:        "Duracao das revalidacoes de autoridade feitas pelo provider local.",
			ConstLabels: constLabels,
			Buckets:     revalidationBuckets,
		}, []string{"result"}),
		authorityLosses: prometheuslib.NewCounterVec(prometheuslib.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "authority_losses_total",
			Help:        "Total de perdas de autoridade observadas pelo provider local.",
			ConstLabels: constLabels,
		}, []string{"stage"}),
	}

	registerer := cfg.Registerer
	if registerer == nil {
		registerer = prometheuslib.DefaultRegisterer
	}

	collectors := []prometheuslib.Collector{
		metrics.roomsOpened,
		metrics.roomsClosed,
		metrics.roomsActive,
		metrics.persistDuration,
		metrics.persistCompacted,
		metrics.persistThrough,
		metrics.persistLastCompacted,
		metrics.authorityRevalidation,
		metrics.authorityLosses,
	}
	for _, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("registrar coletor prometheus: %w", err)
		}
	}

	return metrics, nil
}

// RoomOpened contabiliza a materializacao de um room.
func (m *Metrics) RoomOpened(storage.DocumentKey) {
	m.roomsOpened.Inc()
	m.roomsActive.Inc()
}

// RoomClosed contabiliza o teardown de um room.
func (m *Metrics) RoomClosed(storage.DocumentKey) {
	m.roomsClosed.Inc()
	m.roomsActive.Dec()
}

// Persist observa a duracao da persistencia/compaction local.
func (m *Metrics) Persist(_ storage.DocumentKey, duration time.Duration, through storage.UpdateOffset, compacted storage.UpdateOffset, err error) {
	m.persistDuration.WithLabelValues(resultLabel(err)).Observe(duration.Seconds())
	m.persistThrough.Set(float64(through))
	m.persistLastCompacted.Set(float64(compacted))
	if err == nil && compacted > 0 {
		m.persistCompacted.Add(float64(compacted))
	}
}

// AuthorityRevalidation observa a duracao da revalidacao de autoridade.
func (m *Metrics) AuthorityRevalidation(_ storage.DocumentKey, duration time.Duration, err error) {
	m.authorityRevalidation.WithLabelValues(resultLabel(err)).Observe(duration.Seconds())
}

// AuthorityLost incrementa o contador da fase em que a autoridade se perdeu.
func (m *Metrics) AuthorityLost(_ storage.DocumentKey, stage string) {
	m.authorityLosses.WithLabelValues(normalizeLabel(stage, "unknown")).Inc()
}

func resultLabel(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
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
