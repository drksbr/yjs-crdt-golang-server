package yupdate

import (
	"context"
	"testing"
)

func TestValidateUpdatesFormatWithReasonPreservesV2AfterEmptyPrefixes(t *testing.T) {
	t.Parallel()

	v2 := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	updates := [][]byte{nil, []byte{}, v2}

	tests := []struct {
		name string
		call func(...[]byte) (UpdateFormat, error)
	}{
		{
			name: "public_wrapper",
			call: ValidateUpdatesFormatWithReason,
		},
		{
			name: "context_wrapper",
			call: func(updates ...[]byte) (UpdateFormat, error) {
				return ValidateUpdatesFormatWithReasonContext(context.Background(), updates...)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.call(updates...)
			if err != nil {
				t.Fatalf("%s() unexpected error: %v", tt.name, err)
			}
			if got != UpdateFormatV2 {
				t.Fatalf("%s() = %s, want %s", tt.name, got, UpdateFormatV2)
			}
		})
	}
}
