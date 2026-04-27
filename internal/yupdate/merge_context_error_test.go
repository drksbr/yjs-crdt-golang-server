package yupdate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"yjs-go-bridge/internal/varint"
)

func TestMergeUpdatesV1ContextRejectsMalformedPayloads(t *testing.T) {
	t.Parallel()

	validUpdate := buildUpdate(
		clientBlock{
			client: 11,
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
		wantIndex string
	}{
		{
			name:      "malformed_first_payload",
			updates:   [][]byte{malformedUpdate},
			wantIndex: "update[0]",
		},
		{
			name:      "malformed_after_valid_payload",
			updates:   [][]byte{validUpdate, malformedUpdate},
			wantIndex: "update[1]",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := MergeUpdatesV1Context(context.Background(), tt.updates...)
			if err == nil {
				t.Fatal("MergeUpdatesV1Context() error = nil, want malformed update error")
			}
			if !errors.Is(err, varint.ErrUnexpectedEOF) {
				t.Fatalf("MergeUpdatesV1Context() error = %v, want %v", err, varint.ErrUnexpectedEOF)
			}
			if !strings.Contains(err.Error(), tt.wantIndex) {
				t.Fatalf("MergeUpdatesV1Context() error = %q, want index %q", err.Error(), tt.wantIndex)
			}
		})
	}
}
