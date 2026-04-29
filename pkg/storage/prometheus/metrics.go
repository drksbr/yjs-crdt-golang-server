package storageprometheus

import (
	"fmt"
	"strings"
	"time"

	prometheuslib "github.com/prometheus/client_golang/prometheus"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

const (
	defaultNamespace = "yjsbridge"
	defaultSubsystem = "storage"
)

// Config define o namespace e o registro Prometheus usados pelo adapter.
type Config struct {
	Namespace                string
	Subsystem                string
	Registerer               prometheuslib.Registerer
	ConstLabels              prometheuslib.Labels
	ReplayDurationBuckets    []float64
	RecoveryDurationBuckets  []float64
	CompactionDurationBucket []float64
}

// Metrics implementa `storage.Metrics` com coletores Prometheus.
type Metrics struct {
	replayDuration         *prometheuslib.HistogramVec
	recoveryDuration       *prometheuslib.HistogramVec
	compactionDuration     *prometheuslib.HistogramVec
	replayUpdatesApplied   prometheuslib.Counter
	recoveryUpdatesApplied prometheuslib.Counter
	compactionUpdates      prometheuslib.Counter
	replayThrough          prometheuslib.Gauge
	replayLastEpoch        prometheuslib.Gauge
	recoveryCheckpoint     prometheuslib.Gauge
	recoveryLastOffset     prometheuslib.Gauge
	recoveryTailLag        prometheuslib.Gauge
	recoveryLastEpoch      prometheuslib.Gauge
	compactionThrough      prometheuslib.Gauge
	compactionLastEpoch    prometheuslib.Gauge
}

var _ storage.Metrics = (*Metrics)(nil)

// New constrói e registra um conjunto de métricas para `pkg/storage`.
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

	replayBuckets := cfg.ReplayDurationBuckets
	if len(replayBuckets) == 0 {
		replayBuckets = prometheuslib.DefBuckets
	}
	recoveryBuckets := cfg.RecoveryDurationBuckets
	if len(recoveryBuckets) == 0 {
		recoveryBuckets = prometheuslib.DefBuckets
	}
	compactionBuckets := cfg.CompactionDurationBucket
	if len(compactionBuckets) == 0 {
		compactionBuckets = prometheuslib.DefBuckets
	}

	metrics := &Metrics{
		replayDuration: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "replay_update_log_duration_seconds",
			Help:        "Duracao do replay incremental do update log sobre um snapshot base.",
			ConstLabels: constLabels,
			Buckets:     replayBuckets,
		}, []string{"result"}),
		recoveryDuration: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "recovery_duration_seconds",
			Help:        "Duracao do recovery por snapshot base + replay do tail incremental.",
			ConstLabels: constLabels,
			Buckets:     recoveryBuckets,
		}, []string{"result"}),
		compactionDuration: prometheuslib.NewHistogramVec(prometheuslib.HistogramOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "compaction_duration_seconds",
			Help:        "Duracao da compaction do update log em um snapshot checkpointado.",
			ConstLabels: constLabels,
			Buckets:     compactionBuckets,
		}, []string{"result"}),
		replayUpdatesApplied: prometheuslib.NewCounter(prometheuslib.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "replay_updates_applied_total",
			Help:        "Total de updates incrementais aplicados por replay do update log.",
			ConstLabels: constLabels,
		}),
		recoveryUpdatesApplied: prometheuslib.NewCounter(prometheuslib.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "recovery_updates_applied_total",
			Help:        "Total de updates incrementais aplicados durante recovery.",
			ConstLabels: constLabels,
		}),
		compactionUpdates: prometheuslib.NewCounter(prometheuslib.CounterOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "compaction_updates_applied_total",
			Help:        "Total de updates incrementais absorvidos por compaction/checkpoint.",
			ConstLabels: constLabels,
		}),
		replayThrough: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "replay_through_offset",
			Help:        "Ultimo offset observado apos replay incremental do update log.",
			ConstLabels: constLabels,
		}),
		replayLastEpoch: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "replay_last_epoch",
			Help:        "Ultimo epoch observado apos replay incremental do update log.",
			ConstLabels: constLabels,
		}),
		recoveryCheckpoint: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "recovery_checkpoint_through_offset",
			Help:        "Offset do checkpoint base usado no ultimo recovery observado.",
			ConstLabels: constLabels,
		}),
		recoveryLastOffset: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "recovery_last_offset",
			Help:        "Ultimo offset observado apos recovery por snapshot + tail.",
			ConstLabels: constLabels,
		}),
		recoveryTailLag: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "recovery_tail_lag_updates",
			Help:        "Diferenca entre o ultimo offset e o checkpoint base no ultimo recovery observado.",
			ConstLabels: constLabels,
		}),
		recoveryLastEpoch: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "recovery_last_epoch",
			Help:        "Ultimo epoch observado apos recovery por snapshot + tail.",
			ConstLabels: constLabels,
		}),
		compactionThrough: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "compaction_through_offset",
			Help:        "Ultimo offset compactado em snapshot checkpointado.",
			ConstLabels: constLabels,
		}),
		compactionLastEpoch: prometheuslib.NewGauge(prometheuslib.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   subsystem,
			Name:        "compaction_last_epoch",
			Help:        "Ultimo epoch absorvido pela compaction/checkpoint observada.",
			ConstLabels: constLabels,
		}),
	}

	registerer := cfg.Registerer
	if registerer == nil {
		registerer = prometheuslib.DefaultRegisterer
	}

	collectors := []prometheuslib.Collector{
		metrics.replayDuration,
		metrics.recoveryDuration,
		metrics.compactionDuration,
		metrics.replayUpdatesApplied,
		metrics.recoveryUpdatesApplied,
		metrics.compactionUpdates,
		metrics.replayThrough,
		metrics.replayLastEpoch,
		metrics.recoveryCheckpoint,
		metrics.recoveryLastOffset,
		metrics.recoveryTailLag,
		metrics.recoveryLastEpoch,
		metrics.compactionThrough,
		metrics.compactionLastEpoch,
	}
	for _, collector := range collectors {
		if err := registerer.Register(collector); err != nil {
			return nil, fmt.Errorf("registrar coletor prometheus: %w", err)
		}
	}

	return metrics, nil
}

