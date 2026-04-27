package yupdate

import (
	"errors"
	"testing"
)

func TestParsedContentSliceWindowInvalidOffsets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		startOffset uint32
		endTrim     uint32
	}{
		{name: "start_equal_length", startOffset: 3, endTrim: 0},
		{name: "end_trim_equal_length", startOffset: 0, endTrim: 3},
		{name: "sum_equal_length", startOffset: 1, endTrim: 2},
		{name: "start_greater_than_length", startOffset: 4, endTrim: 0},
		{name: "overflow_wraps_sum", startOffset: ^uint32(0), endTrim: 1},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			content := ParsedContent{
				Ref:       itemContentString,
				LengthVal: utf16Length("abc"),
				Countable: true,
				Raw:       appendVarStringV1(nil, "abc"),
				Text:      "abc",
			}

			_, err := content.SliceWindow(tc.startOffset, tc.endTrim)
			if err == nil {
				t.Fatalf("SliceWindow() error = nil, want %v", ErrInvalidSliceOffset)
			}
			if !errors.Is(err, ErrInvalidSliceOffset) {
				t.Fatalf("SliceWindow() error = %v, want %v", err, ErrInvalidSliceOffset)
			}
		})
	}
}
