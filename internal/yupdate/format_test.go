package yupdate

import (
	"errors"
	"testing"

	"yjs-go-bridge/internal/varint"
)

func TestDetectUpdateFormatWithReasonDetectsV1(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "hello"),
			},
		},
	)

	format, err := DetectUpdateFormatWithReason(update)
	if err != nil {
		t.Fatalf("DetectUpdateFormatWithReason() unexpected error: %v", err)
	}
	if format != UpdateFormatV1 {
		t.Fatalf("DetectUpdateFormatWithReason() = %s, want %s", format, UpdateFormatV1)
	}
}

func TestDetectUpdateFormatWithReasonRejectsBrokenVarint(t *testing.T) {
	t.Parallel()

	if _, err := DetectUpdateFormatWithReason([]byte{0x80}); !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("DetectUpdateFormatWithReason() error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}
}

func TestDetectUpdateFormatWithReasonRejectsTrailingBytesAfterV1(t *testing.T) {
	t.Parallel()

	update := append(buildUpdate(), 0x00)
	if _, err := DetectUpdateFormatWithReason(update); !errors.Is(err, ErrTrailingBytes) {
		t.Fatalf("DetectUpdateFormatWithReason() error = %v, want %v", err, ErrTrailingBytes)
	}
}

func TestDetectUpdateFormatWithReasonReturnsUnknownForEmptyPayload(t *testing.T) {
	t.Parallel()

	format := DetectUpdateFormat(nil)
	if format != UpdateFormatUnknown {
		t.Fatalf("DetectUpdateFormat(nil) = %s, want %s", format, UpdateFormatUnknown)
	}

	if _, err := DetectUpdateFormatWithReason([]byte{}); !errors.Is(err, ErrUnknownUpdateFormat) {
		t.Fatalf("DetectUpdateFormatWithReason() empty error = %v, want %v", err, ErrUnknownUpdateFormat)
	}
}

