package yupdate

import (
	"errors"
	"testing"
)

func TestParsedContentSliceWindowUnsupportedRefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ref  byte
	}{
		{name: "binary", ref: itemContentBinary},
		{name: "embed", ref: itemContentEmbed},
		{name: "format", ref: itemContentFormat},
		{name: "type", ref: itemContentType},
		{name: "doc", ref: itemContentDoc},
		{name: "default-invalid", ref: 0xff},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			content := ParsedContent{
				Ref:       tc.ref,
				LengthVal: 2,
				Raw:       []byte{0x01, 0x02},
			}

			_, err := content.SliceWindow(0, 1)
			if err == nil {
				t.Fatalf("SliceWindow() error = nil, want %v", ErrUnsupportedContentSlice)
			}
			if !errors.Is(err, ErrUnsupportedContentSlice) {
				t.Fatalf("SliceWindow() error = %v, want %v", err, ErrUnsupportedContentSlice)
			}
		})
	}
}
