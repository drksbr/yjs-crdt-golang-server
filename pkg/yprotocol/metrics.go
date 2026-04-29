package yprotocol

import (
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

const (
	authorityLossStageOpen        = "open"
	authorityLossStageAppend      = "append"
	authorityLossStagePersistSave = "persist_save"
	authorityLossStagePersistTrim = "persist_trim"
	authorityLossStageRevalidate  = "revalidate"
)

// Metrics descreve hooks opcionais de observabilidade do runtime local do
// provider, com foco no lifecycle de rooms e autoridade.
type Metrics interface {
	RoomOpened(key storage.DocumentKey)
	RoomClosed(key storage.DocumentKey)
	Persist(key storage.DocumentKey, duration time.Duration, through storage.UpdateOffset, compacted storage.UpdateOffset, err error)
	AuthorityRevalidation(key storage.DocumentKey, duration time.Duration, err error)
	AuthorityLost(key storage.DocumentKey, stage string)
}

type noopMetrics struct{}

func (noopMetrics) RoomOpened(storage.DocumentKey) {}

func (noopMetrics) RoomClosed(storage.DocumentKey) {}

func (noopMetrics) Persist(storage.DocumentKey, time.Duration, storage.UpdateOffset, storage.UpdateOffset, error) {
}

func (noopMetrics) AuthorityRevalidation(storage.DocumentKey, time.Duration, error) {}

func (noopMetrics) AuthorityLost(storage.DocumentKey, string) {}

func normalizeMetrics(metrics Metrics) Metrics {
	if metrics == nil {
		return noopMetrics{}
	}
	return metrics
}

func observeRoomOpened(metrics Metrics, key storage.DocumentKey) {
	if metrics == nil {
		return
	}
	metrics.RoomOpened(key)
}

func observeRoomClosed(metrics Metrics, key storage.DocumentKey) {
	if metrics == nil {
		return
	}
	metrics.RoomClosed(key)
}

func observePersist(metrics Metrics, key storage.DocumentKey, duration time.Duration, through storage.UpdateOffset, compacted storage.UpdateOffset, err error) {
	if metrics == nil {
		return
	}
	metrics.Persist(key, duration, through, compacted, err)
}

func observeAuthorityRevalidation(metrics Metrics, key storage.DocumentKey, duration time.Duration, err error) {
	if metrics == nil {
		return
	}
	metrics.AuthorityRevalidation(key, duration, err)
}

func observeAuthorityLost(metrics Metrics, key storage.DocumentKey, stage string) {
	if metrics == nil {
		return
	}
	metrics.AuthorityLost(key, stage)
}
