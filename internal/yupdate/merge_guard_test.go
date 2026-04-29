package yupdate

import (
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

func TestMergeOverlappingStructListsHandlesEmptySides(t *testing.T) {
	t.Parallel()

	left := []ytypes.Struct{mustGCStruct(t, 5, 2, 3)}
	right := []ytypes.Struct{mustGCStruct(t, 5, 9, 1)}

	tests := []struct {
		name  string
		left  []ytypes.Struct
		right []ytypes.Struct
		want  []ytypes.Struct
	}{
		{
			name:  "left_empty",
			left:  nil,
			right: right,
			want:  right,
		},
		{
			name:  "right_empty",
			left:  left,
			right: nil,
			want:  left,
		},
		{
			name:  "both_empty",
			left:  nil,
			right: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := mergeOverlappingStructLists(5, tt.left, tt.right)
			if err != nil {
				t.Fatalf("mergeOverlappingStructLists() unexpected error: %v", err)
			}
			assertStructSequence(t, got, tt.want)
		})
	}
}

func TestMergeDisjointStructListsHandlesEmptySides(t *testing.T) {
	t.Parallel()

	left := []ytypes.Struct{mustGCStruct(t, 7, 0, 2)}
	right := []ytypes.Struct{mustGCStruct(t, 7, 5, 1)}

	tests := []struct {
		name  string
		left  []ytypes.Struct
		right []ytypes.Struct
		want  []ytypes.Struct
	}{
		{
			name:  "left_empty",
			left:  nil,
			right: right,
			want:  right,
		},
		{
			name:  "right_empty",
			left:  left,
			right: nil,
			want:  left,
		},
		{
			name:  "both_empty",
			left:  nil,
			right: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := mergeDisjointStructLists(7, tt.left, tt.right, 0)
			if err != nil {
				t.Fatalf("mergeDisjointStructLists() unexpected error: %v", err)
			}
			assertStructSequence(t, got, tt.want)
		})
	}
}

func mustGCStruct(t *testing.T, client, clock, length uint32) ytypes.GC {
	t.Helper()

	gc, err := ytypes.NewGC(ytypes.ID{Client: client, Clock: clock}, length)
	if err != nil {
		t.Fatalf("NewGC() unexpected error: %v", err)
	}
	return gc
}

func assertStructSequence(t *testing.T, got, want []ytypes.Struct) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(structs) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Kind() != want[i].Kind() || got[i].ID() != want[i].ID() || got[i].Length() != want[i].Length() {
			t.Fatalf("struct[%d] = kind=%v id=%+v len=%d, want kind=%v id=%+v len=%d", i, got[i].Kind(), got[i].ID(), got[i].Length(), want[i].Kind(), want[i].ID(), want[i].Length())
		}
	}
}
