package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

type testDistributedStore struct{}

var _ DistributedStore = (*testDistributedStore)(nil)

func (testDistributedStore) SaveSnapshot(context.Context, DocumentKey, *yjsbridge.PersistedSnapshot) (*SnapshotRecord, error) {
	return nil, nil
}

func (testDistributedStore) LoadSnapshot(context.Context, DocumentKey) (*SnapshotRecord, error) {
	return nil, nil
}

func (testDistributedStore) AppendUpdate(context.Context, DocumentKey, []byte) (*UpdateLogRecord, error) {
	return nil, nil
}

func (testDistributedStore) ListUpdates(context.Context, DocumentKey, UpdateOffset, int) ([]*UpdateLogRecord, error) {
	return nil, nil
}

func (testDistributedStore) TrimUpdates(context.Context, DocumentKey, UpdateOffset) error {
	return nil
}

func (testDistributedStore) SavePlacement(context.Context, PlacementRecord) (*PlacementRecord, error) {
	return nil, nil
}

func (testDistributedStore) LoadPlacement(context.Context, DocumentKey) (*PlacementRecord, error) {
	return nil, nil
}

func (testDistributedStore) SaveLease(context.Context, LeaseRecord) (*LeaseRecord, error) {
	return nil, nil
}

func (testDistributedStore) LoadLease(context.Context, ShardID) (*LeaseRecord, error) {
	return nil, nil
}

func (testDistributedStore) ReleaseLease(context.Context, ShardID, string) error {
	return nil
}

func TestShardIDValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      ShardID
		wantErr error
	}{
		{
			name:    "valid",
			id:      ShardID("shard-1"),
			wantErr: nil,
		},
		{
			name:    "empty",
			id:      ShardID(""),
			wantErr: ErrInvalidShardID,
		},
		{
			name:    "whitespace",
			id:      ShardID("   "),
			wantErr: ErrInvalidShardID,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.id.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeIDValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      NodeID
		wantErr error
	}{
		{
			name:    "valid",
			id:      NodeID("node-a"),
			wantErr: nil,
		},
		{
			name:    "empty",
			id:      NodeID(""),
			wantErr: ErrInvalidNodeID,
		},
		{
			name:    "whitespace",
			id:      NodeID("  "),
			wantErr: ErrInvalidNodeID,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.id.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestOwnerInfoValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		owner   OwnerInfo
		wantErr error
	}{
		{
			name:    "valid",
			owner:   OwnerInfo{NodeID: NodeID("node-a"), Epoch: 3},
			wantErr: nil,
		},
		{
			name:    "invalid_node",
			owner:   OwnerInfo{Epoch: 1},
			wantErr: ErrInvalidOwnerInfo,
		},
		{
			name:    "invalid_epoch",
			owner:   OwnerInfo{NodeID: NodeID("node-a")},
			wantErr: ErrInvalidOwnerInfo,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.owner.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateLogRecordValidateAndClone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		record  UpdateLogRecord
		wantErr error
	}{
		{
			name: "valid",
			record: UpdateLogRecord{
				Key:      DocumentKey{Namespace: "tenant-a", DocumentID: "doc-1"},
				Offset:   7,
				UpdateV1: []byte{0x01, 0x02, 0x03},
				Epoch:    4,
				StoredAt: time.Unix(200, 0).UTC(),
			},
			wantErr: nil,
		},
		{
			name: "invalid_key",
			record: UpdateLogRecord{
				UpdateV1: []byte{0x01},
			},
			wantErr: ErrInvalidDocumentKey,
		},
		{
			name: "empty_payload",
			record: UpdateLogRecord{
				Key:      DocumentKey{DocumentID: "doc-2"},
				UpdateV1: nil,
			},
			wantErr: ErrInvalidUpdatePayload,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.record.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}

	record := &UpdateLogRecord{
		Key:      DocumentKey{Namespace: "tenant-a", DocumentID: "doc-9"},
		Offset:   11,
		UpdateV1: []byte{0x0a, 0x0b},
		Epoch:    9,
		StoredAt: time.Unix(220, 0).UTC(),
	}
	clone := record.Clone()
	if clone == nil {
		t.Fatal("Clone() = nil, want non-nil")
	}
	if !bytes.Equal(clone.UpdateV1, record.UpdateV1) {
		t.Fatalf("Clone().UpdateV1 = %v, want %v", clone.UpdateV1, record.UpdateV1)
	}
	if clone.Epoch != record.Epoch {
		t.Fatalf("Clone().Epoch = %d, want %d", clone.Epoch, record.Epoch)
	}
	clone.UpdateV1[0] = 0xff
	if record.UpdateV1[0] == 0xff {
		t.Fatal("Clone() compartilhou payload do log")
	}

	if got := (*UpdateLogRecord)(nil).Clone(); got != nil {
		t.Fatalf("(*UpdateLogRecord)(nil).Clone() = %#v, want nil", got)
	}
}

func TestLeaseStoreErrorSentinels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name:    "conflict",
			err:     fmt.Errorf("%w: shard %s token %q", ErrLeaseConflict, ShardID("shard-a"), "lease-token"),
			wantErr: ErrLeaseConflict,
		},
		{
			name:    "stale_epoch",
			err:     fmt.Errorf("%w: shard %s current=8 incoming=7", ErrLeaseStaleEpoch, ShardID("shard-a")),
			wantErr: ErrLeaseStaleEpoch,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !errors.Is(tt.err, tt.wantErr) {
				t.Fatalf("errors.Is(%v, %v) = false, want true", tt.err, tt.wantErr)
			}
		})
	}
}

