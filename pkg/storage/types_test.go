package storage

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestDocumentKeyValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     DocumentKey
		wantErr error
	}{
		{
			name:    "valid_with_namespace",
			key:     DocumentKey{Namespace: "tenant-a", DocumentID: "doc-1"},
			wantErr: nil,
		},
		{
			name:    "valid_without_namespace",
			key:     DocumentKey{DocumentID: "doc-2"},
			wantErr: nil,
		},
		{
			name:    "trims_whitespace_document_id",
			key:     DocumentKey{DocumentID: "  doc-3  "},
			wantErr: nil,
		},
		{
			name:    "empty_document_id",
			key:     DocumentKey{Namespace: "tenant-a", DocumentID: ""},
			wantErr: ErrInvalidDocumentKey,
		},
		{
			name:    "whitespace_document_id",
			key:     DocumentKey{DocumentID: "   "},
			wantErr: ErrInvalidDocumentKey,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.key.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSnapshotRecordClone(t *testing.T) {
	t.Parallel()

	baseTime := time.Unix(123, 0).UTC()
	baseSnapshot := yjsbridge.NewPersistedSnapshot()
	baseSnapshot.UpdateV2 = []byte{0x04, 0x05, 0x06}
	baseSnapshot.UpdateV1 = []byte{0x01, 0x02, 0x03}
	baseSnapshot.Snapshot = yjsbridge.NewSnapshot()
	baseSnapshot.Snapshot.StateVector = map[uint32]uint32{1: 2, 3: 4}

	record := &SnapshotRecord{
		Key:      DocumentKey{Namespace: "tenant", DocumentID: "doc-1"},
		Snapshot: baseSnapshot,
		Through:  17,
		Epoch:    6,
		StoredAt: baseTime,
	}

	clone := record.Clone()
	if clone == nil {
		t.Fatal("Clone() = nil, want non-nil")
	}
	if !bytes.Equal(clone.Snapshot.UpdateV1, record.Snapshot.UpdateV1) {
		t.Fatalf("Clone().Snapshot.UpdateV1 = %v, want %v", clone.Snapshot.UpdateV1, record.Snapshot.UpdateV1)
	}
	if !bytes.Equal(clone.Snapshot.UpdateV2, record.Snapshot.UpdateV2) {
		t.Fatalf("Clone().Snapshot.UpdateV2 = %v, want %v", clone.Snapshot.UpdateV2, record.Snapshot.UpdateV2)
	}
	if clone.Key != record.Key {
		t.Fatalf("Clone().Key = %#v, want %#v", clone.Key, record.Key)
	}
	if !clone.StoredAt.Equal(baseTime) {
		t.Fatalf("Clone().StoredAt = %v, want %v", clone.StoredAt, baseTime)
	}
	if clone.Through != 17 {
		t.Fatalf("Clone().Through = %d, want 17", clone.Through)
	}
	if clone.Epoch != 6 {
		t.Fatalf("Clone().Epoch = %d, want 6", clone.Epoch)
	}

	clone.Key.DocumentID = "other"
	clone.Snapshot.UpdateV2[0] = 0x99
	clone.Snapshot.UpdateV1[0] = 0x99
	clone.Snapshot.Snapshot.StateVector[1] = 99

	if record.Key.DocumentID == clone.Key.DocumentID {
		t.Fatalf("clone mutou DocumentID do registro original")
	}
	if record.Snapshot.UpdateV1[0] == 0x99 {
		t.Fatalf("clone mutou bytes do snapshot original")
	}
	if record.Snapshot.UpdateV2[0] == 0x99 {
		t.Fatalf("clone mutou bytes V2 do snapshot original")
	}
	if record.Snapshot.Snapshot.StateVector[1] == 99 {
		t.Fatalf("clone mutou state vector do snapshot original")
	}

	if got := (*SnapshotRecord)(nil).Clone(); got != nil {
		t.Fatalf("(*SnapshotRecord)(nil).Clone() = %#v, want nil", got)
	}
}
