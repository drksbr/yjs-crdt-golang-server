package storage

import (
	"context"
	"errors"
	"fmt"

	"yjs-go-bridge/pkg/yjsbridge"
)

var (
	// ErrNilUpdateLogStore signals that a replay helper received a nil update log store.
	ErrNilUpdateLogStore = errors.New("storage: update log store obrigatorio")
	// ErrNilSnapshotLogStore signals that a compaction helper received a nil snapshot+log store.
	ErrNilSnapshotLogStore = errors.New("storage: snapshot log store obrigatorio")
	// ErrUpdateLogKeyMismatch signals that replay observed a log record for another document key.
	ErrUpdateLogKeyMismatch = errors.New("storage: update log key inconsistente")
	// ErrUpdateLogOffsetsOutOfOrder signals that replay observed non-monotonic log offsets.
	ErrUpdateLogOffsetsOutOfOrder = errors.New("storage: update log offsets fora de ordem")
)

// SnapshotLogStore narrows the contracts required to persist a compacted
// snapshot and trim the corresponding update log tail.
type SnapshotLogStore interface {
	SnapshotStore
	UpdateLogStore
}

// RecoveryResult materializes the result of loading a base snapshot and replaying
// the incremental update log tail for the same document.
type RecoveryResult struct {
	Snapshot   *yjsbridge.PersistedSnapshot
	Updates    []*UpdateLogRecord
	LastOffset UpdateOffset
}

// UpdateLogReplayResult describes the snapshot rebuilt from a base cut plus a
// paginated update log tail.
//
// Through stores the highest applied offset. When no new log records are
// applied, it remains equal to the input `after`.
type UpdateLogReplayResult struct {
	Snapshot *yjsbridge.PersistedSnapshot
	Through  UpdateOffset
	Applied  int
}

// UpdateLogCompactionResult describes the outcome of compacting a snapshot plus
// its update log tail.
//
// Record stays nil when there were no new updates to persist or trim.
type UpdateLogCompactionResult struct {
	Snapshot *yjsbridge.PersistedSnapshot
	Record   *SnapshotRecord
	Through  UpdateOffset
	Applied  int
}

// ReplaySnapshot applies an ordered set of update log records over a base
// snapshot and returns the merged persisted snapshot.
//
// `ctx == nil` is treated as `context.Background()`.
// `base == nil` is treated as an empty document.
//
// The helper keeps the record order as received. It validates individual
// records, but does not infer document ownership or reorder offsets.
func ReplaySnapshot(ctx context.Context, base *yjsbridge.PersistedSnapshot, updates ...*UpdateLogRecord) (*yjsbridge.PersistedSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	currentUpdate, err := yjsbridge.EncodePersistedSnapshotV1(base)
	if err != nil {
		return nil, err
	}
	if len(updates) == 0 {
		return yjsbridge.DecodePersistedSnapshotV1Context(ctx, currentUpdate)
	}

	payloads := make([][]byte, 0, len(updates)+1)
	payloads = append(payloads, currentUpdate)

	for idx, record := range updates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if record == nil {
			return nil, fmt.Errorf("%w: record %d nil", ErrInvalidUpdatePayload, idx)
		}
		if err := record.Validate(); err != nil {
			return nil, fmt.Errorf("update log record %d: %w", idx, err)
		}
		payloads = append(payloads, record.UpdateV1)
	}

	merged, err := yjsbridge.MergeUpdatesContext(ctx, payloads...)
	if err != nil {
		return nil, err
	}
	return yjsbridge.DecodePersistedSnapshotV1Context(ctx, merged)
}

// ReplayUpdateLog rebuilds a persisted snapshot from a base snapshot plus the
// log tail stored strictly after `after`.
func ReplayUpdateLog(store UpdateLogStore, key DocumentKey, base *yjsbridge.PersistedSnapshot, after UpdateOffset, limit int) (*UpdateLogReplayResult, error) {
	return ReplayUpdateLogContext(context.Background(), store, key, base, after, limit)
}