func TestPlacementRecordValidateAndClone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		record  PlacementRecord
		wantErr error
	}{
		{
			name: "valid",
			record: PlacementRecord{
				Key:       DocumentKey{Namespace: "tenant", DocumentID: "doc-1"},
				ShardID:   ShardID("shard-a"),
				Version:   5,
				UpdatedAt: time.Unix(300, 0).UTC(),
			},
			wantErr: nil,
		},
		{
			name: "invalid_key",
			record: PlacementRecord{
				ShardID: ShardID("shard-a"),
			},
			wantErr: ErrInvalidDocumentKey,
		},
		{
			name: "invalid_shard",
			record: PlacementRecord{
				Key:     DocumentKey{DocumentID: "doc-2"},
				ShardID: ShardID(""),
			},
			wantErr: ErrInvalidShardID,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.record.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}

	record := &PlacementRecord{
		Key:       DocumentKey{Namespace: "tenant", DocumentID: "doc-3"},
		ShardID:   ShardID("shard-b"),
		Version:   9,
		UpdatedAt: time.Unix(301, 0).UTC(),
	}
	clone := record.Clone()
	if clone == nil {
		t.Fatal("Clone() = nil, want non-nil")
	}
	if *clone != *record {
		t.Fatalf("Clone() = %#v, want %#v", clone, record)
	}
	clone.Key.DocumentID = "other"
	if record.Key.DocumentID == clone.Key.DocumentID {
		t.Fatal("Clone() compartilhou chave do placement")
	}

	if got := (*PlacementRecord)(nil).Clone(); got != nil {
		t.Fatalf("(*PlacementRecord)(nil).Clone() = %#v, want nil", got)
	}
}

