package yprotocol

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestProviderMetricsObserveRecoveryPersistAndRoomLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := storage.DocumentKey{
		Namespace:  "tests",
		DocumentID: "provider-metrics-recovery-persist",
	}
	store := memory.New()
	baseUpdate := buildGCOnlyUpdate(51, 1)
	tailUpdate := buildGCOnlyUpdate(52, 1)
	baseSnapshot, err := yjsbridge.PersistedSnapshotFromUpdate(baseUpdate)
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdate(baseUpdate) unexpected error: %v", err)
	}
	if _, err := store.SaveSnapshot(ctx, key, baseSnapshot); err != nil {
		t.Fatalf("store.SaveSnapshot() unexpected error: %v", err)
	}
	if _, err := store.AppendUpdate(ctx, key, tailUpdate); err != nil {
		t.Fatalf("store.AppendUpdate() unexpected error: %v", err)
	}

	providerRecorder := newRecordingProviderMetrics()
	storageRecorder := newRecordingProviderStorageMetrics()
	provider := NewProvider(ProviderConfig{
		Store:          store,
		Metrics:        providerRecorder,
		StorageMetrics: storageRecorder,
	})

	conn, err := provider.Open(ctx, key, "conn-a", 1001)
	if err != nil {
		t.Fatalf("provider.Open() unexpected error: %v", err)
	}
	if _, err := conn.Persist(ctx); err != nil {
		t.Fatalf("conn.Persist() unexpected error: %v", err)
	}
	if _, err := conn.Close(); err != nil {
		t.Fatalf("conn.Close() unexpected error: %v", err)
	}

	providerSnapshot := providerRecorder.snapshot()
	if providerSnapshot.roomsOpened != 1 {
		t.Fatalf("roomsOpened = %d, want 1", providerSnapshot.roomsOpened)
	}
	if providerSnapshot.roomsClosed != 1 {
		t.Fatalf("roomsClosed = %d, want 1", providerSnapshot.roomsClosed)
	}
	if providerSnapshot.persists[recordingProviderPersistKey{
		result:    "ok",
		through:   1,
		compacted: 1,
	}] != 1 {
		t.Fatalf("persist metrics = %#v, want single ok persist event", providerSnapshot.persists)
	}

	storageSnapshot := storageRecorder.snapshot()
	if storageSnapshot.recoveries[recordingProviderRecoveryKey{
		result:            "ok",
		updates:           1,
		checkpointThrough: 0,
		lastOffset:        1,
		lastEpoch:         0,
	}] != 1 {
		t.Fatalf("storage recovery metrics = %#v, want single ok recovery event", storageSnapshot.recoveries)
	}
}

func TestProviderMetricsObserveAuthorityLossOnRevalidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := storage.DocumentKey{
		Namespace:  "tests",
		DocumentID: "provider-metrics-authority-loss",
	}
	store := memory.New()
	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	lookup, err := ycluster.NewStorageOwnerLookup("node-a", resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup() unexpected error: %v", err)
	}
	recorder := newRecordingProviderMetrics()
	provider := NewProvider(ProviderConfig{
		Store:   store,
		Metrics: recorder,
		ResolveAuthorityFence: func(ctx context.Context, key storage.DocumentKey) (*storage.AuthorityFence, error) {
			return ycluster.ResolveStorageAuthorityFence(ctx, lookup, key)
		},
	})
	seedAuthoritativeDocument(t, ctx, store, resolver, key, "node-a", 1, "lease-node-a")

	conn, err := provider.Open(ctx, key, "conn-a", 1002)
	if err != nil {
		t.Fatalf("provider.Open() unexpected error: %v", err)
	}

	handoffAuthority(t, ctx, store, resolver, key, "lease-node-a", "node-b", 2, "lease-node-b")
	if err := conn.RevalidateAuthority(ctx); !errors.Is(err, ErrAuthorityLost) {
		t.Fatalf("conn.RevalidateAuthority() error = %v, want %v", err, ErrAuthorityLost)
	}

	snapshot := recorder.snapshot()
	if snapshot.authorityRevalidations["error"] != 1 {
		t.Fatalf("authorityRevalidations = %#v, want single error revalidation", snapshot.authorityRevalidations)
	}
	if snapshot.authorityLosses[authorityLossStageRevalidate] != 1 {
		t.Fatalf("authorityLosses = %#v, want single revalidate authority loss", snapshot.authorityLosses)
	}
}