// ReplayUpdateLogContext rebuilds a persisted snapshot from a base snapshot plus
// the log tail stored strictly after `after`.
//
// `ctx == nil` is treated as `context.Background()`.
//
// The helper paginates through `ListUpdates`, validates that every record
// belongs to `key`, and requires strictly increasing offsets across pages.
// `limit <= 0` is passed through to every `ListUpdates` call.
func ReplayUpdateLogContext(ctx context.Context, store UpdateLogStore, key DocumentKey, base *yjsbridge.PersistedSnapshot, after UpdateOffset, limit int) (*UpdateLogReplayResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if store == nil {
		return nil, ErrNilUpdateLogStore
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}

	currentUpdate, err := yjsbridge.EncodePersistedSnapshotV1(base)
	if err != nil {
		return nil, err
	}

	result := &UpdateLogReplayResult{
		Through: after,
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		records, err := store.ListUpdates(ctx, key, result.Through, limit)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			break
		}

		currentUpdate, err = replayUpdateBatchContext(ctx, key, currentUpdate, result, records)
		if err != nil {
			return nil, err
		}
	}

	snapshot, err := yjsbridge.DecodePersistedSnapshotV1Context(ctx, currentUpdate)
	if err != nil {
		return nil, err
	}
	result.Snapshot = snapshot
	return result, nil
}

// RecoverSnapshot loads the known base snapshot for the document, pages through
// the update log tail after `after`, and replays the result in order.
//
// `ctx == nil` is treated as `context.Background()`.
// `snapshots == nil` is treated as no base snapshot.
// `updates == nil` returns only the loaded base snapshot.
func RecoverSnapshot(
	ctx context.Context,
	snapshots SnapshotStore,
	updates UpdateLogStore,
	key DocumentKey,
	after UpdateOffset,
	limit int,
) (*RecoveryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}

	base, err := loadReplayBaseSnapshot(ctx, snapshots, key)
	if err != nil {
		return nil, err
	}

	result := &RecoveryResult{
		Snapshot:   base,
		LastOffset: after,
	}
	if updates == nil {
		return result, nil
	}

	tail, lastOffset, err := listReplayTailContext(ctx, updates, key, after, limit)
	if err != nil {
		return nil, err
	}
	if len(tail) == 0 {
		return result, nil
	}

	snapshot, err := ReplaySnapshot(ctx, base, tail...)
	if err != nil {
		return nil, err
	}

	return &RecoveryResult{
		Snapshot:   snapshot,
		Updates:    cloneUpdateLogRecords(tail),
		LastOffset: lastOffset,
	}, nil
}

// CompactUpdateLog replays and persists a compacted snapshot, then trims the
// corresponding log tail through the applied high-water mark.
func CompactUpdateLog(store SnapshotLogStore, key DocumentKey, base *yjsbridge.PersistedSnapshot, after UpdateOffset, limit int) (*UpdateLogCompactionResult, error) {
	return CompactUpdateLogContext(context.Background(), store, key, base, after, limit)
}

// CompactUpdateLogContext replays and persists a compacted snapshot, then trims
// the corresponding log tail through the applied high-water mark.
//
// `ctx == nil` is treated as `context.Background()`.
//
// Save + trim are intentionally sequenced but not transactional unless the
// backing store makes them so. If trimming fails after a successful save, the
// returned result still exposes the rebuilt snapshot and saved record together
// with the error.
func CompactUpdateLogContext(ctx context.Context, store SnapshotLogStore, key DocumentKey, base *yjsbridge.PersistedSnapshot, after UpdateOffset, limit int) (*UpdateLogCompactionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if store == nil {
		return nil, ErrNilSnapshotLogStore
	}

	replay, err := ReplayUpdateLogContext(ctx, store, key, base, after, limit)
	if err != nil {
		return nil, err
	}

	result := &UpdateLogCompactionResult{
		Snapshot: replay.Snapshot,
		Through:  replay.Through,
		Applied:  replay.Applied,
	}
	if replay.Applied == 0 {
		return result, nil
	}

	record, err := store.SaveSnapshot(ctx, key, replay.Snapshot)
	result.Record = record
	if err != nil {
		return result, fmt.Errorf("save compacted snapshot: %w", err)
	}
	if err := store.TrimUpdates(ctx, key, replay.Through); err != nil {
		return result, fmt.Errorf("trim compacted updates through %d: %w", replay.Through, err)
	}

	return result, nil
}

