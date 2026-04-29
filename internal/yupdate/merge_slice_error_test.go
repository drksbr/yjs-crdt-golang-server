package yupdate

import (
	"errors"
	"fmt"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

type nonParsedContent struct {
	length uint32
}

func (c nonParsedContent) Length() uint32 {
	return c.length
}

func (c nonParsedContent) IsCountable() bool {
	return true
}

type unsupportedStruct struct {
	id     ytypes.ID
	length uint32
}

func (s unsupportedStruct) Kind() ytypes.Kind {
	return ytypes.KindSkip
}

func (s unsupportedStruct) ID() ytypes.ID {
	return s.id
}

func (s unsupportedStruct) Length() uint32 {
	return s.length
}

func (s unsupportedStruct) Deleted() bool {
	return false
}

func (s unsupportedStruct) EndClock() uint32 {
	return s.id.Clock + s.length
}

func (s unsupportedStruct) LastID() ytypes.ID {
	return ytypes.ID{Client: s.id.Client, Clock: s.EndClock() - 1}
}

func (s unsupportedStruct) ContainsClock(clock uint32) bool {
	return clock >= s.id.Clock && clock < s.EndClock()
}

func TestSliceStructWindowV1Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		current     func(*testing.T) ytypes.Struct
		startOffset uint32
		endTrim     uint32
		wantErr     error
		wantErrText string
	}{
		{
			name: "invalid_offsets",
			current: func(t *testing.T) ytypes.Struct {
				t.Helper()
				return mustGCStruct(t, 1, 10, 2)
			},
			startOffset: 1,
			endTrim:     1,
			wantErr:     ErrInvalidSliceOffset,
		},
		{
			name: "overflowing_offsets",
			current: func(t *testing.T) ytypes.Struct {
				t.Helper()
				return mustGCStruct(t, 2, 20, 2)
			},
			startOffset: ^uint32(0),
			endTrim:     2,
			wantErr:     ErrInvalidSliceOffset,
		},
		{
			name: "item_with_non_parsed_content",
			current: func(t *testing.T) ytypes.Struct {
				t.Helper()
				item, err := ytypes.NewItem(
					ytypes.ID{Client: 3, Clock: 30},
					nonParsedContent{length: 2},
					ytypes.ItemOptions{},
				)
				if err != nil {
					t.Fatalf("NewItem() unexpected error: %v", err)
				}
				return item
			},
			startOffset: 0,
			endTrim:     1,
			wantErr:     ErrUnsupportedContentSlice,
		},
		{
			name: "unsupported_struct_type",
			current: func(t *testing.T) ytypes.Struct {
				t.Helper()
				return unsupportedStruct{
					id:     ytypes.ID{Client: 4, Clock: 40},
					length: 2,
				}
			},
			startOffset: 0,
			endTrim:     1,
			wantErrText: fmt.Sprintf("yupdate: struct nao suportada para slice: %T", unsupportedStruct{}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := sliceStructWindowV1(tt.current(t), tt.startOffset, tt.endTrim)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("sliceStructWindowV1() error = %v, want %v", err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("sliceStructWindowV1() struct = %v, want nil", got)
				}
				return
			}

			if err == nil {
				t.Fatalf("sliceStructWindowV1() error = nil, want %q", tt.wantErrText)
			}
			if err.Error() != tt.wantErrText {
				t.Fatalf("sliceStructWindowV1() error = %q, want %q", err.Error(), tt.wantErrText)
			}
			if got != nil {
				t.Fatalf("sliceStructWindowV1() struct = %v, want nil", got)
			}
		})
	}
}
