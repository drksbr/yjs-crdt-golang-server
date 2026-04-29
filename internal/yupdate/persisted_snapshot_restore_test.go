package yupdate

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestPersistedSnapshotFromUpdateV1Restore(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 9,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "x"),
			},
		},
		deleteRange{
			client: 9,
			clock:  1,
			length: 1,
		},
	)

	malformed := []byte{0x80}
	v2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{
			name:  "empty_v1_update",
			input: encodeEmptyUpdateV1(),
		},
		{
			name:  "valid_v1_update",
			input: update,
		},
		{
			name:    "malformed_payload",
			input:   malformed,
			wantErr: varint.ErrUnexpectedEOF,
		},
		{
			name:    "v2_rejected_through_dispatch",
			input:   v2,
			wantErr: ErrUnsupportedUpdateFormatV2,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := PersistedSnapshotFromUpdate(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("PersistedSnapshotFromUpdate() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("PersistedSnapshotFromUpdate() unexpected error: %v", err)
			}

			wantUpdate, err := ConvertUpdateToV1(tt.input)
			if err != nil {
				t.Fatalf("ConvertUpdateToV1() unexpected error: %v", err)
			}
			if !bytes.Equal(got.UpdateV1, wantUpdate) {
				t.Fatalf("UpdateV1 = %v, want %v", got.UpdateV1, wantUpdate)
			}

			derived, err := SnapshotFromUpdateV1(got.UpdateV1)
			if err != nil {
				t.Fatalf("SnapshotFromUpdateV1() unexpected error: %v", err)
			}
			assertPersistedSnapshotMatchesSnapshot(t, got, derived)
		})
	}
}

func TestPersistedSnapshotFromUpdatesContextIsCanceled(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 10,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ctx"),
			},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := PersistedSnapshotFromUpdatesContext(ctx, update)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PersistedSnapshotFromUpdatesContext(context canceled) error = %v, want %v", err, context.Canceled)
	}
}
