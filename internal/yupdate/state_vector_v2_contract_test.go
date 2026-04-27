package yupdate

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"yjs-go-bridge/internal/varint"
)

func TestStateVectorFromUpdatesContract(t *testing.T) {
	t.Parallel()

	v1 := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "state"),
			},
		},
	)
	v2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	type contractFn func(...[]byte) error
	apiCalls := []struct {
		name string
		call contractFn
	}{
		{
			name: "StateVectorFromUpdates",
			call: func(updates ...[]byte) error {
				_, err := StateVectorFromUpdates(updates...)
				return err
			},
		},
		{
			name: "EncodeStateVectorFromUpdates",
			call: func(updates ...[]byte) error {
				_, err := EncodeStateVectorFromUpdates(updates...)
				return err
			},
		},
	}

	tests := []struct {
		name      string
		updates   [][]byte
		wantErr   error
		wantIndex int
		hasIndex  bool
	}{
		{
			name:      "unsupported_v2_is_rejected_with_index",
			updates:   [][]byte{nil, []byte{}, v2},
			wantErr:   ErrUnsupportedUpdateFormatV2,
			wantIndex: 2,
			hasIndex:  true,
		},
		{
			name:    "mixed_v1_v2_is_rejected_without_index",
			updates: [][]byte{v1, v2},
			wantErr: ErrMismatchedUpdateFormats,
		},
		{
			name:      "malformed_update_preserves_index",
			updates:   [][]byte{v1, []byte{0x80}, v2},
			wantErr:   varint.ErrUnexpectedEOF,
			wantIndex: 1,
			hasIndex:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, api := range apiCalls {
				api := api
				t.Run(api.name, func(t *testing.T) {
					t.Parallel()

					err := api.call(tt.updates...)
					if err == nil {
						t.Fatalf("%s() error = nil, want %v", api.name, tt.wantErr)
					}
					if !errors.Is(err, tt.wantErr) {
						t.Fatalf("%s() error = %v, want %v", api.name, err, tt.wantErr)
					}
					if tt.hasIndex {
						expect := fmt.Sprintf("update[%d]", tt.wantIndex)
						if !strings.Contains(err.Error(), expect) {
							t.Fatalf("%s() error = %v, want %s", api.name, err, expect)
						}
						return
					}
					if strings.Contains(err.Error(), "update[") {
						t.Fatalf("%s() error = %v, want no update index context", api.name, err)
					}
				})
			}
		})
	}
}
