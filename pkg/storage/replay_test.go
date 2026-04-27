package storage

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
	"yjs-go-bridge/internal/yupdate"
	"yjs-go-bridge/pkg/yjsbridge"
)

type testSnapshotLogStore struct {
	snapshot *SnapshotRecord
	records  []*UpdateLogRecord

	loadErr error
	listErr error
	saveErr error
	trimErr error

	listCalls int
	saveCalls int
	trimCalls int

	trimmedKey     DocumentKey
	trimmedThrough UpdateOffset
}

var _ SnapshotLogStore = (*testSnapshotLogStore)(nil)

func (s *testSnapshotLogStore) SaveSnapshot(_ context.Context, key DocumentKey, snapshot *yjsbridge.PersistedSnapshot) (*SnapshotRecord, error) {
	s.saveCalls++
	if s.saveErr != nil {
		return nil, s.saveErr
	}

	record := &SnapshotRecord{
		Key:      key,
		Snapshot: snapshot.Clone(),
		StoredAt: time.Unix(500, 0).UTC(),
	}
	s.snapshot = record.Clone()
	return record, nil
}

func (s *testSnapshotLogStore) LoadSnapshot(_ context.Context, _ DocumentKey) (*SnapshotRecord, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.snapshot.Clone(), nil
}

func (s *testSnapshotLogStore) AppendUpdate(context.Context, DocumentKey, []byte) (*UpdateLogRecord, error) {
	return nil, nil
}

func (s *testSnapshotLogStore) ListUpdates(_ context.Context, _ DocumentKey, after UpdateOffset, limit int) ([]*UpdateLogRecord, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}

	out := make([]*UpdateLogRecord, 0)
	for _, record := range s.records {
		if record == nil {
			out = append(out, nil)
		} else if record.Offset > after {
			out = append(out, record.Clone())
		}
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out, nil
}

func (s *testSnapshotLogStore) TrimUpdates(_ context.Context, key DocumentKey, through UpdateOffset) error {
	s.trimCalls++
	s.trimmedKey = key
	s.trimmedThrough = through
	if s.trimErr != nil {
		return s.trimErr
	}
	return nil
}

func TestReplaySnapshot(t *testing.T) {
	t.Parallel()

	baseUpdate := buildGCOnlyUpdate(1, 2)
	incremental := buildGCOnlyUpdate(4, 1)
	base := mustPersistedSnapshotFromUpdates(t, baseUpdate)

	replayed, err := ReplaySnapshot(context.Background(), base, &UpdateLogRecord{
		Key:      DocumentKey{Namespace: "team-a", DocumentID: "doc-1"},
		Offset:   7,
		UpdateV1: incremental,
	})
	if err != nil {
		t.Fatalf("ReplaySnapshot() unexpected error: %v", err)
	}

	want := mustPersistedSnapshotFromUpdates(t, baseUpdate, incremental)
	if !bytes.Equal(replayed.UpdateV1, want.UpdateV1) {
		t.Fatalf("ReplaySnapshot().UpdateV1 = %v, want %v", replayed.UpdateV1, want.UpdateV1)
	}

	base.UpdateV1[0] ^= 0xff
	if bytes.Equal(replayed.UpdateV1, base.UpdateV1) {
		t.Fatal("ReplaySnapshot() retained mutable base payload reference")
	}
}