func TestLeaseRecordValidateAndClone(t *testing.T) {
	t.Parallel()

	baseTime := time.Unix(400, 0).UTC()

	tests := []struct {
		name    string
		record  LeaseRecord
		wantErr error
	}{
		{
			name: "valid",
			record: LeaseRecord{
				ShardID:    ShardID("shard-a"),
				Owner:      OwnerInfo{NodeID: NodeID("node-a"), Epoch: 1},
				Token:      "lease-token",
				AcquiredAt: baseTime,
				ExpiresAt:  baseTime.Add(30 * time.Second),
			},
			wantErr: nil,
		},
		{
			name: "invalid_shard",
			record: LeaseRecord{
				Owner:     OwnerInfo{NodeID: NodeID("node-a"), Epoch: 1},
				Token:     "lease-token",
				ExpiresAt: baseTime.Add(time.Second),
			},
			wantErr: ErrInvalidShardID,
		},
		{
			name: "invalid_owner",
			record: LeaseRecord{
				ShardID:   ShardID("shard-a"),
				Owner:     OwnerInfo{NodeID: NodeID("node-a")},
				Token:     "lease-token",
				ExpiresAt: baseTime.Add(time.Second),
			},
			wantErr: ErrInvalidOwnerInfo,
		},
		{
			name: "missing_token",
			record: LeaseRecord{
				ShardID:   ShardID("shard-a"),
				Owner:     OwnerInfo{NodeID: NodeID("node-a"), Epoch: 1},
				ExpiresAt: baseTime.Add(time.Second),
			},
			wantErr: ErrInvalidLeaseToken,
		},
		{
			name: "missing_expiry",
			record: LeaseRecord{
				ShardID: ShardID("shard-a"),
				Owner:   OwnerInfo{NodeID: NodeID("node-a"), Epoch: 1},
				Token:   "lease-token",
			},
			wantErr: ErrInvalidLeaseExpiry,
		},
		{
			name: "expiry_not_after_acquire",
			record: LeaseRecord{
				ShardID:    ShardID("shard-a"),
				Owner:      OwnerInfo{NodeID: NodeID("node-a"), Epoch: 1},
				Token:      "lease-token",
				AcquiredAt: baseTime,
				ExpiresAt:  baseTime,
			},
			wantErr: ErrInvalidLeaseExpiry,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.record.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}

	record := &LeaseRecord{
		ShardID:    ShardID("shard-b"),
		Owner:      OwnerInfo{NodeID: NodeID("node-b"), Epoch: 8},
		Token:      "lease-2",
		AcquiredAt: baseTime,
		ExpiresAt:  baseTime.Add(time.Minute),
	}
	clone := record.Clone()
	if clone == nil {
		t.Fatal("Clone() = nil, want non-nil")
	}
	if *clone != *record {
		t.Fatalf("Clone() = %#v, want %#v", clone, record)
	}
	clone.Token = "other"
	if record.Token == clone.Token {
		t.Fatal("Clone() compartilhou token da lease")
	}

	if got := (*LeaseRecord)(nil).Clone(); got != nil {
		t.Fatalf("(*LeaseRecord)(nil).Clone() = %#v, want nil", got)
	}
}

func TestAuthorityFenceValidateAndClone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fence   AuthorityFence
		wantErr error
	}{
		{
			name: "valid",
			fence: AuthorityFence{
				ShardID: ShardID("7"),
				Owner: OwnerInfo{
					NodeID: NodeID("node-a"),
					Epoch:  3,
				},
				Token: "lease-a",
			},
		},
		{
			name: "missing shard",
			fence: AuthorityFence{
				Owner: OwnerInfo{
					NodeID: NodeID("node-a"),
					Epoch:  3,
				},
				Token: "lease-a",
			},
			wantErr: ErrInvalidShardID,
		},
		{
			name: "invalid owner",
			fence: AuthorityFence{
				ShardID: ShardID("7"),
				Owner: OwnerInfo{
					NodeID: NodeID("node-a"),
				},
				Token: "lease-a",
			},
			wantErr: ErrInvalidOwnerInfo,
		},
		{
			name: "missing token",
			fence: AuthorityFence{
				ShardID: ShardID("7"),
				Owner: OwnerInfo{
					NodeID: NodeID("node-a"),
					Epoch:  3,
				},
			},
			wantErr: ErrInvalidLeaseToken,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.fence.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}

	fence := &AuthorityFence{
		ShardID: ShardID("11"),
		Owner: OwnerInfo{
			NodeID: NodeID("node-b"),
			Epoch:  8,
		},
		Token: "lease-b",
	}
	clone := fence.Clone()
	if clone == nil {
		t.Fatal("Clone() = nil, want non-nil")
	}
	if *clone != *fence {
		t.Fatalf("Clone() = %#v, want %#v", clone, fence)
	}
	clone.Token = "other"
	if fence.Token == clone.Token {
		t.Fatal("Clone() compartilhou token do fence")
	}

	if got := (*AuthorityFence)(nil).Clone(); got != nil {
		t.Fatalf("(*AuthorityFence)(nil).Clone() = %#v, want nil", got)
	}
}
