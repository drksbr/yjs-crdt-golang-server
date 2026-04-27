package yupdate

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
)

func TestEncodePersistedSnapshotV1(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 11,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
		deleteRange{client: 11, clock: 9, length: 1},
	)
	valid, err := PersistedSnapshotFromUpdate(update)
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdate() unexpected error: %v", err)
	}
	wantValid, err := ConvertUpdateToV1(update)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1() unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		snapshot *PersistedSnapshot
		want     []byte
		wantErr  error
	}{
		{
			name: "nil_snapshot_is_empty",
			want: encodeEmptyUpdateV1(),
		},
		{
			name:     "new_snapshot_is_empty",
			snapshot: NewPersistedSnapshot(),
			want:     encodeEmptyUpdateV1(),
		},
		{
			name:     "valid_snapshot_returns_canonical_update",
			snapshot: valid,
			want:     wantValid,
		},
		{
			name: "empty_update_with_non_empty_snapshot_is_rejected",
			snapshot: &PersistedSnapshot{
				UpdateV1: encodeEmptyUpdateV1(),
				Snapshot: &Snapshot{
					StateVector: map[uint32]uint32{1: 1},
					DeleteSet:   ytypes.NewDeleteSet(),
				},
			},
			wantErr: ErrInconsistentPersistedSnapshot,
		},
		{
			name: "missing_update_with_non_empty_snapshot_is_rejected",
			snapshot: &PersistedSnapshot{
				Snapshot: &Snapshot{
					StateVector: map[uint32]uint32{1: 1},
					DeleteSet:   ytypes.NewDeleteSet(),
				},
			},
			wantErr: ErrInconsistentPersistedSnapshot,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := EncodePersistedSnapshotV1(tt.snapshot)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("EncodePersistedSnapshotV1() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("EncodePersistedSnapshotV1() unexpected error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("EncodePersistedSnapshotV1() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodePersistedSnapshotV1(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 12,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
			},
		},
		deleteRange{client: 12, clock: 4, length: 2},
	)
	wantUpdate, err := ConvertUpdateToV1(update)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1() unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		payload []byte
		want    []byte
		wantErr error
	}{
		{
			name: "empty_payload_restores_empty_snapshot",
			want: encodeEmptyUpdateV1(),
		},
		{
			name:    "valid_v1_restores_snapshot",
			payload: update,
			want:    wantUpdate,
		},
		{
			name:    "v2_is_rejected",
			payload: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			wantErr: ErrUnsupportedUpdateFormatV2,
		},
		{
			name:    "malformed_payload_is_rejected",
			payload: []byte{0x80},
			wantErr: varint.ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodePersistedSnapshotV1(tt.payload)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("DecodePersistedSnapshotV1() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodePersistedSnapshotV1() unexpected error: %v", err)
			}
			if !bytes.Equal(got.UpdateV1, tt.want) {
				t.Fatalf("UpdateV1 = %v, want %v", got.UpdateV1, tt.want)
			}

			derived, err := SnapshotFromUpdateV1(got.UpdateV1)
			if err != nil {
				t.Fatalf("SnapshotFromUpdateV1() unexpected error: %v", err)
			}
			assertPersistedSnapshotMatchesSnapshot(t, got, derived)
		})
	}
}

func TestDecodePersistedSnapshotV1ContextRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := buildUpdate(
		clientBlock{
			client: 13,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)

	_, err := DecodePersistedSnapshotV1Context(ctx, update)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DecodePersistedSnapshotV1Context() error = %v, want context.Canceled", err)
	}
}
