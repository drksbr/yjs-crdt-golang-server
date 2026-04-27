package yidset

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestSetAddMergesAdjacentAndOverlappingRanges(t *testing.T) {
	t.Parallel()

	set := New()
	mustAdd(t, set, 7, 10, 5)
	mustAdd(t, set, 7, 15, 3)
	mustAdd(t, set, 7, 12, 10)

	got := set.Ranges(7)
	if len(got) != 1 {
		t.Fatalf("len(Ranges) = %d, want 1", len(got))
	}
	if got[0].Clock != 10 || got[0].Length != 12 {
		t.Fatalf("Ranges()[0] = %+v, want {Clock:10 Length:12}", got[0])
	}
}

func TestSetAddKeepsDisjointRangesSorted(t *testing.T) {
	t.Parallel()

	set := New()
	mustAdd(t, set, 1, 20, 2)
	mustAdd(t, set, 1, 1, 2)
	mustAdd(t, set, 1, 10, 2)

	got := set.Ranges(1)
	if len(got) != 3 {
		t.Fatalf("len(Ranges) = %d, want 3", len(got))
	}

	want := []Range{
		{Clock: 1, Length: 2},
		{Clock: 10, Length: 2},
		{Clock: 20, Length: 2},
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Ranges()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSetHasUsesNormalizedRanges(t *testing.T) {
	t.Parallel()

	set := New()
	mustAdd(t, set, 9, 5, 3)
	mustAdd(t, set, 9, 8, 2)

	tests := []struct {
		clock uint32
		want  bool
	}{
		{clock: 4, want: false},
		{clock: 5, want: true},
		{clock: 9, want: true},
		{clock: 10, want: false},
	}

	for _, tt := range tests {
		if got := set.Has(9, tt.clock); got != tt.want {
			t.Fatalf("Has(9, %d) = %v, want %v", tt.clock, got, tt.want)
		}
	}
}

func TestSetMergeCombinesClientsDeterministically(t *testing.T) {
	t.Parallel()

	left := New()
	right := New()

	mustAdd(t, left, 2, 10, 2)
	mustAdd(t, right, 2, 12, 2)
	mustAdd(t, right, 1, 1, 1)

	if err := left.Merge(right); err != nil {
		t.Fatalf("Merge() unexpected error: %v", err)
	}

	clients := left.Clients()
	if len(clients) != 2 || clients[0] != 1 || clients[1] != 2 {
		t.Fatalf("Clients() = %v, want [1 2]", clients)
	}

	got := left.Ranges(2)
	if len(got) != 1 || got[0].Clock != 10 || got[0].Length != 4 {
		t.Fatalf("Ranges(2) = %v, want [{10 4}]", got)
	}
}

func TestMergeIdSetsNormalizesIndependentOfInputOrder(t *testing.T) {
	t.Parallel()

	a := New()
	b := New()
	c := New()

	mustAdd(t, a, 2, 4, 2)
	mustAdd(t, a, 1, 0, 1)
	mustAdd(t, b, 2, 1, 4)
	mustAdd(t, c, 3, 5, 1)

	merged := MergeIdSets(a, b, c)
	reversed := MergeIdSets(c, b, a)
	want := []string{"1:0+1", "2:1+5", "3:5+1"}

	if got := snapshot(merged); !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot(merged) = %v, want %v", got, want)
	}
	if got := snapshot(reversed); !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot(reversed) = %v, want %v", got, want)
	}
}

func TestInsertIntoIdSetClonesInputData(t *testing.T) {
	t.Parallel()

	src := New()
	mustAdd(t, src, 3, 2, 1)
	mustAdd(t, src, 3, 4, 1)

	dest := New()
	if err := InsertIntoIdSet(dest, src); err != nil {
		t.Fatalf("InsertIntoIdSet() unexpected error: %v", err)
	}

	mustAdd(t, src, 3, 3, 1)

	want := []Range{
		{Clock: 2, Length: 1},
		{Clock: 4, Length: 1},
	}
	if got := dest.Ranges(3); !reflect.DeepEqual(got, want) {
		t.Fatalf("Ranges(3) = %v, want %v", got, want)
	}
}

func TestSetAddRejectsInvalidRanges(t *testing.T) {
	t.Parallel()

	set := New()

	if err := set.Add(1, 10, 0); !errors.Is(err, ErrInvalidLength) {
		t.Fatalf("Add(..., 0) error = %v, want ErrInvalidLength", err)
	}

	if err := set.Add(1, ^uint32(0), 2); !errors.Is(err, ErrRangeOverflow) {
		t.Fatalf("Add(overflow) error = %v, want ErrRangeOverflow", err)
	}
}

func TestSetAcceptsLastClockValue(t *testing.T) {
	t.Parallel()

	set := New()
	mustAdd(t, set, 1, ^uint32(0), 1)

	if !set.Has(1, ^uint32(0)) {
		t.Fatalf("Has(1, maxUint32) = false, want true")
	}
}

