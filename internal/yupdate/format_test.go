package yupdate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
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

func TestDetectUpdateFormatWithReasonDetectsMinimalV2Header(t *testing.T) {
	t.Parallel()

	update := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	format, err := DetectUpdateFormatWithReason(update)
	if err != nil {
		t.Fatalf("DetectUpdateFormatWithReason() unexpected error: %v", err)
	}
	if format != UpdateFormatV2 {
		t.Fatalf("DetectUpdateFormatWithReason() = %s, want %s", format, UpdateFormatV2)
	}

	got, err := FormatFromUpdate(update)
	if err != nil {
		t.Fatalf("FormatFromUpdate() unexpected error: %v", err)
	}
	if got != UpdateFormatV2 {
		t.Fatalf("FormatFromUpdate() = %s, want %s", got, UpdateFormatV2)
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

func TestDetectUpdatesFormatWithReasonValidatesCommonFormat(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "b"),
			},
		},
	)

	format, err := DetectUpdatesFormatWithReason(nil, []byte{}, left, right)
	if err != nil {
		t.Fatalf("DetectUpdatesFormatWithReason() unexpected error: %v", err)
	}
	if format != UpdateFormatV1 {
		t.Fatalf("DetectUpdatesFormatWithReason() = %s, want %s", format, UpdateFormatV1)
	}
}

func TestDetectUpdatesFormatWithReasonReturnsIndexOnMalformedUpdate(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)

	_, err := DetectUpdatesFormatWithReason(left, []byte{0x80})
	if err == nil {
		t.Fatalf("DetectUpdatesFormatWithReason() error = nil, want malformed update error")
	}
	if !strings.Contains(err.Error(), "update[1]") {
		t.Fatalf("DetectUpdatesFormatWithReason() error = %v, want update index 1", err)
	}
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("DetectUpdatesFormatWithReason() error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}
}

func TestDetectUpdatesFormatWithReasonRejectsMixedV1AndV2(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)
	right := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	_, err := DetectUpdatesFormatWithReason(left, right)
	if !errors.Is(err, ErrMismatchedUpdateFormats) {
		t.Fatalf("DetectUpdatesFormatWithReason() error = %v, want %v", err, ErrMismatchedUpdateFormats)
	}
}

func TestDetectUpdatesFormatWithReasonRejectsAllEmptyPayloads(t *testing.T) {
	t.Parallel()

	_, err := DetectUpdatesFormatWithReason(nil, []byte{}, []byte{}, nil)
	if !errors.Is(err, ErrUnknownUpdateFormat) {
		t.Fatalf("DetectUpdatesFormatWithReason() all-empty error = %v, want %v", err, ErrUnknownUpdateFormat)
	}
}

func TestDetectUpdatesFormatWithReasonContextRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ok"),
			},
		},
	)

	_, err := DetectUpdatesFormatWithReasonContext(ctx, update)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DetectUpdatesFormatWithReasonContext() error = %v, want context.Canceled", err)
	}
}

func TestDetectUpdatesFormatWithReasonReportsMalformedIndexWithEmptyPrefix(t *testing.T) {
	t.Parallel()

	_, err := DetectUpdatesFormatWithReason(nil, []byte{}, []byte{0x80}, buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ok"),
			},
		},
	))
	if err == nil {
		t.Fatal("DetectUpdatesFormatWithReason() error = nil, want malformed update error")
	}
	if !strings.Contains(err.Error(), "update[2]") {
		t.Fatalf("DetectUpdatesFormatWithReason() error = %v, want update index 2", err)
	}
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("DetectUpdatesFormatWithReason() error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}
}

func TestValidateUpdatesFormatWrappersMatchDetectUpdatesFormat(t *testing.T) {
	t.Parallel()

	v1 := buildUpdate(
		clientBlock{
			client: 11,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)

	detected, err := DetectUpdatesFormatWithReason(nil, []byte{}, v1)
	if err != nil {
		t.Fatalf("DetectUpdatesFormatWithReason() unexpected error: %v", err)
	}

	validated, err := ValidateUpdatesFormatWithReason(nil, []byte{}, v1)
	if err != nil {
		t.Fatalf("ValidateUpdatesFormatWithReason() unexpected error: %v", err)
	}
	if validated != detected {
		t.Fatalf("ValidateUpdatesFormatWithReason() = %s, want %s", validated, detected)
	}
}

func TestValidateUpdateFormatWithReasonHandlesUnknownPayload(t *testing.T) {
	t.Parallel()

	_, err := ValidateUpdateFormatWithReason(nil)
	if !errors.Is(err, ErrUnknownUpdateFormat) {
		t.Fatalf("ValidateUpdateFormatWithReason() nil error = %v, want %v", err, ErrUnknownUpdateFormat)
	}
}

func TestDecodeUpdateReturnsUnsupportedForDetectedV2(t *testing.T) {
	t.Parallel()

	update := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	_, err := DecodeUpdate(update)
	if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
		t.Fatalf("DecodeUpdate() error = %v, want %v", err, ErrUnsupportedUpdateFormatV2)
	}
}

func TestEncodeStateVectorFromUpdatesAggregatesAndSkipsEmptyPayloads(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
				gc(1),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "x"),
			},
		},
	)

	leftSVData, err := EncodeStateVectorFromUpdateV1(left)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdateV1(left) unexpected error: %v", err)
	}
	rightSVData, err := EncodeStateVectorFromUpdateV1(right)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdateV1(right) unexpected error: %v", err)
	}
	leftSV, err := DecodeStateVectorV1(leftSVData)
	if err != nil {
		t.Fatalf("DecodeStateVectorV1(leftSVData) unexpected error: %v", err)
	}
	rightSV, err := DecodeStateVectorV1(rightSVData)
	if err != nil {
		t.Fatalf("DecodeStateVectorV1(rightSVData) unexpected error: %v", err)
	}

	expected := map[uint32]uint32{}
	for client, clock := range leftSV {
		expected[client] = clock
	}
	for client, clock := range rightSV {
		if prev, ok := expected[client]; !ok || clock > prev {
			expected[client] = clock
		}
	}

	gotState, err := StateVectorFromUpdates(nil, []byte{}, left, right)
	if err != nil {
		t.Fatalf("StateVectorFromUpdates() unexpected error: %v", err)
	}
	if len(gotState) != len(expected) {
		t.Fatalf("StateVectorFromUpdates() len map = %d, want %d", len(gotState), len(expected))
	}
	for client, expectedClock := range expected {
		if got := gotState[client]; got != expectedClock {
			t.Fatalf("StateVectorFromUpdates()[%d] = %d, want %d", client, got, expectedClock)
		}
	}

	gotEncoded, err := EncodeStateVectorFromUpdates(nil, []byte{}, left, right)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdates() unexpected error: %v", err)
	}
	gotDecoded, err := DecodeStateVector(gotEncoded)
	if err != nil {
		t.Fatalf("DecodeStateVector() unexpected error: %v", err)
	}
	if len(gotDecoded) != len(expected) {
		t.Fatalf("EncodeStateVectorFromUpdates() decoded len = %d, want %d", len(gotDecoded), len(expected))
	}
	for client, expectedClock := range expected {
		if got := gotDecoded[client]; got != expectedClock {
			t.Fatalf("EncodeStateVectorFromUpdates() decoded[%d] = %d, want %d", client, got, expectedClock)
		}
	}
}
