package yjsbridge

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"unicode/utf16"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
	"yjs-go-bridge/internal/yupdate"
)

var (
	v2UpdatePayload = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
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

	t.Run("single_update_apis_reject_v2", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			call func() error
		}{
			{
				name: "StateVectorFromUpdate",
				call: func() error { _, err := StateVectorFromUpdate(v2UpdatePayload); return err },
			},
			{
				name: "EncodeStateVectorFromUpdate",
				call: func() error { _, err := EncodeStateVectorFromUpdate(v2UpdatePayload); return err },
			},
			{
				name: "CreateContentIDsFromUpdate",
				call: func() error { _, err := CreateContentIDsFromUpdate(v2UpdatePayload); return err },
			},
			{
				name: "IntersectUpdateWithContentIDs",
				call: func() error { _, err := IntersectUpdateWithContentIDs(v2UpdatePayload, filter); return err },
			},
			{
				name: "DiffUpdate",
				call: func() error { _, err := DiffUpdate(v2UpdatePayload, nil); return err },
			},
			{
				name: "MergeUpdates",
				call: func() error { _, err := MergeUpdates(v2UpdatePayload); return err },
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := tt.call()
				if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
					t.Fatalf("%s error = %v, want %v", tt.name, err, ErrUnsupportedUpdateFormatV2)
				}
			})
		}
	})

	t.Run("aggregated_apis_keep_mixed_format_or_v2_precedence", func(t *testing.T) {
		t.Parallel()

		_, err := StateVectorFromUpdates(updateV1, v2UpdatePayload)
		if !errors.Is(err, ErrMismatchedUpdateFormats) {
			t.Fatalf("StateVectorFromUpdates(v1, v2) error = %v, want %v", err, ErrMismatchedUpdateFormats)
		}

		_, err = ContentIDsFromUpdates(nil, v2UpdatePayload)
		if !errors.Is(err, ErrUnsupportedUpdateFormatV2) {
			t.Fatalf("ContentIDsFromUpdates(v2-only) error = %v, want %v", err, ErrUnsupportedUpdateFormatV2)
		}
	})
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