type recordingProviderMetrics struct {
	mu                     sync.Mutex
	roomsOpened            int
	roomsClosed            int
	persists               map[recordingProviderPersistKey]int
	authorityRevalidations map[string]int
	authorityLosses        map[string]int
}

type recordingProviderMetricsSnapshot struct {
	roomsOpened            int
	roomsClosed            int
	persists               map[recordingProviderPersistKey]int
	authorityRevalidations map[string]int
	authorityLosses        map[string]int
}

type recordingProviderPersistKey struct {
	result    string
	through   storage.UpdateOffset
	compacted storage.UpdateOffset
}

func newRecordingProviderMetrics() *recordingProviderMetrics {
	return &recordingProviderMetrics{
		persists:               make(map[recordingProviderPersistKey]int),
		authorityRevalidations: make(map[string]int),
		authorityLosses:        make(map[string]int),
	}
}

func (r *recordingProviderMetrics) RoomOpened(storage.DocumentKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roomsOpened++
}

func (r *recordingProviderMetrics) RoomClosed(storage.DocumentKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roomsClosed++
}

func (r *recordingProviderMetrics) Persist(_ storage.DocumentKey, _ time.Duration, through storage.UpdateOffset, compacted storage.UpdateOffset, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.persists[recordingProviderPersistKey{
		result:    recordingProviderResultLabel(err),
		through:   through,
		compacted: compacted,
	}]++
}

func (r *recordingProviderMetrics) AuthorityRevalidation(_ storage.DocumentKey, _ time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.authorityRevalidations[recordingProviderResultLabel(err)]++
}

func (r *recordingProviderMetrics) AuthorityLost(_ storage.DocumentKey, stage string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.authorityLosses[stage]++
}

func (r *recordingProviderMetrics) snapshot() recordingProviderMetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	persists := make(map[recordingProviderPersistKey]int, len(r.persists))
	for key, total := range r.persists {
		persists[key] = total
	}
	revalidations := make(map[string]int, len(r.authorityRevalidations))
	for result, total := range r.authorityRevalidations {
		revalidations[result] = total
	}
	losses := make(map[string]int, len(r.authorityLosses))
	for stage, total := range r.authorityLosses {
		losses[stage] = total
	}

	return recordingProviderMetricsSnapshot{
		roomsOpened:            r.roomsOpened,
		roomsClosed:            r.roomsClosed,
		persists:               persists,
		authorityRevalidations: revalidations,
		authorityLosses:        losses,
	}
}

type recordingProviderStorageMetrics struct {
	mu         sync.Mutex
	recoveries map[recordingProviderRecoveryKey]int
}

type recordingProviderStorageMetricsSnapshot struct {
	recoveries map[recordingProviderRecoveryKey]int
}

type recordingProviderRecoveryKey struct {
	result            string
	updates           int
	checkpointThrough storage.UpdateOffset
	lastOffset        storage.UpdateOffset
	lastEpoch         uint64
}

func newRecordingProviderStorageMetrics() *recordingProviderStorageMetrics {
	return &recordingProviderStorageMetrics{
		recoveries: make(map[recordingProviderRecoveryKey]int),
	}
}

func (r *recordingProviderStorageMetrics) ReplayUpdateLog(storage.DocumentKey, time.Duration, int, storage.UpdateOffset, uint64, error) {
}

func (r *recordingProviderStorageMetrics) RecoverSnapshot(_ storage.DocumentKey, _ time.Duration, updates int, checkpointThrough storage.UpdateOffset, lastOffset storage.UpdateOffset, lastEpoch uint64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recoveries[recordingProviderRecoveryKey{
		result:            recordingProviderResultLabel(err),
		updates:           updates,
		checkpointThrough: checkpointThrough,
		lastOffset:        lastOffset,
		lastEpoch:         lastEpoch,
	}]++
}

func (r *recordingProviderStorageMetrics) CompactUpdateLog(storage.DocumentKey, time.Duration, int, storage.UpdateOffset, uint64, error) {
}

func (r *recordingProviderStorageMetrics) snapshot() recordingProviderStorageMetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	recoveries := make(map[recordingProviderRecoveryKey]int, len(r.recoveries))
	for key, total := range r.recoveries {
		recoveries[key] = total
	}
	return recordingProviderStorageMetricsSnapshot{recoveries: recoveries}
}

func recordingProviderResultLabel(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}
