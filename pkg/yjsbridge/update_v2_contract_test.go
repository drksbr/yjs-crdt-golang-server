package yjsbridge

import (
	"bytes"
	"testing"
)

func TestPublicV2OutputAPIsReturnV2Payloads(t *testing.T) {
	t.Parallel()

	v1, err := ConvertUpdateToV1(v2UpdatePayload)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(v2) unexpected error: %v", err)
	}

	convertedV2, err := ConvertUpdateToV2(v1)
	if err != nil {
		t.Fatalf("ConvertUpdateToV2(v1) unexpected error: %v", err)
	}
	assertPublicV2EquivalentToV1(t, "ConvertUpdateToV2", convertedV2, v1)

	mergedV2, err := MergeUpdatesV2(v2UpdatePayload)
	if err != nil {
		t.Fatalf("MergeUpdatesV2(v2) unexpected error: %v", err)
	}
	assertPublicV2EquivalentToV1(t, "MergeUpdatesV2", mergedV2, v1)

	firstV2 := mustDecodeHexPayload(t, "000002a50100000104060374686901020101000001010000")
	secondV2 := mustDecodeHexPayload(t, "0000048a03a50101020001840301210100000001010000")
	mergedV1 := mustDecodeHexPayload(t, "0201ca010084650101210165000401017402686900")
	convertedManyV2, err := ConvertUpdatesToV2(firstV2, secondV2)
	if err != nil {
		t.Fatalf("ConvertUpdatesToV2(v2...) unexpected error: %v", err)
	}
	assertPublicV2EquivalentToV1(t, "ConvertUpdatesToV2", convertedManyV2, mergedV1)
	mergedManyV2, err := MergeUpdatesV2(firstV2, secondV2)
	if err != nil {
		t.Fatalf("MergeUpdatesV2(v2...) unexpected error: %v", err)
	}
	assertPublicV2EquivalentToV1(t, "MergeUpdatesV2 multi", mergedManyV2, mergedV1)

	emptyStateVector, err := EncodeStateVectorFromUpdate(NewPersistedSnapshot().UpdateV1)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdate(empty) unexpected error: %v", err)
	}
	diffV2, err := DiffUpdateV2(v2UpdatePayload, emptyStateVector)
	if err != nil {
		t.Fatalf("DiffUpdateV2(v2) unexpected error: %v", err)
	}
	diffV1, err := DiffUpdate(v1, emptyStateVector)
	if err != nil {
		t.Fatalf("DiffUpdate(v1) unexpected error: %v", err)
	}
	assertPublicV2EquivalentToV1(t, "DiffUpdateV2", diffV2, diffV1)

	filter := NewContentIDs()
	if err := filter.AddInsert(101, 0, 2); err != nil {
		t.Fatalf("filter.AddInsert() unexpected error: %v", err)
	}
	intersectedV2, err := IntersectUpdateWithContentIDsV2(v2UpdatePayload, filter)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDsV2(v2) unexpected error: %v", err)
	}
	intersectedV1, err := IntersectUpdateWithContentIDs(v1, filter)
	if err != nil {
		t.Fatalf("IntersectUpdateWithContentIDs(v1) unexpected error: %v", err)
	}
	assertPublicV2EquivalentToV1(t, "IntersectUpdateWithContentIDsV2", intersectedV2, intersectedV1)
}

func assertPublicV2EquivalentToV1(t *testing.T, op string, gotV2, wantV1 []byte) {
	t.Helper()

	format, err := FormatFromUpdate(gotV2)
	if err != nil {
		t.Fatalf("%s format error: %v", op, err)
	}
	if format != UpdateFormatV2 {
		t.Fatalf("%s format = %s, want %s", op, format, UpdateFormatV2)
	}
	if err := ValidateUpdate(gotV2); err != nil {
		t.Fatalf("ValidateUpdate(%s) unexpected error: %v", op, err)
	}
	gotV1, err := ConvertUpdateToV1(gotV2)
	if err != nil {
		t.Fatalf("ConvertUpdateToV1(%s) unexpected error: %v", op, err)
	}
	if !bytes.Equal(gotV1, wantV1) {
		t.Fatalf("ConvertUpdateToV1(%s) = %x, want %x", op, gotV1, wantV1)
	}
}
