package yjsbridge

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
	"github.com/drksbr/yjs-crdt-golang-server/internal/yupdate"
)

var (
	v2UpdatePayload = []byte{0x00, 0x00, 0x02, 0xa5, 0x01, 0x00, 0x00, 0x01, 0x04, 0x06, 0x03, 0x74, 0x68, 0x69, 0x01, 0x02, 0x01, 0x01, 0x00, 0x00, 0x01, 0x01, 0x00, 0x00}
)

func TestPublicUpdateAPIFormatsAndDispatch(t *testing.T) {
	t.Parallel()

	updateA := mustBuildStringUpdate(t, 1, 0, "x")
	updateB := mustBuildStringUpdate(t, 1, 1, "y")

	t.Run("FormatFromUpdate_dispatches_to_v1_and_v2", func(t *testing.T) {
		t.Parallel()

		got, err := FormatFromUpdate(updateA)
		if err != nil {
			t.Fatalf("FormatFromUpdate(v1) unexpected error: %v", err)
		}
		if got != UpdateFormatV1 {
			t.Fatalf("FormatFromUpdate(v1) = %s, want %s", got, UpdateFormatV1)
		}

		got, err = FormatFromUpdate(v2UpdatePayload)
		if err != nil {
			t.Fatalf("FormatFromUpdate(v2) unexpected error: %v", err)
		}
		if got != UpdateFormatV2 {
			t.Fatalf("FormatFromUpdate(v2) = %s, want %s", got, UpdateFormatV2)
		}
	})

	t.Run("FormatFromUpdates_context_matches_non_context_and_is_indexed", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		got, err := FormatFromUpdatesContext(ctx, updateA, updateB)
		if err != nil {
			t.Fatalf("FormatFromUpdatesContext(v1, v1) unexpected error: %v", err)
		}
		if got != UpdateFormatV1 {
			t.Fatalf("FormatFromUpdatesContext(v1, v1) = %s, want %s", got, UpdateFormatV1)
		}

		gotFromUpdates, err := FormatFromUpdates(updateA, updateB)
		if err != nil {
			t.Fatalf("FormatFromUpdates(v1, v1) unexpected error: %v", err)
		}
		if gotFromUpdates != got {
			t.Fatalf("FormatFromUpdates(v1, v1) = %s, want %s", gotFromUpdates, got)
		}
	})

	t.Run("FormatFromUpdates_rejects_mixed_formats", func(t *testing.T) {
		t.Parallel()

		_, err := FormatFromUpdates(updateA, v2UpdatePayload)
		if !errors.Is(err, ErrMismatchedUpdateFormats) {
			t.Fatalf("FormatFromUpdates(v1, v2) error = %v, want %v", err, ErrMismatchedUpdateFormats)
		}
	})

	t.Run("ValidateUpdate_decodes_detected_v2", func(t *testing.T) {
		t.Parallel()

		if err := ValidateUpdate(v2UpdatePayload); err != nil {
			t.Fatalf("ValidateUpdate(v2) unexpected error: %v", err)
		}
		if err := ValidateUpdate([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); !errors.Is(err, varint.ErrUnexpectedEOF) {
			t.Fatalf("ValidateUpdate(malformed v2) error = %v, want %v", err, varint.ErrUnexpectedEOF)
		}
	})

	t.Run("ValidateUpdates_context_preserves_index", func(t *testing.T) {
		t.Parallel()

		if err := ValidateUpdates(updateA, v2UpdatePayload); err != nil {
			t.Fatalf("ValidateUpdates(v1, v2) unexpected error: %v", err)
		}

		err := ValidateUpdates(updateA, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
		if err == nil {
			t.Fatal("ValidateUpdates() error = nil, want malformed update")
		}
		if !errors.Is(err, varint.ErrUnexpectedEOF) {
			t.Fatalf("ValidateUpdates() error = %v, want %v", err, varint.ErrUnexpectedEOF)
		}
		if !strings.Contains(err.Error(), "update[1]") {
			t.Fatalf("ValidateUpdates() error = %v, want update index 1", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := ValidateUpdatesContext(ctx, updateA); !errors.Is(err, context.Canceled) {
			t.Fatalf("ValidateUpdatesContext(cancelled) error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestPublicUpdateAPIMergeAndDiffContract(t *testing.T) {
	t.Parallel()

	updateA := mustBuildStringUpdate(t, 1, 0, "x")
	updateB := mustBuildStringUpdate(t, 1, 1, "y")
	merged, err := yupdate.MergeUpdatesV1(updateA, updateB)
	if err != nil {
		t.Fatalf("MergeUpdatesV1(v1, v1) unexpected error: %v", err)
	}

	t.Run("MergeUpdates_dispatches_v1", func(t *testing.T) {
		t.Parallel()

		got, err := MergeUpdates(updateA, updateB)
		if err != nil {
			t.Fatalf("MergeUpdates() unexpected error: %v", err)
		}
		if !bytes.Equal(got, merged) {
			t.Fatalf("MergeUpdates() = %v, want %v", got, merged)
		}

		gotCtx, err := MergeUpdatesContext(context.Background(), updateA, updateB)
		if err != nil {
			t.Fatalf("MergeUpdatesContext() unexpected error: %v", err)
		}
		if !bytes.Equal(gotCtx, merged) {
			t.Fatalf("MergeUpdatesContext() = %v, want %v", gotCtx, merged)
		}
	})

	t.Run("MergeUpdates_respects_context_cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := MergeUpdatesContext(ctx, updateA, updateB)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("MergeUpdatesContext() error = %v, want %v", err, context.Canceled)
		}
	})

	t.Run("MergeUpdates_handles_empty_payloads_as_noop", func(t *testing.T) {
		t.Parallel()

		got, err := MergeUpdates(nil, nil)
		if err != nil {
			t.Fatalf("MergeUpdates(nil, nil) unexpected error: %v", err)
		}
		if !bytes.Equal(got, emptyV1Update(t)) {
			t.Fatalf("MergeUpdates(nil, nil) = %v, want %v", got, emptyV1Update(t))
		}
	})

	t.Run("DiffUpdate_context_dispatches_and_returns_empty_for_full_state_vector", func(t *testing.T) {
		t.Parallel()

		fullStateVector, err := EncodeStateVectorFromUpdate(merged)
		if err != nil {
			t.Fatalf("EncodeStateVectorFromUpdate() unexpected error: %v", err)
		}

		diff, err := DiffUpdate(updateA, fullStateVector)
		if err != nil {
			t.Fatalf("DiffUpdate() unexpected error: %v", err)
		}
		if !bytes.Equal(diff, emptyV1Update(t)) {
			t.Fatalf("DiffUpdate() = %v, want %v", diff, emptyV1Update(t))
		}

		diffCtx, err := DiffUpdateContext(context.Background(), updateA, fullStateVector)
		if err != nil {
			t.Fatalf("DiffUpdateContext() unexpected error: %v", err)
		}
		if !bytes.Equal(diffCtx, emptyV1Update(t)) {
			t.Fatalf("DiffUpdateContext() = %v, want %v", diffCtx, emptyV1Update(t))
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = DiffUpdateContext(ctx, updateA, fullStateVector)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DiffUpdateContext() error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestPublicUpdateAPIStateVectorContract(t *testing.T) {
	t.Parallel()

	updateA := mustBuildStringUpdate(t, 1, 0, "x")
	updateB := mustBuildStringUpdate(t, 2, 0, "y")

	t.Run("StateVector_dispatches_v1_and_aggregates_inputs", func(t *testing.T) {
		t.Parallel()

		got, err := StateVectorFromUpdate(updateA)
		if err != nil {
			t.Fatalf("StateVectorFromUpdate() unexpected error: %v", err)
		}
		if got[1] != 1 || len(got) != 1 {
			t.Fatalf("StateVectorFromUpdate() = %#v, want map[1:1]", got)
		}

		aggregated, err := StateVectorFromUpdates(updateA, updateB)
		if err != nil {
			t.Fatalf("StateVectorFromUpdates() unexpected error: %v", err)
		}
		if aggregated[1] != 1 || aggregated[2] != 1 || len(aggregated) != 2 {
			t.Fatalf("StateVectorFromUpdates() = %#v, want map[1:1 2:1]", aggregated)
		}

		svEncoded, err := EncodeStateVectorFromUpdates(updateA, nil, updateB)
		if err != nil {
			t.Fatalf("EncodeStateVectorFromUpdates() unexpected error: %v", err)
		}

		decoded, err := DecodeStateVector(svEncoded)
		if err != nil {
			t.Fatalf("DecodeStateVector() unexpected error: %v", err)
		}
		if decoded[1] != 1 || decoded[2] != 1 || len(decoded) != 2 {
			t.Fatalf("EncodeStateVectorFromUpdates() decoded = %#v, want map[1:1 2:1]", decoded)
		}
	})

	t.Run("StateVectorContext_respects_cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := StateVectorFromUpdatesContext(ctx, updateA, updateB)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("StateVectorFromUpdatesContext() error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestPublicUpdateAPIContentIDsAndIntersectContract(t *testing.T) {
	t.Parallel()

	updateA := mustBuildStringUpdate(t, 1, 0, "x")
	updateB := mustBuildStringUpdate(t, 1, 1, "y")
	merged, err := MergeUpdates(updateA, updateB)
	if err != nil {
		t.Fatalf("MergeUpdates() unexpected error: %v", err)
	}

	t.Run("ContentIDs_dispatches_v1_and_intersects", func(t *testing.T) {
		t.Parallel()

		got, err := CreateContentIDsFromUpdate(merged)
		if err != nil {
			t.Fatalf("CreateContentIDsFromUpdate() unexpected error: %v", err)
		}
		if got.IsEmpty() {
			t.Fatalf("CreateContentIDsFromUpdate() should not be empty, got %#v", got)
		}
		t.Logf("CreateContentIDsFromUpdate(merged) = %#v", got.InsertRanges())

		filter := NewContentIDs()
		if err := filter.AddInsert(1, 1, 1); err != nil {
			t.Fatalf("filter.AddInsert() unexpected error: %v", err)
		}

		intersected, err := IntersectUpdateWithContentIDs(merged, filter)
		if err != nil {
			t.Fatalf("IntersectUpdateWithContentIDs() unexpected error: %v", err)
		}

		gotFromIntersect, err := CreateContentIDsFromUpdate(intersected)
		if err != nil {
			t.Fatalf("CreateContentIDsFromUpdate(intersect) unexpected error: %v", err)
		}

		if !IsSubsetContentIDs(gotFromIntersect, got) {
			t.Fatalf("CreateContentIDsFromUpdate(IntersectUpdate...) = %#v, want subset %#v", gotFromIntersect, got)
		}
	})

	t.Run("ContentIDsFromUpdates_aggregates_and_ignores_empty_payloads", func(t *testing.T) {
		t.Parallel()

		got, err := ContentIDsFromUpdates(nil, updateA, []byte{}, updateB)
		if err != nil {
			t.Fatalf("ContentIDsFromUpdates() unexpected error: %v", err)
		}

		fromMerged, err := CreateContentIDsFromUpdate(merged)
		if err != nil {
			t.Fatalf("CreateContentIDsFromUpdate() unexpected error: %v", err)
		}

		if !IsSubsetContentIDs(got, fromMerged) {
			t.Fatalf("ContentIDsFromUpdates() = %#v, want subset %#v", got, fromMerged)
		}
		if !IsSubsetContentIDs(fromMerged, got) {
			t.Fatalf("ContentIDsFromUpdates() = %#v, want %#v", got, fromMerged)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = ContentIDsFromUpdatesContext(ctx, nil, updateA)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ContentIDsFromUpdatesContext() error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestPublicUpdateAPIV2LimitsAreExplicitlyEnforced(t *testing.T) {
	t.Parallel()

	updateV1 := mustBuildStringUpdate(t, 1, 0, "x")
	filter := NewContentIDs()
	if err := filter.AddInsert(1, 0, 1); err != nil {
		t.Fatalf("filter.AddInsert() unexpected error: %v", err)
	}

	t.Run("single_update_mutating_apis_use_v1_conversion", func(t *testing.T) {
		t.Parallel()

		converted, err := ConvertUpdateToV1(v2UpdatePayload)
		if err != nil {
			t.Fatalf("ConvertUpdateToV1(v2) unexpected error: %v", err)
		}

		merged, err := MergeUpdates(v2UpdatePayload)
		if err != nil {
			t.Fatalf("MergeUpdates(v2) unexpected error: %v", err)
		}
		if !bytes.Equal(merged, converted) {
			t.Fatalf("MergeUpdates(v2) = %x, want %x", merged, converted)
		}

		emptyStateVector, err := EncodeStateVectorFromUpdate(NewPersistedSnapshot().UpdateV1)
		if err != nil {
			t.Fatalf("EncodeStateVectorFromUpdate(empty) unexpected error: %v", err)
		}
		diff, err := DiffUpdate(v2UpdatePayload, emptyStateVector)
		if err != nil {
			t.Fatalf("DiffUpdate(v2) unexpected error: %v", err)
		}
		wantDiff, err := DiffUpdate(converted, emptyStateVector)
		if err != nil {
			t.Fatalf("DiffUpdate(v1) unexpected error: %v", err)
		}
		if !bytes.Equal(diff, wantDiff) {
			t.Fatalf("DiffUpdate(v2) = %x, want %x", diff, wantDiff)
		}

		intersected, err := IntersectUpdateWithContentIDs(v2UpdatePayload, filter)
		if err != nil {
			t.Fatalf("IntersectUpdateWithContentIDs(v2) unexpected error: %v", err)
		}
		wantIntersected, err := IntersectUpdateWithContentIDs(converted, filter)
		if err != nil {
			t.Fatalf("IntersectUpdateWithContentIDs(v1) unexpected error: %v", err)
		}
		if !bytes.Equal(intersected, wantIntersected) {
			t.Fatalf("IntersectUpdateWithContentIDs(v2) = %x, want %x", intersected, wantIntersected)
		}
	})

	t.Run("single_update_v2_derived_apis_use_v1_conversion", func(t *testing.T) {
		t.Parallel()

		converted, err := ConvertUpdateToV1(v2UpdatePayload)
		if err != nil {
			t.Fatalf("ConvertUpdateToV1(v2) unexpected error: %v", err)
		}

		gotStateVector, err := StateVectorFromUpdate(v2UpdatePayload)
		if err != nil {
			t.Fatalf("StateVectorFromUpdate(v2) unexpected error: %v", err)
		}
		wantStateVector, err := StateVectorFromUpdate(converted)
		if err != nil {
			t.Fatalf("StateVectorFromUpdate(v1) unexpected error: %v", err)
		}
		if len(gotStateVector) != len(wantStateVector) || gotStateVector[101] != wantStateVector[101] {
			t.Fatalf("StateVectorFromUpdate(v2) = %#v, want %#v", gotStateVector, wantStateVector)
		}

		gotEncoded, err := EncodeStateVectorFromUpdate(v2UpdatePayload)
		if err != nil {
			t.Fatalf("EncodeStateVectorFromUpdate(v2) unexpected error: %v", err)
		}
		wantEncoded, err := EncodeStateVectorFromUpdate(converted)
		if err != nil {
			t.Fatalf("EncodeStateVectorFromUpdate(v1) unexpected error: %v", err)
		}
		if !bytes.Equal(gotEncoded, wantEncoded) {
			t.Fatalf("EncodeStateVectorFromUpdate(v2) = %x, want %x", gotEncoded, wantEncoded)
		}

		gotContentIDs, err := CreateContentIDsFromUpdate(v2UpdatePayload)
		if err != nil {
			t.Fatalf("CreateContentIDsFromUpdate(v2) unexpected error: %v", err)
		}
		if gotContentIDs == nil || gotContentIDs.IsEmpty() {
			t.Fatalf("CreateContentIDsFromUpdate(v2) = %#v, want non-empty content ids", gotContentIDs)
		}
	})

	t.Run("multi_update_v2_apis_use_canonical_v1", func(t *testing.T) {
		t.Parallel()

		firstV2 := mustDecodeHexPayload(t, "000002a50100000104060374686901020101000001010000")
		secondV2 := mustDecodeHexPayload(t, "0000048a03a50101020001840301210100000001010000")
		mergedV2 := mustDecodeHexPayload(t, "0000058a03e501000102000384000408042174686941000201010000020100010000")
		mergedV1 := mustDecodeHexPayload(t, "0201ca010084650101210165000401017402686900")

		merged, err := MergeUpdates(firstV2, secondV2)
		if err != nil {
			t.Fatalf("MergeUpdates(v2...) unexpected error: %v", err)
		}
		if !bytes.Equal(merged, mergedV1) {
			t.Fatalf("MergeUpdates(v2...) = %x, want %x", merged, mergedV1)
		}

		converted, err := ConvertUpdatesToV1(firstV2, secondV2)
		if err != nil {
			t.Fatalf("ConvertUpdatesToV1(v2...) unexpected error: %v", err)
		}
		if !bytes.Equal(converted, mergedV1) {
			t.Fatalf("ConvertUpdatesToV1(v2...) = %x, want %x", converted, mergedV1)
		}

		gotStateVector, err := StateVectorFromUpdates(firstV2, secondV2)
		if err != nil {
			t.Fatalf("StateVectorFromUpdates(v2...) unexpected error: %v", err)
		}
		wantStateVector, err := StateVectorFromUpdates(mergedV1)
		if err != nil {
			t.Fatalf("StateVectorFromUpdates(mergedV1) unexpected error: %v", err)
		}
		if len(gotStateVector) != len(wantStateVector) {
			t.Fatalf("StateVectorFromUpdates(v2...) = %#v, want %#v", gotStateVector, wantStateVector)
		}
		for client, clock := range wantStateVector {
			if gotStateVector[client] != clock {
				t.Fatalf("StateVectorFromUpdates(v2...)[%d] = %d, want %d", client, gotStateVector[client], clock)
			}
		}

		gotEncodedStateVector, err := EncodeStateVectorFromUpdates(firstV2, secondV2)
		if err != nil {
			t.Fatalf("EncodeStateVectorFromUpdates(v2...) unexpected error: %v", err)
		}
		wantEncodedStateVector, err := EncodeStateVectorFromUpdates(mergedV1)
		if err != nil {
			t.Fatalf("EncodeStateVectorFromUpdates(mergedV1) unexpected error: %v", err)
		}
		if !bytes.Equal(gotEncodedStateVector, wantEncodedStateVector) {
			t.Fatalf("EncodeStateVectorFromUpdates(v2...) = %x, want %x", gotEncodedStateVector, wantEncodedStateVector)
		}

		stateVector, err := EncodeStateVectorFromUpdate(convertedTextInsertV1ForTest(t))
		if err != nil {
			t.Fatalf("EncodeStateVectorFromUpdate(first v1) unexpected error: %v", err)
		}
		diff, err := DiffUpdate(mergedV2, stateVector)
		if err != nil {
			t.Fatalf("DiffUpdate(mergedV2) unexpected error: %v", err)
		}
		wantDiff, err := DiffUpdate(mergedV1, stateVector)
		if err != nil {
			t.Fatalf("DiffUpdate(mergedV1) unexpected error: %v", err)
		}
		if !bytes.Equal(diff, wantDiff) {
			t.Fatalf("DiffUpdate(mergedV2) = %x, want %x", diff, wantDiff)
		}

		contentIDs, err := CreateContentIDsFromUpdate(mergedV1)
		if err != nil {
			t.Fatalf("CreateContentIDsFromUpdate(mergedV1) unexpected error: %v", err)
		}
		intersected, err := IntersectUpdateWithContentIDs(mergedV2, contentIDs)
		if err != nil {
			t.Fatalf("IntersectUpdateWithContentIDs(mergedV2) unexpected error: %v", err)
		}
		wantIntersected, err := IntersectUpdateWithContentIDs(mergedV1, contentIDs)
		if err != nil {
			t.Fatalf("IntersectUpdateWithContentIDs(mergedV1) unexpected error: %v", err)
		}
		if !bytes.Equal(intersected, wantIntersected) {
			t.Fatalf("IntersectUpdateWithContentIDs(mergedV2) = %x, want %x", intersected, wantIntersected)
		}
	})

	t.Run("aggregated_apis_keep_mixed_format_or_v2_precedence", func(t *testing.T) {
		t.Parallel()

		_, err := StateVectorFromUpdates(updateV1, v2UpdatePayload)
		if !errors.Is(err, ErrMismatchedUpdateFormats) {
			t.Fatalf("StateVectorFromUpdates(v1, v2) error = %v, want %v", err, ErrMismatchedUpdateFormats)
		}

		contentIDs, err := ContentIDsFromUpdates(nil, v2UpdatePayload)
		if err != nil {
			t.Fatalf("ContentIDsFromUpdates(v2-only) unexpected error: %v", err)
		}
		if contentIDs == nil || contentIDs.IsEmpty() {
			t.Fatalf("ContentIDsFromUpdates(v2-only) = %#v, want non-empty content ids", contentIDs)
		}
	})
}

func convertedTextInsertV1ForTest(t *testing.T) []byte {
	t.Helper()

	return mustDecodeHexPayload(t, "010165000401017402686900")
}

func mustDecodeHexPayload(t *testing.T, value string) []byte {
	t.Helper()

	data, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q) unexpected error: %v", value, err)
	}
	return data
}

func mustBuildStringUpdate(t *testing.T, client, clock uint32, value string) []byte {
	t.Helper()

	parent, err := ytypes.NewParentRoot("doc")
	if err != nil {
		t.Fatalf("NewParentRoot() unexpected error: %v", err)
	}

	raw := varint.Append(nil, uint32(len(value)))
	raw = append(raw, value...)

	item, err := ytypes.NewItem(ytypes.ID{Client: client, Clock: clock}, yupdate.ParsedContent{
		Ref:       4,
		LengthVal: uint32(len(utf16.Encode([]rune(value)))),
		Countable: true,
		Raw:       raw,
		Text:      value,
	}, ytypes.ItemOptions{Parent: parent})
	if err != nil {
		t.Fatalf("NewItem() unexpected error: %v", err)
	}

	update, err := yupdate.EncodeV1(&yupdate.DecodedUpdate{Structs: []ytypes.Struct{item}, DeleteSet: ytypes.NewDeleteSet()})
	if err != nil {
		t.Fatalf("EncodeV1() unexpected error: %v", err)
	}

	return update
}

func emptyV1Update(t *testing.T) []byte {
	t.Helper()

	return yupdate.AppendDeleteSetBlockV1(varint.Append(nil, 0), ytypes.NewDeleteSet())
}
