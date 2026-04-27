package yupdate

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"yjs-go-bridge/internal/varint"
)

func TestConvertUpdatesToV1(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  2,
			structs: []structEncoding{
				gc(1),
			},
		},
		clientBlock{
			client: 7,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
			},
		},
	)
	mergedV1, err := MergeUpdatesV1(left, right)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	v2Update := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	malformedUpdate := []byte{0x80}

	tests := []struct {
		name      string
		updates   [][]byte
		want      []byte
		wantErr   error
		wantIndex string
	}{
		{
			name:    "multiple_v1_updates_are_canonicalized",
			updates: [][]byte{left, right},
			want:    mergedV1,
		},
		{
			name: "all_updates_empty_are_noop",
			updates: [][]byte{
				nil,
				[]byte{},
			},
			want: encodeEmptyUpdateV1(),
		},
		{
			name:    "no_updates_returns_empty_update",
			updates: nil,
			want:    encodeEmptyUpdateV1(),
		},
		{
			name:      "v2_after_empty_prefix_is_rejected_with_index",
			updates:   [][]byte{nil, []byte{}, v2Update},
			wantErr:   ErrUnsupportedUpdateFormatV2,
			wantIndex: "update[2]",
		},
		{
			name:      "malformed_payload_is_rejected_with_index",
			updates:   [][]byte{left, malformedUpdate},
			wantErr:   varint.ErrUnexpectedEOF,
			wantIndex: "update[1]",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertUpdatesToV1(tt.updates...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ConvertUpdatesToV1() error = %v, want %v", err, tt.wantErr)
				}
				if tt.wantIndex != "" && !strings.Contains(err.Error(), tt.wantIndex) {
					t.Fatalf("ConvertUpdatesToV1() error = %v, want %s", err, tt.wantIndex)
				}
				return
			}
			if err != nil {
				t.Fatalf("ConvertUpdatesToV1() unexpected error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("ConvertUpdatesToV1() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertUpdatesToV1Context(t *testing.T) {
	t.Parallel()

	left := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
	)
	right := buildUpdate(
		clientBlock{
			client: 4,
			clock:  2,
			structs: []structEncoding{
				gc(1),
			},
		},
		clientBlock{
			client: 7,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 2),
			},
		},
	)
	mergedV1, err := MergeUpdatesV1(left, right)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	v2Update := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	malformedUpdate := []byte{0x80}

	tests := []struct {
		name      string
		ctx       context.Context
		updates   [][]byte
		want      []byte
		wantErr   error
		wantIndex string
	}{
		{
			name:    "multiple_v1_updates_are_canonicalized",
			ctx:     context.Background(),
			updates: [][]byte{left, right},
			want:    mergedV1,
		},
		{
			name: "all_updates_empty_are_noop",
			ctx:  context.Background(),
			updates: [][]byte{
				nil,
				[]byte{},
			},
			want: encodeEmptyUpdateV1(),
		},
		{
			name:    "no_updates_returns_empty_update",
			ctx:     context.Background(),
			updates: nil,
			want:    encodeEmptyUpdateV1(),
		},
		{
			name:      "v2_after_empty_prefix_is_rejected_with_index",
			ctx:       context.Background(),
			updates:   [][]byte{nil, []byte{}, v2Update},
			wantErr:   ErrUnsupportedUpdateFormatV2,
			wantIndex: "update[2]",
		},
		{
			name:      "context_is_respected",
			ctx:       canceledContext(),
			updates:   [][]byte{left},
			wantErr:   context.Canceled,
			wantIndex: "",
		},
		{
			name:      "malformed_payload_is_rejected_with_index",
			ctx:       context.Background(),
			updates:   [][]byte{left, malformedUpdate},
			wantErr:   varint.ErrUnexpectedEOF,
			wantIndex: "update[1]",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ConvertUpdatesToV1Context(tt.ctx, tt.updates...)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ConvertUpdatesToV1Context() error = %v, want %v", err, tt.wantErr)
				}
				if tt.wantIndex != "" && !strings.Contains(err.Error(), tt.wantIndex) {
					t.Fatalf("ConvertUpdatesToV1Context() error = %v, want %s", err, tt.wantIndex)
				}
				return
			}
			if err != nil {
				t.Fatalf("ConvertUpdatesToV1Context() unexpected error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("ConvertUpdatesToV1Context() = %v, want %v", got, tt.want)
			}
		})
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