func loadReplayBaseSnapshot(ctx context.Context, snapshots SnapshotStore, key DocumentKey) (*yjsbridge.PersistedSnapshot, error) {
	if snapshots == nil {
		return yjsbridge.NewPersistedSnapshot(), nil
	}

	record, err := snapshots.LoadSnapshot(ctx, key)
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			return yjsbridge.NewPersistedSnapshot(), nil
		}
		return nil, err
	}
	if record == nil || record.Snapshot == nil {
		return yjsbridge.NewPersistedSnapshot(), nil
	}
	return ReplaySnapshot(ctx, record.Snapshot)
}

func listReplayTailContext(ctx context.Context, store UpdateLogStore, key DocumentKey, after UpdateOffset, limit int) ([]*UpdateLogRecord, UpdateOffset, error) {
	if store == nil {
		return nil, after, ErrNilUpdateLogStore
	}

	through := after
	tail := make([]*UpdateLogRecord, 0)

	for {
		if err := ctx.Err(); err != nil {
			return nil, after, err
		}

		records, err := store.ListUpdates(ctx, key, through, limit)
		if err != nil {
			return nil, after, err
		}
		if len(records) == 0 {
			break
		}

		for idx, record := range records {
			if record == nil {
				return nil, after, fmt.Errorf("%w: record %d nil", ErrInvalidUpdatePayload, idx)
			}
			if err := record.Validate(); err != nil {
				return nil, after, fmt.Errorf("update log record %d: %w", idx, err)
			}
			if record.Key != key {
				return nil, after, fmt.Errorf("%w: record %d key %#v", ErrUpdateLogKeyMismatch, idx, record.Key)
			}
			if record.Offset <= through {
				return nil, after, fmt.Errorf("%w: offset %d after %d", ErrUpdateLogOffsetsOutOfOrder, record.Offset, through)
			}

			tail = append(tail, record.Clone())
			through = record.Offset
		}
	}

	return tail, through, nil
}

func replayUpdateBatchContext(ctx context.Context, key DocumentKey, currentUpdate []byte, result *UpdateLogReplayResult, records []*UpdateLogRecord) ([]byte, error) {
	updates := make([][]byte, 0, len(records)+1)
	updates = append(updates, currentUpdate)

	for idx, record := range records {
		if record == nil {
			return nil, fmt.Errorf("%w: record %d nil", ErrInvalidUpdatePayload, idx)
		}
		if err := record.Validate(); err != nil {
			return nil, fmt.Errorf("update log record %d: %w", idx, err)
		}
		if record.Key != key {
			return nil, fmt.Errorf("%w: record %d key %#v", ErrUpdateLogKeyMismatch, idx, record.Key)
		}
		if record.Offset <= result.Through {
			return nil, fmt.Errorf("%w: offset %d after %d", ErrUpdateLogOffsetsOutOfOrder, record.Offset, result.Through)
		}

		updates = append(updates, record.UpdateV1)
		result.Through = record.Offset
		result.Applied++
	}

	return yjsbridge.MergeUpdatesContext(ctx, updates...)
}

func cloneUpdateLogRecords(records []*UpdateLogRecord) []*UpdateLogRecord {
	if len(records) == 0 {
		return nil
	}

	cloned := make([]*UpdateLogRecord, 0, len(records))
	for _, record := range records {
		if record == nil {
			cloned = append(cloned, nil)
			continue
		}
		cloned = append(cloned, record.Clone())
	}
	return cloned
}
