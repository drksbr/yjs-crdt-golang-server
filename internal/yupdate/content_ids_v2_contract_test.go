package yupdate

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestContentIDsFromUpdatesContractWithV2Detected(t *testing.T) {
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

	tests := []struct {
		name    string
		updates [][]byte
	}{
		{
			name:    "all_payloads_empty_then_v2",
			updates: [][]byte{nil, []byte{}, nil, v2Update},
		},
		{
			name:    "v2_with_trailing_empty_payload",
			updates: [][]byte{v2Update, []byte{}, nil},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ContentIDsFromUpdates(tt.updates...)
			if err != nil {
				t.Fatalf("ContentIDsFromUpdates(v2) unexpected error: %v", err)
			}
			if !IsSubsetContentIDs(got, want) || !IsSubsetContentIDs(want, got) {
				t.Fatalf("ContentIDsFromUpdates(v2) = %#v, want %#v", got, want)
			}
		})
	}
}

func TestContentIDsFromUpdatesContractMixedFormatsAreRejected(t *testing.T) {
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

	tests := []struct {
		name    string
		updates [][]byte
	}{
		{
			name:    "v1_v2_interleaving_with_empties",
			updates: [][]byte{nil, []byte{}, v1Update, []byte{}, v2Update},
		},
		{
			name:    "v2_then_v1",
			updates: [][]byte{v2Update, v1Update},
		},
		{
			name:    "v1_v2_v1",
			updates: [][]byte{v1Update, nil, v2Update, []byte{}, v1Update},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ContentIDsFromUpdates(tt.updates...)
			if !errors.Is(err, ErrMismatchedUpdateFormats) {
				t.Fatalf("ContentIDsFromUpdates() error = %v, want %v", err, ErrMismatchedUpdateFormats)
			}
			if errors.Is(err, ErrUnsupportedUpdateFormatV2) {
				t.Fatalf("ContentIDsFromUpdates() should fail with format mismatch, not v2 unsupported")
			}
		})
	}
}

func TestContentIDsFromUpdatesContractPropagatesIndexedErrors(t *testing.T) {
	t.Parallel()

	validUpdate := buildUpdate(
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ok"),
			},
		},
	)
	malformedUpdate := []byte{0x80}

	tests := []struct {
		name      string
		updates   [][]byte
		wantIndex int
	}{
		{
			name:      "first_payload_malformed",
			updates:   [][]byte{malformedUpdate, validUpdate},
			wantIndex: 0,
		},
		{
			name:      "malformed_after_empty_prefix_is_indexed",
			updates:   [][]byte{nil, []byte{}, malformedUpdate, validUpdate},
			wantIndex: 2,
		},
		{
			name:      "malformed_after_v2_payload_keeps_index",
			updates:   [][]byte{mustDecodeHex(t, "000002a50100000104060374686901020101000001010000"), nil, malformedUpdate},
			wantIndex: 2,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ContentIDsFromUpdates(tt.updates...)
			if err == nil {
				t.Fatal("ContentIDsFromUpdates() error = nil, want malformed update error")
			}
			wantMsg := fmt.Sprintf("update[%d]", tt.wantIndex)
			if !strings.Contains(err.Error(), wantMsg) {
				t.Fatalf("ContentIDsFromUpdates() error = %v, want update index marker %q", err, wantMsg)
			}
			if !errors.Is(err, varint.ErrUnexpectedEOF) {
				t.Fatalf("ContentIDsFromUpdates() error = %v, want %v", err, varint.ErrUnexpectedEOF)
			}
		})
	}
}
