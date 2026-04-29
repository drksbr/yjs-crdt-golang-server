package yupdate

import (
	"errors"
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestContentIDsFromUpdatesReturnsEmptyForAllEmptyInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		updates [][]byte
	}{
		{
			name:    "no_updates",
			updates: nil,
		},
		{
			name:    "only_nil",
			updates: [][]byte{nil, nil},
		},
		{
			name:    "nil_and_empty_payloads",
			updates: [][]byte{nil, []byte{}, nil, []byte{}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			contentIDs, err := ContentIDsFromUpdates(tt.updates...)
			if err != nil {
				t.Fatalf("ContentIDsFromUpdates() unexpected error: %v", err)
			}
			if contentIDs == nil {
				t.Fatal("ContentIDsFromUpdates() = nil, want empty ContentIDs")
			}
			if !contentIDs.IsEmpty() {
				t.Fatalf("ContentIDsFromUpdates() = %#v, want empty ContentIDs", contentIDs)
			}
		})
	}
}

func TestContentIDsFromUpdatesRejectsMixedFormatsAfterSkippingEmptyPayloads(t *testing.T) {
	t.Parallel()

	v1Update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "a"),
			},
		},
	)
	v2Update := mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")

	_, err := ContentIDsFromUpdates(nil, []byte{}, v1Update, nil, v2Update, []byte{})
	if !errors.Is(err, ErrMismatchedUpdateFormats) {
		t.Fatalf("ContentIDsFromUpdates() error = %v, want %v", err, ErrMismatchedUpdateFormats)
	}
	if errors.Is(err, ErrUnsupportedUpdateFormatV2) {
		t.Fatalf("ContentIDsFromUpdates() error = %v, want mixed-format rejection before V2 dispatch", err)
	}
}

func TestContentIDsFromUpdatesConvertsDetectedV2AfterSkippingEmptyPayloads(t *testing.T) {
	t.Parallel()

	v2Update := mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")
	v2AsV1, err := ConvertUpdateToV1(v2Update)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1() unexpected error: %v", err)
	}
	want, err := ContentIDsFromUpdates(v2AsV1)
	if err != nil {
		t.Fatalf("ContentIDsFromUpdates(v1) unexpected error: %v", err)
	}

	got, err := ContentIDsFromUpdates(nil, []byte{}, v2Update, nil)
	if err != nil {
		t.Fatalf("ContentIDsFromUpdates(v2) unexpected error: %v", err)
	}
	if !IsSubsetContentIDs(got, want) || !IsSubsetContentIDs(want, got) {
		t.Fatalf("ContentIDsFromUpdates(v2) = %#v, want %#v", got, want)
	}
}

func TestContentIDsFromUpdatesPreservesIndexedMalformedErrorAfterSkippingEmptyPayloads(t *testing.T) {
	t.Parallel()

	valid := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 1),
			},
		},
	)

	_, err := ContentIDsFromUpdates(nil, []byte{}, []byte{0x80}, valid)
	if err == nil {
		t.Fatal("ContentIDsFromUpdates() error = nil, want malformed update error")
	}
	if !strings.Contains(err.Error(), "update[2]") {
		t.Fatalf("ContentIDsFromUpdates() error = %v, want update index 2", err)
	}
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("ContentIDsFromUpdates() error = %v, want %v", err, varint.ErrUnexpectedEOF)
	}
}
