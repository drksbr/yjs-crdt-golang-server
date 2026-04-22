package ytypes

import (
	"errors"
	"math"
	"testing"
)

func TestDeleteSetNormalizesAndQueriesRanges(t *testing.T) {
	t.Parallel()

	ds := NewDeleteSet()
	if err := ds.Add(7, 10, 3); err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}
	if err := ds.AddID(ID{Client: 7, Clock: 13}, 2); err != nil {
		t.Fatalf("AddID() unexpected error: %v", err)
	}
	if err := ds.Add(3, 1, 1); err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}

	if !ds.Has(ID{Client: 7, Clock: 11}) || !ds.Has(ID{Client: 7, Clock: 14}) {
		t.Fatalf("Has() should recognize merged ranges")
	}
	if ds.Has(ID{Client: 7, Clock: 15}) {
		t.Fatalf("Has() = true outside merged range")
	}

	clients := ds.Clients()
	if len(clients) != 2 || clients[0] != 3 || clients[1] != 7 {
		t.Fatalf("Clients() = %v, want [3 7]", clients)
	}

	ranges := ds.Ranges(7)
	if len(ranges) != 1 || ranges[0].Clock != 10 || ranges[0].Length != 5 {
		t.Fatalf("Ranges(7) = %v, want [{10 5}]", ranges)
	}
}

func TestDeleteSetCloneAndMerge(t *testing.T) {
	t.Parallel()

	left := NewDeleteSet()
	right := NewDeleteSet()

	if err := left.Add(1, 1, 2); err != nil {
		t.Fatalf("left.Add() unexpected error: %v", err)
	}
	if err := right.Add(1, 3, 2); err != nil {
		t.Fatalf("right.Add() unexpected error: %v", err)
	}

	clone := left.Clone()
	if err := clone.Merge(right); err != nil {
		t.Fatalf("Merge() unexpected error: %v", err)
	}

	if len(left.Ranges(1)) != 1 || left.Ranges(1)[0].Length != 2 {
		t.Fatalf("Clone() should not mutate original set")
	}
	if len(clone.Ranges(1)) != 1 || clone.Ranges(1)[0].Length != 4 {
		t.Fatalf("merged clone = %v, want [{1 4}]", clone.Ranges(1))
	}
}

func TestDeleteSetRejectsInvalidRanges(t *testing.T) {
	t.Parallel()

	ds := NewDeleteSet()

	if err := ds.Add(1, 10, 0); !errors.Is(err, ErrInvalidLength) {
		t.Fatalf("Add(..., 0) error = %v, want propagated invalid length", err)
	}
	if err := ds.Add(1, math.MaxUint32, 2); !errors.Is(err, ErrStructOverflow) {
		t.Fatalf("Add(overflow) error = %v, want propagated overflow", err)
	}
}