func TestReplaySnapshotErrors(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		base    *yjsbridge.PersistedSnapshot
		updates []*UpdateLogRecord
		wantErr error
	}{
		{
			name:    "canceled_context",
			ctx:     ctx,
			updates: []*UpdateLogRecord{{Key: DocumentKey{DocumentID: "doc-1"}, UpdateV1: buildGCOnlyUpdate(1, 1)}},
			wantErr: context.Canceled,
		},
		{
			name:    "invalid_record",
			ctx:     context.Background(),
			updates: []*UpdateLogRecord{{}},
			wantErr: ErrInvalidDocumentKey,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ReplaySnapshot(tt.ctx, tt.base, tt.updates...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ReplaySnapshot() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestReplayUpdateLogContextRebuildsSnapshotFromBaseAndTail(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-2"}
	baseUpdate := buildGCOnlyUpdate(7, 1)
	base := mustPersistedSnapshotFromUpdates(t, baseUpdate)
	tailLeft := buildGCOnlyUpdate(8, 2)
	tailRight := buildGCOnlyUpdate(9, 1)

	store := &testSnapshotLogStore{
		records: []*UpdateLogRecord{
			{Key: key, Offset: 11, UpdateV1: tailLeft, StoredAt: time.Unix(11, 0).UTC()},
			{Key: key, Offset: 15, UpdateV1: tailRight, StoredAt: time.Unix(15, 0).UTC()},
		},
	}

	got, err := ReplayUpdateLogContext(context.Background(), store, key, base, 10, 1)
	if err != nil {
		t.Fatalf("ReplayUpdateLogContext() unexpected error: %v", err)
	}

	want := mustPersistedSnapshotFromUpdates(t, baseUpdate, tailLeft, tailRight)
	if got.Applied != 2 {
		t.Fatalf("ReplayUpdateLogContext().Applied = %d, want 2", got.Applied)
	}
	if got.Through != 15 {
		t.Fatalf("ReplayUpdateLogContext().Through = %d, want 15", got.Through)
	}
	if !bytes.Equal(got.Snapshot.UpdateV1, want.UpdateV1) {
		t.Fatalf("ReplayUpdateLogContext().Snapshot.UpdateV1 = %v, want %v", got.Snapshot.UpdateV1, want.UpdateV1)
	}
	if store.listCalls != 3 {
		t.Fatalf("ListUpdates() calls = %d, want 3", store.listCalls)
	}
}

func TestReplayUpdateLogRebuildsFromNilBase(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-3"}
	left := buildGCOnlyUpdate(12, 1)
	right := buildGCOnlyUpdate(13, 3)

	store := &testSnapshotLogStore{
		records: []*UpdateLogRecord{
			{Key: key, Offset: 1, UpdateV1: left},
			{Key: key, Offset: 2, UpdateV1: right},
		},
	}

	got, err := ReplayUpdateLog(store, key, nil, 0, 0)
	if err != nil {
		t.Fatalf("ReplayUpdateLog() unexpected error: %v", err)
	}

	want := mustPersistedSnapshotFromUpdates(t, left, right)
	if got.Applied != 2 {
		t.Fatalf("ReplayUpdateLog().Applied = %d, want 2", got.Applied)
	}
	if got.Through != 2 {
		t.Fatalf("ReplayUpdateLog().Through = %d, want 2", got.Through)
	}
	if !bytes.Equal(got.Snapshot.UpdateV1, want.UpdateV1) {
		t.Fatalf("ReplayUpdateLog().Snapshot.UpdateV1 = %v, want %v", got.Snapshot.UpdateV1, want.UpdateV1)
	}
}

func TestReplayUpdateLogContextNoTailReturnsIndependentSnapshot(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-4"}
	base := mustPersistedSnapshotFromUpdates(t, buildGCOnlyUpdate(14, 2))

	got, err := ReplayUpdateLogContext(context.Background(), &testSnapshotLogStore{}, key, base, 33, 2)
	if err != nil {
		t.Fatalf("ReplayUpdateLogContext() unexpected error: %v", err)
	}
	if got.Applied != 0 {
		t.Fatalf("ReplayUpdateLogContext().Applied = %d, want 0", got.Applied)
	}
	if got.Through != 33 {
		t.Fatalf("ReplayUpdateLogContext().Through = %d, want 33", got.Through)
	}
	if !bytes.Equal(got.Snapshot.UpdateV1, base.UpdateV1) {
		t.Fatalf("ReplayUpdateLogContext().Snapshot.UpdateV1 = %v, want %v", got.Snapshot.UpdateV1, base.UpdateV1)
	}

	got.Snapshot.UpdateV1[0] ^= 0xff
	if bytes.Equal(got.Snapshot.UpdateV1, base.UpdateV1) {
		t.Fatal("ReplayUpdateLogContext() reused base snapshot bytes")
	}
}

func TestReplayUpdateLogContextRejectsInconsistentTail(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-5"}
	otherKey := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-6"}

	tests := []struct {
		name    string
		store   UpdateLogStore
		key     DocumentKey
		wantErr error
	}{
		{
			name:    "nil_store",
			store:   nil,
			key:     key,
			wantErr: ErrNilUpdateLogStore,
		},
		{
			name:    "invalid_key",
			store:   &testSnapshotLogStore{},
			key:     DocumentKey{},
			wantErr: ErrInvalidDocumentKey,
		},
		{
			name: "mismatched_key",
			store: &testSnapshotLogStore{
				records: []*UpdateLogRecord{
					{Key: otherKey, Offset: 1, UpdateV1: buildGCOnlyUpdate(15, 1)},
				},
			},
			key:     key,
			wantErr: ErrUpdateLogKeyMismatch,
		},
		{
			name: "out_of_order_offsets",
			store: &testSnapshotLogStore{
				records: []*UpdateLogRecord{
					{Key: key, Offset: 3, UpdateV1: buildGCOnlyUpdate(16, 1)},
					{Key: key, Offset: 2, UpdateV1: buildGCOnlyUpdate(17, 1)},
				},
			},
			key:     key,
			wantErr: ErrUpdateLogOffsetsOutOfOrder,
		},
		{
			name: "invalid_payload",
			store: &testSnapshotLogStore{
				records: []*UpdateLogRecord{
					{Key: key, Offset: 1, UpdateV1: nil},
				},
			},
			key:     key,
			wantErr: ErrInvalidUpdatePayload,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ReplayUpdateLogContext(context.Background(), tt.store, tt.key, nil, 0, 0)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ReplayUpdateLogContext() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRecoverSnapshotPagesTailAndClonesUpdates(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-7"}
	baseUpdate := buildGCOnlyUpdate(20, 1)
	left := buildGCOnlyUpdate(21, 2)
	right := buildGCOnlyUpdate(22, 1)

	store := &testSnapshotLogStore{
		snapshot: &SnapshotRecord{
			Key:      key,
			Snapshot: mustPersistedSnapshotFromUpdates(t, baseUpdate),
			StoredAt: time.Unix(100, 0).UTC(),
		},
		records: []*UpdateLogRecord{
			{Key: key, Offset: 7, UpdateV1: left},
			{Key: key, Offset: 9, UpdateV1: right},
		},
	}

	got, err := RecoverSnapshot(context.Background(), store, store, key, 5, 1)
	if err != nil {
		t.Fatalf("RecoverSnapshot() unexpected error: %v", err)
	}

	want := mustPersistedSnapshotFromUpdates(t, baseUpdate, left, right)
	if got.LastOffset != 9 {
		t.Fatalf("RecoverSnapshot().LastOffset = %d, want 9", got.LastOffset)
	}
	if len(got.Updates) != 2 {
		t.Fatalf("len(RecoverSnapshot().Updates) = %d, want 2", len(got.Updates))
	}
	if !bytes.Equal(got.Snapshot.UpdateV1, want.UpdateV1) {
		t.Fatalf("RecoverSnapshot().Snapshot.UpdateV1 = %v, want %v", got.Snapshot.UpdateV1, want.UpdateV1)
	}
	if store.listCalls != 3 {
		t.Fatalf("ListUpdates() calls = %d, want 3", store.listCalls)
	}

	got.Updates[0].UpdateV1[0] ^= 0xff
	reloaded, err := RecoverSnapshot(context.Background(), store, store, key, 5, 1)
	if err != nil {
		t.Fatalf("RecoverSnapshot() reload unexpected error: %v", err)
	}
	if bytes.Equal(got.Updates[0].UpdateV1, reloaded.Updates[0].UpdateV1) {
		t.Fatal("RecoverSnapshot() leaked mutable update slice")
	}
}

func TestRecoverSnapshotWithoutSnapshotOrUpdates(t *testing.T) {
	t.Parallel()

	key := DocumentKey{DocumentID: "doc-8"}

	got, err := RecoverSnapshot(context.Background(), nil, nil, key, 9, 0)
	if err != nil {
		t.Fatalf("RecoverSnapshot() unexpected error: %v", err)
	}
	if got.LastOffset != 9 {
		t.Fatalf("RecoverSnapshot().LastOffset = %d, want 9", got.LastOffset)
	}
	if got.Snapshot == nil || !got.Snapshot.IsEmpty() {
		t.Fatalf("RecoverSnapshot().Snapshot = %#v, want empty snapshot", got.Snapshot)
	}
}

func TestCompactUpdateLogContextSavesSnapshotAndTrimsTail(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-9"}
	baseUpdate := buildGCOnlyUpdate(30, 1)
	base := mustPersistedSnapshotFromUpdates(t, baseUpdate)
	tail := buildGCOnlyUpdate(31, 2)

	store := &testSnapshotLogStore{
		records: []*UpdateLogRecord{
			{Key: key, Offset: 8, UpdateV1: tail},
		},
	}

	got, err := CompactUpdateLogContext(context.Background(), store, key, base, 5, 1)
	if err != nil {
		t.Fatalf("CompactUpdateLogContext() unexpected error: %v", err)
	}

	want := mustPersistedSnapshotFromUpdates(t, baseUpdate, tail)
	if got.Record == nil {
		t.Fatal("CompactUpdateLogContext().Record = nil, want non-nil")
	}
	if got.Applied != 1 {
		t.Fatalf("CompactUpdateLogContext().Applied = %d, want 1", got.Applied)
	}
	if got.Through != 8 {
		t.Fatalf("CompactUpdateLogContext().Through = %d, want 8", got.Through)
	}
	if !bytes.Equal(got.Snapshot.UpdateV1, want.UpdateV1) {
		t.Fatalf("CompactUpdateLogContext().Snapshot.UpdateV1 = %v, want %v", got.Snapshot.UpdateV1, want.UpdateV1)
	}
	if !bytes.Equal(got.Record.Snapshot.UpdateV1, want.UpdateV1) {
		t.Fatalf("CompactUpdateLogContext().Record.Snapshot.UpdateV1 = %v, want %v", got.Record.Snapshot.UpdateV1, want.UpdateV1)
	}
	if store.saveCalls != 1 {
		t.Fatalf("SaveSnapshot() calls = %d, want 1", store.saveCalls)
	}
	if store.trimCalls != 1 {
		t.Fatalf("TrimUpdates() calls = %d, want 1", store.trimCalls)
	}
	if store.trimmedKey != key {
		t.Fatalf("TrimUpdates() key = %#v, want %#v", store.trimmedKey, key)
	}
	if store.trimmedThrough != 8 {
		t.Fatalf("TrimUpdates() through = %d, want 8", store.trimmedThrough)
	}
}

func TestCompactUpdateLogContextNoTailDoesNotPersistOrTrim(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-10"}
	base := mustPersistedSnapshotFromUpdates(t, buildGCOnlyUpdate(40, 1))
	store := &testSnapshotLogStore{}

	got, err := CompactUpdateLogContext(context.Background(), store, key, base, 12, 3)
	if err != nil {
		t.Fatalf("CompactUpdateLogContext() unexpected error: %v", err)
	}
	if got.Record != nil {
		t.Fatalf("CompactUpdateLogContext().Record = %#v, want nil", got.Record)
	}
	if got.Applied != 0 {
		t.Fatalf("CompactUpdateLogContext().Applied = %d, want 0", got.Applied)
	}
	if got.Through != 12 {
		t.Fatalf("CompactUpdateLogContext().Through = %d, want 12", got.Through)
	}
	if !bytes.Equal(got.Snapshot.UpdateV1, base.UpdateV1) {
		t.Fatalf("CompactUpdateLogContext().Snapshot.UpdateV1 = %v, want %v", got.Snapshot.UpdateV1, base.UpdateV1)
	}
	if store.saveCalls != 0 {
		t.Fatalf("SaveSnapshot() calls = %d, want 0", store.saveCalls)
	}
	if store.trimCalls != 0 {
		t.Fatalf("TrimUpdates() calls = %d, want 0", store.trimCalls)
	}
}

func TestCompactUpdateLogContextReturnsPartialResultOnTrimError(t *testing.T) {
	t.Parallel()

	key := DocumentKey{Namespace: "tenant-a", DocumentID: "doc-11"}
	store := &testSnapshotLogStore{
		records: []*UpdateLogRecord{
			{Key: key, Offset: 4, UpdateV1: buildGCOnlyUpdate(50, 1)},
		},
		trimErr: errors.New("boom"),
	}

	got, err := CompactUpdateLogContext(context.Background(), store, key, nil, 0, 0)
	if err == nil {
		t.Fatal("CompactUpdateLogContext() error = nil, want non-nil")
	}
	if got == nil {
		t.Fatal("CompactUpdateLogContext() result = nil, want partial result")
	}
	if got.Record == nil {
		t.Fatal("CompactUpdateLogContext().Record = nil, want saved record despite trim error")
	}
	if got.Through != 4 {
		t.Fatalf("CompactUpdateLogContext().Through = %d, want 4", got.Through)
	}
	if store.saveCalls != 1 {
		t.Fatalf("SaveSnapshot() calls = %d, want 1", store.saveCalls)
	}
	if store.trimCalls != 1 {
		t.Fatalf("TrimUpdates() calls = %d, want 1", store.trimCalls)
	}
}

func mustPersistedSnapshotFromUpdates(t *testing.T, updates ...[]byte) *yjsbridge.PersistedSnapshot {
	t.Helper()

	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates(updates...)
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}
	return snapshot
}

func buildGCOnlyUpdate(client, length uint32) []byte {
	update := varint.Append(nil, 1)
	update = varint.Append(update, 1)
	update = varint.Append(update, client)
	update = varint.Append(update, 0)
	update = append(update, 0)
	update = varint.Append(update, length)
	return append(update, yupdate.EncodeDeleteSetBlockV1(ytypes.NewDeleteSet())...)
}