func TestSetSliceReturnsPresentAndMissingSegments(t *testing.T) {
	t.Parallel()

	set := New()
	mustAdd(t, set, 4, 2, 2)
	mustAdd(t, set, 4, 6, 1)

	got := set.Slice(4, 1, 7)
	want := []SliceSegment{
		{Clock: 1, Length: 1, Exists: false},
		{Clock: 2, Length: 2, Exists: true},
		{Clock: 4, Length: 2, Exists: false},
		{Clock: 6, Length: 1, Exists: true},
		{Clock: 7, Length: 1, Exists: false},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Slice() = %#v, want %#v", got, want)
	}
}

func TestSubtractSetsPartialOverlap(t *testing.T) {
	t.Parallel()

	left := New()
	mustAdd(t, left, 1, 0, 10)

	remove := New()
	mustAdd(t, remove, 1, 3, 3)

	got := SubtractSets(left, remove)
	want := []string{"1:0+3", "1:6+4"}

	if snapshot := snapshot(got); !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("SubtractSets() = %v, want %v", snapshot, want)
	}
}

func TestSubtractSetsRemovesEntireClientRange(t *testing.T) {
	t.Parallel()

	left := New()
	mustAdd(t, left, 1, 1, 4)

	remove := New()
	mustAdd(t, remove, 1, 0, 10)

	got := SubtractSets(left, remove)
	if got == nil || !got.IsEmpty() {
		t.Fatalf("SubtractSets() = %+v, want empty set", snapshot(got))
	}
}

func TestSubtractSetsAffectsOnlyMatchingClients(t *testing.T) {
	t.Parallel()

	left := New()
	mustAdd(t, left, 1, 0, 3)
	mustAdd(t, left, 2, 10, 2)

	remove := New()
	mustAdd(t, remove, 1, 1, 1)
	mustAdd(t, remove, 3, 0, 1)

	got := SubtractSets(left, remove)
	want := []string{"1:0+1", "1:2+1", "2:10+2"}

	if snapshot := snapshot(got); !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("SubtractSets() = %v, want %v", snapshot, want)
	}
}

func TestSubtractSetsPreservesPostRemovalTailLength(t *testing.T) {
	t.Parallel()

	left := New()
	mustAdd(t, left, 1, 5, 4)

	remove := New()
	mustAdd(t, remove, 1, 6, 1)

	got := SubtractSets(left, remove)
	want := []string{"1:5+1", "1:7+2"}

	if snapshot := snapshot(got); !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("SubtractSets() = %v, want %v", snapshot, want)
	}
}

func TestSetSubtractIsIdempotent(t *testing.T) {
	t.Parallel()

	base := New()
	mustAdd(t, base, 1, 0, 10)
	mustAdd(t, base, 1, 20, 5)

	rem := New()
	mustAdd(t, rem, 1, 2, 4)

	got := base.Clone()
	if err := got.Subtract(rem); err != nil {
		t.Fatalf("Subtract() unexpected error: %v", err)
	}
	first := snapshot(got)

	if err := got.Subtract(rem); err != nil {
		t.Fatalf("Subtract() second call unexpected error: %v", err)
	}
	second := snapshot(got)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("idempotence violated: first=%v second=%v", first, second)
	}
}

func TestSetSubtractRejectsInvalidRanges(t *testing.T) {
	t.Parallel()

	left := &IdSet{clients: map[uint32][]Range{
		1: {{Clock: 1, Length: 0}},
	}}

	remove := New()
	mustAdd(t, remove, 1, 2, 1)

	if err := left.Subtract(remove); !errors.Is(err, ErrInvalidLength) {
		t.Fatalf("Subtract() error = %v, want ErrInvalidLength", err)
	}
}

func TestSetSubtractRejectsOverflowRanges(t *testing.T) {
	t.Parallel()

	left := &IdSet{clients: map[uint32][]Range{
		1: {{Clock: ^uint32(0), Length: 2}},
	}}

	remove := New()
	mustAdd(t, remove, 1, 0, 1)

	if err := left.Subtract(remove); !errors.Is(err, ErrRangeOverflow) {
		t.Fatalf("Subtract() error = %v, want ErrRangeOverflow", err)
	}
}

func TestIntersectSetsKeepsOnlyOverlaps(t *testing.T) {
	t.Parallel()

	left := New()
	right := New()

	mustAdd(t, left, 1, 0, 3)
	mustAdd(t, left, 1, 5, 3)
	mustAdd(t, right, 1, 2, 5)
	mustAdd(t, right, 2, 0, 1)

	got := IntersectSets(left, right)
	want := []string{"1:2+1", "1:5+2"}
	if snapshot := snapshot(got); !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("IntersectSets() = %v, want %v", snapshot, want)
	}
}

func mustAdd(t *testing.T, set *IdSet, client, clock, length uint32) {
	t.Helper()

	if err := set.Add(client, clock, length); err != nil {
		t.Fatalf("Add(%d, %d, %d) unexpected error: %v", client, clock, length, err)
	}
}

func snapshot(set *IdSet) []string {
	out := make([]string, 0)
	set.ForEach(func(client uint32, r Range) {
		out = append(out, fmt.Sprintf("%d:%d+%d", client, r.Clock, r.Length))
	})
	return out
}
