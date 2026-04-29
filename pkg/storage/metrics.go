package storage

import (
	"context"
	"time"
)

// ReplayMetrics adiciona observabilidade opcional ao helper de replay do tail
// incremental do update log.
type ReplayMetrics interface {
	ReplayUpdateLog(key DocumentKey, duration time.Duration, applied int, through UpdateOffset, lastEpoch uint64, err error)
}

// RecoveryMetrics adiciona observabilidade opcional ao helper de recovery por
// `snapshot + update log`.
type RecoveryMetrics interface {
	RecoverSnapshot(key DocumentKey, duration time.Duration, updates int, checkpointThrough UpdateOffset, lastOffset UpdateOffset, lastEpoch uint64, err error)
}

// CompactionMetrics adiciona observabilidade opcional ao helper de compaction
// do update log em snapshot.
type CompactionMetrics interface {
	CompactUpdateLog(key DocumentKey, duration time.Duration, applied int, through UpdateOffset, lastEpoch uint64, err error)
}

type metricsContextKey struct{}

// Metrics agrega os hooks opcionais de replay/recovery/compaction do pacote.
type Metrics interface {
	ReplayMetrics
	RecoveryMetrics
	CompactionMetrics
}

// ContextWithMetrics anexa hooks opcionais de observabilidade ao contexto
// usado pelos helpers públicos de replay/recovery/compaction.
func ContextWithMetrics(ctx context.Context, metrics Metrics) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if metrics == nil {
		return ctx
	}
	return context.WithValue(ctx, metricsContextKey{}, metrics)
}

func metricsFromContext(ctx context.Context) Metrics {
	if ctx == nil {
		return nil
	}
	metrics, _ := ctx.Value(metricsContextKey{}).(Metrics)
	return metrics
}

func observeReplayUpdateLog(ctx context.Context, key DocumentKey, duration time.Duration, applied int, through UpdateOffset, lastEpoch uint64, err error) {
	observer, ok := metricsFromContext(ctx).(ReplayMetrics)
	if !ok {
		return
	}
	observer.ReplayUpdateLog(key, duration, applied, through, lastEpoch, err)
}

func observeRecoverSnapshot(ctx context.Context, key DocumentKey, duration time.Duration, updates int, checkpointThrough UpdateOffset, lastOffset UpdateOffset, lastEpoch uint64, err error) {
	observer, ok := metricsFromContext(ctx).(RecoveryMetrics)
	if !ok {
		return
	}
	observer.RecoverSnapshot(key, duration, updates, checkpointThrough, lastOffset, lastEpoch, err)
}

func observeCompactUpdateLog(ctx context.Context, key DocumentKey, duration time.Duration, applied int, through UpdateOffset, lastEpoch uint64, err error) {
	observer, ok := metricsFromContext(ctx).(CompactionMetrics)
	if !ok {
		return
	}
	observer.CompactUpdateLog(key, duration, applied, through, lastEpoch, err)
}
