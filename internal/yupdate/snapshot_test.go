package yupdate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yjs-go-bridge/internal/ytypes"
)

func TestSnapshotFromUpdateDispatchesV1(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 3),
			},
		},
		deleteRange{client: 1, clock: 9, length: 2},
	)

	got, err := SnapshotFromUpdate(update)
	if err != nil {
		t.Fatalf("SnapshotFromUpdate() unexpected error: %v", err)
	}

	if len(got.StateVector) != 2 || got.StateVector[1] != 2 || got.StateVector[2] != 3 {
		t.Fatalf("StateVector = %v, want map[1:2 2:3]", got.StateVector)
	}
	if !got.DeleteSet.Has(ytypes.ID{Client: 1, Clock: 9}) || !got.DeleteSet.Has(ytypes.ID{Client: 1, Clock: 10}) {
		t.Fatalf("DeleteSet = %#v, want client 1 clocks 9 and 10", got.DeleteSet)
	}
}

func TestSnapshotFromUpdatesContextAggregatesV1(t *testing.T) {
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

	got, err := SnapshotFromUpdatesContext(context.Background(), nil, left, []byte{}, right)
	if err != nil {
		t.Fatalf("SnapshotFromUpdatesContext() unexpected error: %v", err)
	}

	if len(got.StateVector) != 2 || got.StateVector[4] != 3 || got.StateVector[7] != 2 {
		t.Fatalf("StateVector = %v, want map[4:3 7:2]", got.StateVector)
	}
	if !got.DeleteSet.Has(ytypes.ID{Client: 4, Clock: 10}) || !got.DeleteSet.Has(ytypes.ID{Client: 7, Clock: 5}) || !got.DeleteSet.Has(ytypes.ID{Client: 7, Clock: 6}) {
		t.Fatalf("DeleteSet = %#v, want merged delete ranges", got.DeleteSet)
	}
}

func TestSnapshotFromUpdatesContextHandlesEmptyAndV2(t *testing.T) {
	t.Parallel()

	v2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	empty, err := SnapshotFromUpdatesContext(context.Background(), nil, []byte{})
	if err != nil {
		t.Fatalf("SnapshotFromUpdatesContext(empty) unexpected error: %v", err)
	}
	if !empty.IsEmpty() {
		t.Fatalf("SnapshotFromUpdatesContext(empty) = %#v, want empty snapshot", empty)
	}

	_, err = SnapshotFromUpdatesContext(context.Background(), nil, []byte{}, v2)
	if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
		t.Fatalf("SnapshotFromUpdatesContext(v2) error = %v, want %v", err, ErrUnsupportedUpdateFormatV2)
	}
	if !strings.Contains(err.Error(), "update[2]") {
		t.Fatalf("SnapshotFromUpdatesContext(v2) error = %v, want update index 2", err)
	}
}

func TestSnapshotClone(t *testing.T) {
	t.Parallel()

	original := &Snapshot{
		StateVector: map[uint32]uint32{1: 2},
		DeleteSet:   ytypes.NewDeleteSet(),
	}
	if err := original.DeleteSet.Add(1, 9, 1); err != nil {
		t.Fatalf("DeleteSet.Add() unexpected error: %v", err)
	}

	clone := original.Clone()
	clone.StateVector[1] = 99
	if err := clone.DeleteSet.Add(2, 3, 1); err != nil {
		t.Fatalf("clone.DeleteSet.Add() unexpected error: %v", err)
	}

	if original.StateVector[1] != 2 {
		t.Fatalf("original StateVector mutated: %v", original.StateVector)
	}
	if original.DeleteSet.Has(ytypes.ID{Client: 2, Clock: 3}) {
		t.Fatalf("original DeleteSet mutated: %#v", original.DeleteSet)
	}
}
