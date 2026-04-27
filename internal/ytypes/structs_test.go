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

func TestKindStringUnknown(t *testing.T) {
	t.Parallel()

	if got := Kind(255).String(); got != "unknown" {
		t.Fatalf("Kind(255).String() = %q, want %q", got, "unknown")
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

func TestEndClockOnInvalidStructsIsPanicFree(t *testing.T) {
	t.Parallel()

	testNoPanic := func(t *testing.T, fn func()) {
		t.Helper()
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("operation panicked unexpectedly: %v", recovered)
			}
		}()
		fn()
	}

	tests := []struct {
		name             string
		structure        baseStruct
		expectEnd        uint32
		expectCheckedErr error
		expectContains   bool
	}{
		{
			name:      "overflow",
			structure: baseStruct{id: ID{Client: 7, Clock: math.MaxUint32}, length: 1},
			// fallback for overflow keeps id.clock to avoid undefined behavior
			expectEnd:        math.MaxUint32,
			expectCheckedErr: ErrStructOverflow,
			expectContains:   false,
		},
		{
			name:             "zero_length",
			structure:        baseStruct{id: ID{Client: 7, Clock: 1}, length: 0},
			expectEnd:        1,
			expectCheckedErr: ErrInvalidLength,
			expectContains:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testNoPanic(t, func() {
				if got := tt.structure.EndClock(); got != tt.expectEnd {
					t.Fatalf("EndClock() = %d, want %d", got, tt.expectEnd)
				}
			})

			testNoPanic(t, func() {
				last := tt.structure.LastID()
				if last != tt.structure.id {
					t.Fatalf("LastID() = %#v, want %#v", last, tt.structure.id)
				}
			})

			testNoPanic(t, func() {
				if got := tt.structure.ContainsClock(tt.structure.id.Clock); got != tt.expectContains {
					t.Fatalf("ContainsClock() = %v, want %v", got, tt.expectContains)
				}
			})

			testNoPanic(t, func() {
				_, err := tt.structure.checkedEndClock()
				if !errors.Is(err, tt.expectCheckedErr) {
					t.Fatalf("checkedEndClock() error = %v, want %v", err, tt.expectCheckedErr)
				}
			})
		})
	}
}