// ReplayUpdateLog observa duracao e volume aplicado no replay incremental.
func (m *Metrics) ReplayUpdateLog(_ storage.DocumentKey, duration time.Duration, applied int, through storage.UpdateOffset, lastEpoch uint64, err error) {
	m.replayDuration.WithLabelValues(resultLabel(err)).Observe(duration.Seconds())
	if applied > 0 {
		m.replayUpdatesApplied.Add(float64(applied))
	}
	m.replayThrough.Set(float64(through))
	m.replayLastEpoch.Set(float64(lastEpoch))
}

// RecoverSnapshot observa duracao e volume aplicado no recovery.
func (m *Metrics) RecoverSnapshot(_ storage.DocumentKey, duration time.Duration, updates int, checkpointThrough storage.UpdateOffset, lastOffset storage.UpdateOffset, lastEpoch uint64, err error) {
	m.recoveryDuration.WithLabelValues(resultLabel(err)).Observe(duration.Seconds())
	if updates > 0 {
		m.recoveryUpdatesApplied.Add(float64(updates))
	}
	m.recoveryCheckpoint.Set(float64(checkpointThrough))
	m.recoveryLastOffset.Set(float64(lastOffset))
	m.recoveryTailLag.Set(updateOffsetDelta(lastOffset, checkpointThrough))
	m.recoveryLastEpoch.Set(float64(lastEpoch))
}

// CompactUpdateLog observa duracao e volume absorvido na compaction.
func (m *Metrics) CompactUpdateLog(_ storage.DocumentKey, duration time.Duration, applied int, through storage.UpdateOffset, lastEpoch uint64, err error) {
	m.compactionDuration.WithLabelValues(resultLabel(err)).Observe(duration.Seconds())
	if applied > 0 {
		m.compactionUpdates.Add(float64(applied))
	}
	m.compactionThrough.Set(float64(through))
	m.compactionLastEpoch.Set(float64(lastEpoch))
}

func resultLabel(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}

func updateOffsetDelta(last storage.UpdateOffset, base storage.UpdateOffset) float64 {
	if last <= base {
		return 0
	}
	return float64(last - base)
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
