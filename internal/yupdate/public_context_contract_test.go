package yupdate

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestPublicContextAPIsMatchLegacyV1(t *testing.T) {
	t.Parallel()

	left, right, diffUpdate, stateVector, intersectUpdate, filter := publicContextContractV1Fixtures()

	t.Run("FormatFromUpdates", func(t *testing.T) {
		t.Parallel()

		got, err := FormatFromUpdatesContext(context.Background(), left, right)
		if err != nil {
			t.Fatalf("FormatFromUpdatesContext() unexpected error: %v", err)
		}

		want, err := FormatFromUpdates(left, right)
		if err != nil {
			t.Fatalf("FormatFromUpdates() unexpected error: %v", err)
		}

		if got != want {
			t.Fatalf("FormatFromUpdatesContext() = %s, want %s", got, want)
		}
	})

	t.Run("MergeUpdates", func(t *testing.T) {
		t.Parallel()

		got, err := MergeUpdatesContext(context.Background(), left, right)
		if err != nil {
			t.Fatalf("MergeUpdatesContext() unexpected error: %v", err)
		}

		want, err := MergeUpdates(left, right)
		if err != nil {
			t.Fatalf("MergeUpdates() unexpected error: %v", err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("MergeUpdatesContext() = %v, want %v", got, want)
		}
	})

	t.Run("DiffUpdate", func(t *testing.T) {
		t.Parallel()

		got, err := DiffUpdateContext(context.Background(), diffUpdate, stateVector)
		if err != nil {
			t.Fatalf("DiffUpdateContext() unexpected error: %v", err)
		}

		want, err := DiffUpdate(diffUpdate, stateVector)
		if err != nil {
			t.Fatalf("DiffUpdate() unexpected error: %v", err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("DiffUpdateContext() = %v, want %v", got, want)
		}
	})

	t.Run("IntersectUpdateWithContentIDs", func(t *testing.T) {
		t.Parallel()

		got, err := IntersectUpdateWithContentIDsContext(context.Background(), intersectUpdate, filter)
		if err != nil {
			t.Fatalf("IntersectUpdateWithContentIDsContext() unexpected error: %v", err)
		}

		want, err := IntersectUpdateWithContentIDs(intersectUpdate, filter)
		if err != nil {
			t.Fatalf("IntersectUpdateWithContentIDs() unexpected error: %v", err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("IntersectUpdateWithContentIDsContext() = %v, want %v", got, want)
		}
	})
}

func TestPublicContextAPIsRespectCanceledContextOnV1(t *testing.T) {
	t.Parallel()

	left, right, diffUpdate, stateVector, intersectUpdate, filter := publicContextContractV1Fixtures()

	t.Run("FormatFromUpdates", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := FormatFromUpdatesContext(ctx, left, right)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("FormatFromUpdatesContext() error = %v, want context.Canceled", err)
		}
	})

	t.Run("MergeUpdates", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := MergeUpdatesContext(ctx, left, right)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("MergeUpdatesContext() error = %v, want context.Canceled", err)
		}
	})

	t.Run("DiffUpdate", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := DiffUpdateContext(ctx, diffUpdate, stateVector)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DiffUpdateContext() error = %v, want context.Canceled", err)
		}
	})

	t.Run("IntersectUpdateWithContentIDs", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := IntersectUpdateWithContentIDsContext(ctx, intersectUpdate, filter)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("IntersectUpdateWithContentIDsContext() error = %v, want context.Canceled", err)
		}
	})
}

func publicContextContractV1Fixtures() (left, right, diffUpdate, stateVector, intersectUpdate []byte, filter *ContentIDs) {
	left = buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				gc(1),
			},
		},
	)
	right = buildUpdate(
		clientBlock{
			client: 4,
			clock:  1,
			structs: []structEncoding{
				itemString(rootParent("doc"), "x"),
			},
		},
	)
	diffUpdate = buildUpdate(
		clientBlock{
			client: 8,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "abcd"),
			},
		},
		deleteRange{
			client: 8,
			clock:  3,
			length: 1,
		},
	)
	stateVector = encodeStateVectorEntry(8, 2)
	intersectUpdate = diffUpdate
	filter = NewContentIDs()
	_ = filter.Inserts.Add(8, 1, 2)
	_ = filter.Deletes.Add(8, 3, 1)
	return
}
