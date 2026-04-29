package yupdate

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestPersistedSnapshotFromUpdate(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
		deleteRange{client: 1, clock: 9, length: 2},
	)

	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{
			name:  "single_v1_update",
			input: update,
		},
		{
			name:  "empty_v1_update",
			input: encodeEmptyUpdateV1(),
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

func TestPersistedSnapshotFromUpdatesContext(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
		deleteRange{client: 4, clock: 10, length: 1},
	)
	right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  2,
			structs: []structEncoding{
				gc(1),
			},
		},
		clientBlock{
			client: 7,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
			},
		},
		deleteRange{client: 7, clock: 5, length: 2},
	)

	merged, err := MergeUpdatesV1(left, right)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	v2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	tests := []struct {
		name       string
		updates    [][]byte
		wantUpdate []byte
		wantErr    error
		wantIndex  string
	}{
		{
			name:       "multiple_v1_updates_merge_canonical",
			updates:    [][]byte{left, []byte{}, right},
			wantUpdate: merged,
		},
		{
			name:       "empty_payloads_are_noop",
			updates:    [][]byte{nil, []byte{}, nil},
			wantUpdate: encodeEmptyUpdateV1(),
		},
		{
			name:      "v2_rejected_with_original_index",
			updates:   [][]byte{nil, []byte{}, v2},
			wantErr:   ErrUnsupportedUpdateFormatV2,
			wantIndex: "update[2]",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := PersistedSnapshotFromUpdatesContext(context.Background(), tt.updates...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("PersistedSnapshotFromUpdatesContext() error = %v, want %v", err, tt.wantErr)
				}
				if tt.wantIndex != "" && !strings.Contains(err.Error(), tt.wantIndex) {
					t.Fatalf("PersistedSnapshotFromUpdatesContext() error = %v, want %s", err, tt.wantIndex)
				}
				return
			}

			if err != nil {
				t.Fatalf("PersistedSnapshotFromUpdatesContext() unexpected error: %v", err)
			}
			if !bytes.Equal(got.UpdateV1, tt.wantUpdate) {
				t.Fatalf("UpdateV1 = %v, want %v", got.UpdateV1, tt.wantUpdate)
			}

			derived, err := SnapshotFromUpdateV1(got.UpdateV1)
			if err != nil {
				t.Fatalf("SnapshotFromUpdateV1() unexpected error: %v", err)
			}
			assertPersistedSnapshotMatchesSnapshot(t, got, derived)
		})
	}
}

func TestPersistedSnapshotClone(t *testing.T) {
	t.Parallel()

	original := &PersistedSnapshot{
		UpdateV1: []byte{1, 2, 3},
		Snapshot: &Snapshot{
			StateVector: map[uint32]uint32{1: 2},
			DeleteSet:   ytypes.NewDeleteSet(),
		},
	}
	if err := original.Snapshot.DeleteSet.Add(1, 9, 1); err != nil {
		t.Fatalf("DeleteSet.Add() unexpected error: %v", err)
	}

	clone := original.Clone()
	clone.UpdateV1[0] = 9
	clone.Snapshot.StateVector[1] = 99
	if err := clone.Snapshot.DeleteSet.Add(2, 3, 1); err != nil {
		t.Fatalf("clone.DeleteSet.Add() unexpected error: %v", err)
	}

	if original.UpdateV1[0] != 1 {
		t.Fatalf("original UpdateV1 mutated: %v", original.UpdateV1)
	}
	if original.Snapshot.StateVector[1] != 2 {
		t.Fatalf("original StateVector mutated: %v", original.Snapshot.StateVector)
	}
	if original.Snapshot.DeleteSet.Has(ytypes.ID{Client: 2, Clock: 3}) {
		t.Fatalf("original DeleteSet mutated: %#v", original.Snapshot.DeleteSet)
	}
}

func TestPersistedSnapshotNilClone(t *testing.T) {
	t.Parallel()

	got := (*PersistedSnapshot)(nil).Clone()
	if got == nil {
		t.Fatalf("(*PersistedSnapshot)(nil).Clone() = nil, want non-nil snapshot")
	}
	if !got.IsEmpty() {
		t.Fatalf("(*PersistedSnapshot)(nil).Clone().IsEmpty() = false, want true")
	}
	if !bytes.Equal(got.UpdateV1, encodeEmptyUpdateV1()) {
		t.Fatalf("nil clone UpdateV1 = %v, want %v", got.UpdateV1, encodeEmptyUpdateV1())
	}
}

func assertPersistedSnapshotMatchesSnapshot(t *testing.T, got *PersistedSnapshot, want *Snapshot) {
	t.Helper()

	if len(got.Snapshot.StateVector) != len(want.StateVector) {
		t.Fatalf("StateVector len = %d, want %d", len(got.Snapshot.StateVector), len(want.StateVector))
	}
	for client, wantClock := range want.StateVector {
		if got.Snapshot.StateVector[client] != wantClock {
			t.Fatalf("StateVector[%d] = %d, want %d", client, got.Snapshot.StateVector[client], wantClock)
		}
	}

	gotClients := got.Snapshot.DeleteSet.Clients()
	wantClients := want.DeleteSet.Clients()
	if len(gotClients) != len(wantClients) {
		t.Fatalf("DeleteSet clients = %v, want %v", gotClients, wantClients)
	}
	for _, client := range wantClients {
		wantRanges := want.DeleteSet.Ranges(client)
		gotRanges := got.Snapshot.DeleteSet.Ranges(client)
		if len(gotRanges) != len(wantRanges) {
			t.Fatalf("DeleteSet ranges for client %d = %v, want %v", client, gotRanges, wantRanges)
		}
		for i, wantRange := range wantRanges {
			if gotRanges[i] != wantRange {
				t.Fatalf("DeleteSet range[%d] for client %d = %v, want %v", i, client, gotRanges[i], wantRange)
			}
		}
	}
}
