package ytypes

import (
	"errors"
	"math"
	"testing"
)

func TestGCAndSkipExposeSharedStructSemantics(t *testing.T) {
	t.Parallel()

	gc, err := NewGC(ID{Client: 9, Clock: 10}, 3)
	if err != nil {
		t.Fatalf("NewGC() unexpected error: %v", err)
	}

	skip, err := NewSkip(ID{Client: 3, Clock: 20}, 2)
	if err != nil {
		t.Fatalf("NewSkip() unexpected error: %v", err)
	}

	if gc.Kind() != KindGC || gc.Kind().String() != "gc" {
		t.Fatalf("gc.Kind() = %v, want KindGC", gc.Kind())
	}
	if !gc.Deleted() {
		t.Fatalf("gc.Deleted() = false, want true")
	}
	if gc.EndClock() != 13 {
		t.Fatalf("gc.EndClock() = %d, want 13", gc.EndClock())
	}
	if gc.LastID() != (ID{Client: 9, Clock: 12}) {
		t.Fatalf("gc.LastID() = %+v, want {Client:9 Clock:12}", gc.LastID())
	}
	if !gc.ContainsClock(10) || !gc.ContainsClock(12) || gc.ContainsClock(13) {
		t.Fatalf("gc.ContainsClock() returned inconsistent results")
	}

	if skip.Kind() != KindSkip || skip.Kind().String() != "skip" {
		t.Fatalf("skip.Kind() = %v, want KindSkip", skip.Kind())
	}
	if skip.Deleted() {
		t.Fatalf("skip.Deleted() = true, want false")
	}
	if skip.EndClock() != 22 {
		t.Fatalf("skip.EndClock() = %d, want 22", skip.EndClock())
	}
}

func TestStructConstructorsRejectInvalidRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		build   func() error
		wantErr error
	}{
		{
			name: "zero_length_gc",
			build: func() error {
				_, err := NewGC(ID{Client: 1, Clock: 0}, 0)
				return err
			},
			wantErr: ErrInvalidLength,
		},
		{
			name: "overflow_skip",
			build: func() error {
				_, err := NewSkip(ID{Client: 1, Clock: math.MaxUint32}, 1)
				return err
			},
			wantErr: ErrStructOverflow,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.build(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("constructor error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
