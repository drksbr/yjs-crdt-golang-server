package yupdate

import (
	"bytes"
	"errors"
	"testing"
)

func TestV2NextStepMergeUpdatesConvertsDetectedV2(t *testing.T) {
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
	v2 := mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")

	got, err := MergeUpdates(v2)
	if err != nil {
		t.Fatalf("MergeUpdates(v2) unexpected error: %v", err)
	}
	want, err := ConvertUpdateToV1(v2)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(v2) unexpected error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("MergeUpdates(v2) = %x, want %x", got, want)
	}

	_, err = MergeUpdates(v1, v2)
	if !errors.Is(err, ErrMismatchedUpdateFormats) {
		t.Fatalf("MergeUpdates(v1, v2) error = %v, want %v", err, ErrMismatchedUpdateFormats)
	}
}

func TestV2NextStepAggregateAPIsConvertDetectedV2AfterEmptyPrefixes(t *testing.T) {
	t.Parallel()

	v2 := mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")

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
			if err != nil {
				t.Fatalf("%s() unexpected error: %v", api.name, err)
			}
		})
	}
}
