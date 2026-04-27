package yupdate

import (
	"errors"
	"strings"
	"testing"
)

func TestV2NextStepMergeUpdatesRejectsDetectedV2(t *testing.T) {
	t.Parallel()

	v1 := buildUpdate(
		clientBlock{
			client: 13,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "v1"),
			},
		},
	)
	v2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	tests := []struct {
		name      string
		updates   [][]byte
		wantErr   error
		wantIndex string
	}{
		{
			name:      "single_v2_payload_is_rejected",
			updates:   [][]byte{v2},
			wantErr:   ErrUnsupportedUpdateFormatV2,
			wantIndex: "update[0]",
		},
		{
			name:    "mixed_v1_and_v2_still_rejects_as_mismatch",
			updates: [][]byte{v1, v2},
			wantErr: ErrMismatchedUpdateFormats,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := MergeUpdates(tt.updates...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("MergeUpdates() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantIndex != "" && !strings.Contains(err.Error(), tt.wantIndex) {
				t.Fatalf("MergeUpdates() error = %v, want %s", err, tt.wantIndex)
			}
		})
	}
}

func TestV2NextStepAggregateAPIsRejectDetectedV2AfterEmptyPrefixes(t *testing.T) {
	t.Parallel()

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
		{
			name: "ContentIDsFromUpdates",
			call: func(updates ...[]byte) error {
				_, err := ContentIDsFromUpdates(updates...)
				return err
			},
		},
	}

	for _, api := range apiCalls {
		api := api
		t.Run(api.name, func(t *testing.T) {
			t.Parallel()

			err := api.call(nil, []byte{}, v2)
			if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
				t.Fatalf("%s() error = %v, want %v", api.name, err, ErrUnsupportedUpdateFormatV2)
			}
			if !strings.Contains(err.Error(), "update[2]") {
				t.Fatalf("%s() error = %v, want update index 2", api.name, err)
			}
		})
	}
}
