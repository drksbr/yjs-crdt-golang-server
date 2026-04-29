package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestReplayUpdateLogContextInvokesMetrics(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-metrics-replay"}
	store := &testSnapshotLogStore{
		records: []*UpdateLogRecord{
			{Key: key, Offset: 1, UpdateV1: buildGCOnlyUpdate(41, 1), Epoch: 3, StoredAt: time.Unix(1, 0).UTC()},
			{Key: key, Offset: 2, UpdateV1: buildGCOnlyUpdate(42, 1), Epoch: 4, StoredAt: time.Unix(2, 0).UTC()},
		},
	}
	recorder := newRecordingStorageMetrics()
	ctx := ContextWithMetrics(context.Background(), recorder)

	result, err := ReplayUpdateLogContext(ctx, store, key, nil, 0, 0)
	if err != nil {
		t.Fatalf("ReplayUpdateLogContext() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("ReplayUpdateLogContext() = nil, want result")
	}

	snapshot := recorder.snapshot()
	if snapshot.replays[recordingReplayKey{result: "ok", applied: 2, through: 2, lastEpoch: 4}] != 1 {
		t.Fatalf("replay metrics = %#v, want single ok replay event", snapshot.replays)
	}
}

func TestRecoverSnapshotInvokesMetrics(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-metrics-recovery"}
	base := mustPersistedSnapshotFromUpdates(t, buildGCOnlyUpdate(43, 1))
	store := &testSnapshotLogStore{
		snapshot: &SnapshotRecord{
			Key:      key,
			Snapshot: base,
			Through:  7,
			Epoch:    9,
			StoredAt: time.Unix(10, 0).UTC(),
		},
		records: []*UpdateLogRecord{
			{Key: key, Offset: 8, UpdateV1: buildGCOnlyUpdate(44, 1), Epoch: 9, StoredAt: time.Unix(11, 0).UTC()},
			{Key: key, Offset: 9, UpdateV1: buildGCOnlyUpdate(45, 1), Epoch: 10, StoredAt: time.Unix(12, 0).UTC()},
		},
	}
	recorder := newRecordingStorageMetrics()
	ctx := ContextWithMetrics(context.Background(), recorder)

	result, err := RecoverSnapshot(ctx, store, store, key, 0, 0)
	if err != nil {
		t.Fatalf("RecoverSnapshot() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("RecoverSnapshot() = nil, want result")
	}

	snapshot := recorder.snapshot()
	if snapshot.recoveries[recordingRecoveryKey{
		result:            "ok",
		updates:           2,
		checkpointThrough: 7,
		lastOffset:        9,
		lastEpoch:         10,
	}] != 1 {
		t.Fatalf("recovery metrics = %#v, want single ok recovery event", snapshot.recoveries)
	}
}

func TestCompactUpdateLogContextInvokesMetricsOnError(t *testing.T) {
	t.Parallel()

	trimErr := errors.New("trim failed")
	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-metrics-compaction"}
	store := &testSnapshotLogStore{
		records: []*UpdateLogRecord{
			{Key: key, Offset: 3, UpdateV1: buildGCOnlyUpdate(46, 1), Epoch: 6, StoredAt: time.Unix(13, 0).UTC()},
		},
		trimErr: trimErr,
	}
	recorder := newRecordingStorageMetrics()
	ctx := ContextWithMetrics(context.Background(), recorder)

	result, err := CompactUpdateLogContext(ctx, store, key, nil, 0, 0)
	if !errors.Is(err, trimErr) {
		t.Fatalf("CompactUpdateLogContext() error = %v, want %v", err, trimErr)
	}
	if result == nil {
		t.Fatal("CompactUpdateLogContext() = nil, want result")
	}

	snapshot := recorder.snapshot()
	if snapshot.compactions[recordingCompactionKey{result: "error", applied: 1, through: 3, lastEpoch: 6}] != 1 {
		t.Fatalf("compaction metrics = %#v, want single error compaction event", snapshot.compactions)
	}
}

type recordingStorageMetrics struct {
	mu          sync.Mutex
	replays     map[recordingReplayKey]int
	recoveries  map[recordingRecoveryKey]int
	compactions map[recordingCompactionKey]int
}

type recordingStorageMetricsSnapshot struct {
	replays     map[recordingReplayKey]int
	recoveries  map[recordingRecoveryKey]int
	compactions map[recordingCompactionKey]int
}

type recordingReplayKey struct {
	result    string
	applied   int
	through   UpdateOffset
	lastEpoch uint64
}

type recordingRecoveryKey struct {
	result            string
	updates           int
	checkpointThrough UpdateOffset
	lastOffset        UpdateOffset
	lastEpoch         uint64
}

type recordingCompactionKey struct {
	result    string
	applied   int
	through   UpdateOffset
	lastEpoch uint64
}

func newRecordingStorageMetrics() *recordingStorageMetrics {
	return &recordingStorageMetrics{
		replays:     make(map[recordingReplayKey]int),
		recoveries:  make(map[recordingRecoveryKey]int),
		compactions: make(map[recordingCompactionKey]int),
	}
}

func (r *recordingStorageMetrics) ReplayUpdateLog(_ DocumentKey, _ time.Duration, applied int, through UpdateOffset, lastEpoch uint64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replays[recordingReplayKey{
		result:    recordingResultLabel(err),
		applied:   applied,
		through:   through,
		lastEpoch: lastEpoch,
	}]++
}

func (r *recordingStorageMetrics) RecoverSnapshot(_ DocumentKey, _ time.Duration, updates int, checkpointThrough UpdateOffset, lastOffset UpdateOffset, lastEpoch uint64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recoveries[recordingRecoveryKey{
		result:            recordingResultLabel(err),
		updates:           updates,
		checkpointThrough: checkpointThrough,
		lastOffset:        lastOffset,
		lastEpoch:         lastEpoch,
	}]++
}

func (r *recordingStorageMetrics) CompactUpdateLog(_ DocumentKey, _ time.Duration, applied int, through UpdateOffset, lastEpoch uint64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.compactions[recordingCompactionKey{
		result:    recordingResultLabel(err),
		applied:   applied,
		through:   through,
		lastEpoch: lastEpoch,
	}]++
}

func (r *recordingStorageMetrics) snapshot() recordingStorageMetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	replays := make(map[recordingReplayKey]int, len(r.replays))
	for key, total := range r.replays {
		replays[key] = total
	}
	recoveries := make(map[recordingRecoveryKey]int, len(r.recoveries))
	for key, total := range r.recoveries {
		recoveries[key] = total
	}
	compactions := make(map[recordingCompactionKey]int, len(r.compactions))
	for key, total := range r.compactions {
		compactions[key] = total
	}

	return recordingStorageMetricsSnapshot{
		replays:     replays,
		recoveries:  recoveries,
		compactions: compactions,
	}
}

func recordingResultLabel(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}
