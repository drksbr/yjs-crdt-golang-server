package yupdate

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func boundaryV1Update() []byte {
	return buildUpdate(
		clientBlock{
			client: 11,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "boundary"),
			},
		},
	)
}

func boundaryV2Update() []byte {
	return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
}

func TestFormatFromUpdatesContextBoundaryCases(t *testing.T) {
	t.Parallel()

	v1 := boundaryV1Update()
	v2 := boundaryV2Update()

	tests := []struct {
		name      string
		updates   [][]byte
		want      UpdateFormat
		wantErr   error
		hasResult bool
	}{
		{
			name:      "v2_detected",
			updates:   [][]byte{v2},
			want:      UpdateFormatV2,
			hasResult: true,
		},
		{
			name:    "mixed_formats_rejected",
			updates: [][]byte{v1, v2},
			wantErr: ErrMismatchedUpdateFormats,
		},
		{
			name:    "empty_payload_rejected",
			updates: [][]byte{v1, nil},
			wantErr: ErrUnknownUpdateFormat,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := FormatFromUpdatesContext(context.Background(), tt.updates...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("FormatFromUpdatesContext() error = %v, want %v", err, tt.wantErr)
				}
				if got != UpdateFormatUnknown {
					t.Fatalf("FormatFromUpdatesContext() = %s, want %s", got, UpdateFormatUnknown)
				}
				return
			}

			if err != nil {
				t.Fatalf("FormatFromUpdatesContext() unexpected error: %v", err)
			}
			if tt.hasResult && got != tt.want {
				t.Fatalf("FormatFromUpdatesContext() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMergeUpdatesContextBoundaryCases(t *testing.T) {
	t.Parallel()

	v1 := boundaryV1Update()
	v2 := boundaryV2Update()

	tests := []struct {
		name    string
		updates [][]byte
		want    []byte
		wantErr error
	}{
		{
			name:    "v2_rejected",
			updates: [][]byte{v2},
			wantErr: ErrUnsupportedUpdateFormatV2,
		},
		{
			name:    "mixed_formats_rejected",
			updates: [][]byte{v1, v2},
			wantErr: ErrMismatchedUpdateFormats,
		},
		{
			name:    "no_updates_returns_empty_v1",
			want:    encodeEmptyUpdateV1(),
			updates: nil,
		},
		{
			name:    "all_empty_payloads_return_empty_v1",
			want:    encodeEmptyUpdateV1(),
			updates: [][]byte{nil, []byte{}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := MergeUpdatesContext(context.Background(), tt.updates...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MergeUpdatesContext() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("MergeUpdatesContext() unexpected error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("MergeUpdatesContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStateVectorFromUpdatesContextBoundaryCases(t *testing.T) {
	t.Parallel()

	v1 := boundaryV1Update()
	v2 := boundaryV2Update()

	tests := []struct {
		name    string
		updates [][]byte
		wantErr error
		wantLen int
	}{
		{
			name:    "empty_payloads_are_noop",
			updates: [][]byte{nil, []byte{}},
			wantLen: 0,
		},
		{
			name:    "v2_detected_even_with_empty_prefix",
			updates: [][]byte{nil, []byte{}, v2},
			wantErr: ErrUnsupportedUpdateFormatV2,
		},
		{
			name:    "mixed_formats_rejected",
			updates: [][]byte{v1, v2},
			wantErr: ErrMismatchedUpdateFormats,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := StateVectorFromUpdatesContext(context.Background(), tt.updates...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("StateVectorFromUpdatesContext() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("StateVectorFromUpdatesContext() unexpected error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("StateVectorFromUpdatesContext() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestContentIDsFromUpdatesContextBoundaryCases(t *testing.T) {
	t.Parallel()

	v1 := boundaryV1Update()
	v2 := boundaryV2Update()

	tests := []struct {
		name    string
		updates [][]byte
		wantErr error
		wantLen int
	}{
		{
			name:    "empty_payloads_are_noop",
			updates: [][]byte{nil, []byte{}},
			wantLen: 0,
		},
		{
			name:    "v2_detected_even_with_empty_prefix",
			updates: [][]byte{nil, []byte{}, v2},
			wantErr: ErrUnsupportedUpdateFormatV2,
		},
		{
			name:    "mixed_formats_rejected",
			updates: [][]byte{v1, v2},
			wantErr: ErrMismatchedUpdateFormats,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ContentIDsFromUpdatesContext(context.Background(), tt.updates...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ContentIDsFromUpdatesContext() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ContentIDsFromUpdatesContext() unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("ContentIDsFromUpdatesContext() returned nil, want empty ContentIDs")
			}
			if !got.IsEmpty() {
				t.Fatalf("ContentIDsFromUpdatesContext() = %#v, want empty ContentIDs", got)
			}
			if tt.wantLen != 0 {
				t.Fatalf("ContentIDsFromUpdatesContext() wantLen = %d, want 0", tt.wantLen)
			}
		})
	}
}
