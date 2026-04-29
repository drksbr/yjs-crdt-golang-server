package yupdate

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestStateVectorAggregateAPIs(t *testing.T) {
	t.Parallel()

	v1 := buildUpdate(
		clientBlock{
			client: 7,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
	)
	v2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	tests := []struct {
		name    string
		updates [][]byte
		run     func(*testing.T, ...[]byte) error
		check   func(*testing.T, error)
	}{
		{
			name:    "all_empty_payloads_are_noop_for_state_vector",
			updates: [][]byte{nil, []byte{}, nil},
			run: func(t *testing.T, updates ...[]byte) error {
				got, err := StateVectorFromUpdates(updates...)
				if err != nil {
					return err
				}
				if got == nil {
					t.Fatal("StateVectorFromUpdates() = nil map, want empty map")
				}
				if len(got) != 0 {
					t.Fatalf("StateVectorFromUpdates() len = %d, want 0", len(got))
				}
				return nil
			},
			check: func(t *testing.T, err error) {
				if err != nil {
					t.Fatalf("StateVectorFromUpdates() unexpected error: %v", err)
				}
			},
		},
		{
			name:    "all_empty_payloads_are_noop_for_encoded_state_vector",
			updates: [][]byte{nil, []byte{}, nil},
			run: func(t *testing.T, updates ...[]byte) error {
				got, err := EncodeStateVectorFromUpdates(updates...)
				if err != nil {
					return err
				}

				want := varint.Append(nil, 0)
				if !bytes.Equal(got, want) {
					t.Fatalf("EncodeStateVectorFromUpdates() = %v, want %v", got, want)
				}

				decoded, err := DecodeStateVector(got)
				if err != nil {
					t.Fatalf("DecodeStateVector() unexpected error: %v", err)
				}
				if len(decoded) != 0 {
					t.Fatalf("DecodeStateVector() len = %d, want 0", len(decoded))
				}
				return nil
			},
			check: func(t *testing.T, err error) {
				if err != nil {
					t.Fatalf("EncodeStateVectorFromUpdates() unexpected error: %v", err)
				}
			},
		},
		{
			name:    "mixed_v1_and_v2_are_rejected_before_extraction_state_vector",
			updates: [][]byte{nil, v1, []byte{}, v2},
			run: func(_ *testing.T, updates ...[]byte) error {
				_, err := StateVectorFromUpdates(updates...)
				return err
			},
			check: func(t *testing.T, err error) {
				if !errors.Is(err, ErrMismatchedUpdateFormats) {
					t.Fatalf("StateVectorFromUpdates() error = %v, want %v", err, ErrMismatchedUpdateFormats)
				}
			},
		},
		{
			name:    "mixed_v1_and_v2_are_rejected_before_extraction_encoded_state_vector",
			updates: [][]byte{nil, v1, []byte{}, v2},
			run: func(_ *testing.T, updates ...[]byte) error {
				_, err := EncodeStateVectorFromUpdates(updates...)
				return err
			},
			check: func(t *testing.T, err error) {
				if !errors.Is(err, ErrMismatchedUpdateFormats) {
					t.Fatalf("EncodeStateVectorFromUpdates() error = %v, want %v", err, ErrMismatchedUpdateFormats)
				}
			},
		},
		{
			name:    "detected_v2_is_rejected_for_state_vector",
			updates: [][]byte{nil, []byte{}, v2},
			run: func(_ *testing.T, updates ...[]byte) error {
				_, err := StateVectorFromUpdates(updates...)
				return err
			},
			check: func(t *testing.T, err error) {
				if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
					t.Fatalf("StateVectorFromUpdates() error = %v, want %v", err, ErrUnsupportedUpdateFormatV2)
				}
				if !strings.Contains(err.Error(), "update[2]") {
					t.Fatalf("StateVectorFromUpdates() error = %v, want update index 2", err)
				}
			},
		},
		{
			name:    "detected_v2_is_rejected_for_encoded_state_vector",
			updates: [][]byte{nil, []byte{}, v2},
			run: func(_ *testing.T, updates ...[]byte) error {
				_, err := EncodeStateVectorFromUpdates(updates...)
				return err
			},
			check: func(t *testing.T, err error) {
				if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
					t.Fatalf("EncodeStateVectorFromUpdates() error = %v, want %v", err, ErrUnsupportedUpdateFormatV2)
				}
				if !strings.Contains(err.Error(), "update[2]") {
					t.Fatalf("EncodeStateVectorFromUpdates() error = %v, want update index 2", err)
				}
			},
		},
		{
			name:    "malformed_payload_keeps_index_for_state_vector",
			updates: [][]byte{nil, []byte{}, []byte{0x80}, v1},
			run: func(_ *testing.T, updates ...[]byte) error {
				_, err := StateVectorFromUpdates(updates...)
				return err
			},
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("StateVectorFromUpdates() error = nil, want malformed update error")
				}
				if !strings.Contains(err.Error(), "update[2]") {
					t.Fatalf("StateVectorFromUpdates() error = %v, want update index 2", err)
				}
				if !errors.Is(err, varint.ErrUnexpectedEOF) {
					t.Fatalf("StateVectorFromUpdates() error = %v, want %v", err, varint.ErrUnexpectedEOF)
				}
			},
		},
		{
			name:    "malformed_payload_keeps_index_for_encoded_state_vector",
			updates: [][]byte{nil, []byte{}, []byte{0x80}, v1},
			run: func(_ *testing.T, updates ...[]byte) error {
				_, err := EncodeStateVectorFromUpdates(updates...)
				return err
			},
			check: func(t *testing.T, err error) {
				if err == nil {
					t.Fatal("EncodeStateVectorFromUpdates() error = nil, want malformed update error")
				}
				if !strings.Contains(err.Error(), "update[2]") {
					t.Fatalf("EncodeStateVectorFromUpdates() error = %v, want update index 2", err)
				}
				if !errors.Is(err, varint.ErrUnexpectedEOF) {
					t.Fatalf("EncodeStateVectorFromUpdates() error = %v, want %v", err, varint.ErrUnexpectedEOF)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.check(t, tt.run(t, tt.updates...))
		})
	}
}
