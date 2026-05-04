package yupdate

import (
	"bytes"
	"errors"
	"testing"
)

func TestEncodeV2RoundTripsYjsFixtures(t *testing.T) {
	t.Parallel()

	for _, fixture := range yjsV2Fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			v1 := mustDecodeHex(t, fixture.v1)
			decoded, err := DecodeV1(v1)
			if err != nil {
				t.Fatalf("DecodeV1() unexpected error: %v", err)
			}

			encodedV2, err := EncodeV2(decoded)
			if err != nil {
				t.Fatalf("EncodeV2() unexpected error: %v", err)
			}
			fixtureV2 := mustDecodeHex(t, fixture.v2)
			if !bytes.Equal(encodedV2, fixtureV2) {
				t.Fatalf("EncodeV2() = %x, want upstream fixture %x", encodedV2, fixtureV2)
			}
			format, err := FormatFromUpdate(encodedV2)
			if err != nil {
				t.Fatalf("FormatFromUpdate(EncodeV2()) unexpected error: %v", err)
			}
			if format != UpdateFormatV2 {
				t.Fatalf("FormatFromUpdate(EncodeV2()) = %s, want %s", format, UpdateFormatV2)
			}

			converted, err := ConvertUpdateToV1(encodedV2)
			if err != nil {
				t.Fatalf("ConvertUpdateToV1(EncodeV2()) unexpected error: %v", err)
			}
			if !bytes.Equal(converted, v1) {
				t.Fatalf("ConvertUpdateToV1(EncodeV2()) = %x, want %x", converted, v1)
			}

			convertedV2, err := ConvertUpdateToV2(v1)
			if err != nil {
				t.Fatalf("ConvertUpdateToV2(v1) unexpected error: %v", err)
			}
			if !bytes.Equal(convertedV2, fixtureV2) {
				t.Fatalf("ConvertUpdateToV2(v1) = %x, want upstream fixture %x", convertedV2, fixtureV2)
			}
		})
	}
}

func TestV2OutputAPIsPreserveV2Format(t *testing.T) {
	t.Parallel()

	for _, fixture := range yjsV2MultiUpdateFixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			updates := make([][]byte, 0, len(fixture.updates))
			for _, update := range fixture.updates {
				updates = append(updates, mustDecodeHex(t, update.v2))
			}
			mergedV1 := mustDecodeHex(t, fixture.mergedV1)
			upstreamMergedV2 := mustDecodeHex(t, fixture.mergedV2)

			convertedV2, err := ConvertUpdatesToV2(updates...)
			if err != nil {
				t.Fatalf("ConvertUpdatesToV2() unexpected error: %v", err)
			}
			if !bytes.Equal(convertedV2, upstreamMergedV2) {
				t.Fatalf("ConvertUpdatesToV2() = %x, want upstream fixture %x", convertedV2, upstreamMergedV2)
			}
			assertV2EquivalentToV1(t, "ConvertUpdatesToV2", convertedV2, mergedV1)

			mergedV2, err := MergeUpdatesV2(updates...)
			if err != nil {
				t.Fatalf("MergeUpdatesV2() unexpected error: %v", err)
			}
			if !bytes.Equal(mergedV2, upstreamMergedV2) {
				t.Fatalf("MergeUpdatesV2() = %x, want upstream fixture %x", mergedV2, upstreamMergedV2)
			}
			assertV2EquivalentToV1(t, "MergeUpdatesV2", mergedV2, mergedV1)

			stateVector, err := EncodeStateVectorFromUpdate(updates[0])
			if err != nil {
				t.Fatalf("EncodeStateVectorFromUpdate() unexpected error: %v", err)
			}
			diffV2, err := DiffUpdateV2(mergedV2, stateVector)
			if err != nil {
				t.Fatalf("DiffUpdateV2() unexpected error: %v", err)
			}
			diffV1, err := DiffUpdateV1(mergedV1, stateVector)
			if err != nil {
				t.Fatalf("DiffUpdateV1() unexpected error: %v", err)
			}
			assertV2EquivalentToV1(t, "DiffUpdateV2", diffV2, diffV1)

			contentIDs, err := CreateContentIDsFromUpdate(mergedV1)
			if err != nil {
				t.Fatalf("CreateContentIDsFromUpdate() unexpected error: %v", err)
			}
			intersectedV2, err := IntersectUpdateWithContentIDsV2(mergedV2, contentIDs)
			if err != nil {
				t.Fatalf("IntersectUpdateWithContentIDsV2() unexpected error: %v", err)
			}
			intersectedV1, err := IntersectUpdateWithContentIDsV1(mergedV1, contentIDs)
			if err != nil {
				t.Fatalf("IntersectUpdateWithContentIDsV1() unexpected error: %v", err)
			}
			assertV2EquivalentToV1(t, "IntersectUpdateWithContentIDsV2", intersectedV2, intersectedV1)
		})
	}
}

func TestV2OutputAggregateAPIsRejectMixedFormats(t *testing.T) {
	t.Parallel()

	v1 := buildUpdate(clientBlock{
		client: 1,
		clock:  0,
		structs: []structEncoding{
			itemString(rootParent("doc"), "v1"),
		},
	})
	v2 := mustDecodeHex(t, "000002a50100000104060374686901020101000001010000")

	calls := []struct {
		name string
		call func(...[]byte) ([]byte, error)
	}{
		{name: "ConvertUpdatesToV2", call: ConvertUpdatesToV2},
		{name: "MergeUpdatesV2", call: MergeUpdatesV2},
	}

	for _, tt := range calls {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.call(nil, v2, []byte{}, v1)
			if !errors.Is(err, ErrMismatchedUpdateFormats) {
				t.Fatalf("%s() error = %v, want %v", tt.name, err, ErrMismatchedUpdateFormats)
			}
		})
	}
}

func assertV2EquivalentToV1(t *testing.T, op string, gotV2, wantV1 []byte) {
	t.Helper()

	format, err := FormatFromUpdate(gotV2)
	if err != nil {
		t.Fatalf("%s format error: %v", op, err)
	}
	if format != UpdateFormatV2 {
		t.Fatalf("%s format = %s, want %s", op, format, UpdateFormatV2)
	}
	gotV1, err := ConvertUpdateToV1(gotV2)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(%s) unexpected error: %v", op, err)
	}
	if !bytes.Equal(gotV1, wantV1) {
		t.Fatalf("ConvertUpdateToV1(%s) = %x, want %x", op, gotV1, wantV1)
	}
}
