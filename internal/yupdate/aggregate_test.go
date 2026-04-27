package yupdate

import (
	"context"
	"errors"
	"testing"
)

func TestAggregatePayloadsInParallelReturnsContextCanceledWithoutReducer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reducerCalled := false
	got, err := aggregatePayloadsInParallel(
		ctx,
		[][]byte{{0x01}, {0x02}},
		1,
		func(ctx context.Context, index int, _ []byte) (int, error) {
			if index == 0 {
				cancel()
				return 1, nil
			}
			<-ctx.Done()
			return 0, ctx.Err()
		},
		func(context.Context, []int) (int, error) {
			reducerCalled = true
			return 99, nil
		},
	)
	if got != 0 {
		t.Fatalf("aggregatePayloadsInParallel() value = %d, want 0 on cancel", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("aggregatePayloadsInParallel() error = %v, want context.Canceled", err)
	}
	if reducerCalled {
		t.Fatal("aggregatePayloadsInParallel() called reducer after cancellation")
	}
}

func TestStateVectorFromUpdatesContextRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := buildUpdate(
		clientBlock{
			client: 1,
			clock:  0,
			structs: []structEncoding{
				itemDeleted(rootParent("doc"), 1),
			},
		},
	)

	_, err := StateVectorFromUpdatesContext(ctx, update)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StateVectorFromUpdatesContext() error = %v, want context.Canceled", err)
	}
}

func TestContentIDsFromUpdatesContextRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := buildUpdate(
		clientBlock{
			client: 2,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
	)

	_, err := ContentIDsFromUpdatesContext(ctx, update)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ContentIDsFromUpdatesContext() error = %v, want context.Canceled", err)
	}
}

func TestMergeUpdatesV1ContextRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := buildUpdate(
		clientBlock{
			client: 3,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
	)

	_, err := MergeUpdatesV1Context(ctx, update)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("MergeUpdatesV1Context() error = %v, want context.Canceled", err)
	}
}

func TestDiffUpdateV1ContextRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := buildUpdate(
		clientBlock{
			client: 4,
			clock:  0,
			structs: []structEncoding{
				itemString(rootParent("doc"), "ab"),
			},
		},
	)

	_, err := DiffUpdateV1Context(ctx, update, encodeStateVectorEntry(4, 0))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DiffUpdateV1Context() error = %v, want context.Canceled", err)
	}
}

func TestIntersectUpdateWithContentIDsV1ContextRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	update := buildUpdate(
		clientBlock{
			client: 5,
			clock:  0,
			structs: []structEncoding{
				itemJSON(rootParent("doc"), `"a"`, `"b"`),
			},
		},
	)
	ids := NewContentIDs()
	_ = ids.Inserts.Add(5, 0, 1)

	_, err := IntersectUpdateWithContentIDsV1Context(ctx, update, ids)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("IntersectUpdateWithContentIDsV1Context() error = %v, want context.Canceled", err)
	}
}
