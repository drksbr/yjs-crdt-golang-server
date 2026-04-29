package yupdate

import (
	"errors"
	"math"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

type varyingLengthStruct struct {
	id      ytypes.ID
	lengths []uint32
	calls   int
}

func (s *varyingLengthStruct) Kind() ytypes.Kind {
	return ytypes.KindGC
}

func (s *varyingLengthStruct) ID() ytypes.ID {
	return s.id
}

func (s *varyingLengthStruct) Length() uint32 {
	if s.calls < len(s.lengths) {
		length := s.lengths[s.calls]
		s.calls++
		return length
	}
	return s.lengths[len(s.lengths)-1]
}

func (s *varyingLengthStruct) Deleted() bool {
	return false
}

func (s *varyingLengthStruct) EndClock() uint32 {
	return s.id.Clock + s.lengths[len(s.lengths)-1]
}

func (s *varyingLengthStruct) LastID() ytypes.ID {
	return ytypes.ID{Client: s.id.Client, Clock: s.EndClock() - 1}
}

func (s *varyingLengthStruct) ContainsClock(clock uint32) bool {
	return clock >= s.id.Clock && clock < s.EndClock()
}

func TestSliceStructWindowV1RejectsOverflowingOffsets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		current     func(*testing.T) ytypes.Struct
		startOffset uint32
		endTrim     uint32
		wantErr     error
	}{
		{
			name: "trim_exceeds_length",
			current: func(t *testing.T) ytypes.Struct {
				t.Helper()
				skip, err := ytypes.NewSkip(ytypes.ID{Client: 1, Clock: 1}, 2)
				if err != nil {
					t.Fatalf("NewSkip() unexpected error: %v", err)
				}
				return skip
			},
			startOffset: 0,
			endTrim:     math.MaxUint32,
			wantErr:     ErrInvalidSliceOffset,
		},
		{
			name: "shrinking_length_underflows_new_length",
			current: func(t *testing.T) ytypes.Struct {
				t.Helper()
				return &varyingLengthStruct{
					id:      ytypes.ID{Client: 2, Clock: math.MaxUint32},
					lengths: []uint32{3, 1},
				}
			},
			startOffset: 1,
			endTrim:     1,
			wantErr:     ytypes.ErrStructOverflow,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := sliceStructWindowV1(tt.current(t), tt.startOffset, tt.endTrim)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("sliceStructWindowV1() error = %v, want %v", err, tt.wantErr)
			}
			if got != nil {
				t.Fatalf("sliceStructWindowV1() struct = %v, want nil", got)
			}
		})
	}
}
