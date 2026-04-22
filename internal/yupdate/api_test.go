package yupdate

import (
	"bytes"
	"errors"
	"testing"

	"yjs-go-bridge/internal/varint"
)

func TestDecodeUpdateDispatchesV1(t *testing.T) {
	t.Parallel()

	update := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "hello"),
			},
		},
	)

	got, err := DecodeUpdate(update)
	if err != nil {
		t.Fatalf("DecodeUpdate() unexpected error: %v", err)
	}

	encoded, err := EncodeUpdate(got)
	if err != nil {
		t.Fatalf("EncodeUpdate() unexpected error: %v", err)
	}

	if !bytes.Equal(encoded, update) {
		t.Fatalf("DecodeUpdate() decode+encode roundtrip = %v, want %v", encoded, update)
	}
}

func TestMergeUpdatesDispatchesV1(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				gc(1),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  1,
			structs: []structEncoding{
				itemString(rootParent("doc"), "x"),
			},
		},
	)

	got, err := MergeUpdates(left, right)
	if err != nil {
		t.Fatalf("MergeUpdates() unexpected error: %v", err)
	}

	expected, err := MergeUpdatesV1(left, right)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	if !bytes.Equal(got, expected) {
		t.Fatalf("MergeUpdates() = %v, want %v", got, expected)
	}
}

func TestUpdateDispatchErrors(t *testing.T) {
	t.Parallel()

	if _, err := DecodeUpdate(nil); !errors.Is(err, ErrUnknownUpdateFormat) {
		t.Fatalf("DecodeUpdate(nil) error = %v, want %v", err, ErrUnknownUpdateFormat)
	}

	if _, err := DecodeUpdate([]byte{0x80}); !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("DecodeUpdate(v1 header broken) error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}

	if _, err := DecodeUpdate(append(buildUpdate(), 0x00)); !errors.Is(err, ErrTrailingBytes) {
		t.Fatalf("DecodeUpdate(v1 with trailing) error = %v, want %v", err, ErrTrailingBytes)
	}

	if _, err := DiffUpdate([]byte{0x00, 0x01}, nil); !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("DiffUpdate(v2-ambiguous candidate) error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}
}
