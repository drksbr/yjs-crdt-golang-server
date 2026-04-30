package ycluster

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
)

func TestStorageOwnershipCoordinatorPlansAndExecutesLoadSheddingRebalance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(128)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	nodeA := newRebalanceTestCoordinator(t, "node-a", resolver, store, time.Minute)
	nodeB := newRebalanceTestCoordinator(t, "node-b", resolver, store, time.Minute)
	keys := distinctRebalanceKeys(t, resolver, 4)

	for i := 0; i < 3; i++ {
		if _, err := nodeA.ClaimDocument(ctx, ClaimDocumentRequest{
			DocumentKey:      keys[i],
			Token:            fmt.Sprintf("node-a-%d", i),
			PlacementVersion: uint64(i + 1),
		}); err != nil {
			t.Fatalf("nodeA.ClaimDocument(%d) unexpected error: %v", i, err)
		}
	}
	if _, err := nodeB.ClaimDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      keys[3],
		Token:            "node-b-0",
		PlacementVersion: 4,
	}); err != nil {
		t.Fatalf("nodeB.ClaimDocument() unexpected error: %v", err)
	}

	plan, err := nodeB.PlanDocumentRebalance(ctx, RebalancePlanRequest{
		Documents:     keys,
		TargetHolders: []NodeID{"node-a", "node-b"},
		MaxMoves:      1,
	})
	if err != nil {
		t.Fatalf("PlanDocumentRebalance() unexpected error: %v", err)
	}
	if len(plan.Moves) != 1 {
		t.Fatalf("len(plan.Moves) = %d, want 1; plan=%#v", len(plan.Moves), plan)
	}
	move := plan.Moves[0]
	if move.From != "node-a" || move.To != "node-b" || move.Reason != RebalanceReasonLoadShedding {
		t.Fatalf("plan.Moves[0] = %#v, want node-a -> node-b load_shedding", move)
	}

	results, err := nodeB.ExecuteRebalancePlan(ctx, plan, RebalancePlanExecutionOptions{
		TTL: time.Minute,
		TokenForDocument: func(PlannedRebalance) string {
			return "rebalance-token"
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRebalancePlan() unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil || results[0].Result == nil || !results[0].Result.Changed {
		t.Fatalf("ExecuteRebalancePlan() = %#v, want one changed result", results)
	}

	resolution, err := nodeB.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: move.DocumentKey})
	if err != nil {
		t.Fatalf("LookupOwner(moved) unexpected error: %v", err)
	}
	if !resolution.Local || resolution.Placement.NodeID != "node-b" {
		t.Fatalf("LookupOwner(moved) = %#v, want local node-b", resolution)
	}
}

func TestStorageOwnershipCoordinatorPlansMissingOwnerPromotion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver := mustRebalanceTestResolver(t)
	nodeB := newRebalanceTestCoordinator(t, "node-b", resolver, store, time.Minute)
	key := storage.DocumentKey{Namespace: "tests", DocumentID: "plan-missing-owner"}

	plan, err := nodeB.PlanDocumentRebalance(ctx, RebalancePlanRequest{
		Documents:             []storage.DocumentKey{key},
		TargetHolders:         []NodeID{"node-b"},
		PromoteIfOwnerMissing: true,
	})
	if err != nil {
		t.Fatalf("PlanDocumentRebalance(missing) unexpected error: %v", err)
	}
	if len(plan.Moves) != 1 {
		t.Fatalf("len(plan.Moves) = %d, want 1", len(plan.Moves))
	}
	if plan.Moves[0].From != "" || plan.Moves[0].To != "node-b" || plan.Moves[0].Reason != RebalanceReasonMissingOwner {
		t.Fatalf("plan.Moves[0] = %#v, want missing -> node-b", plan.Moves[0])
	}

	results, err := nodeB.ExecuteRebalancePlan(ctx, plan, RebalancePlanExecutionOptions{TTL: time.Minute})
	if err != nil {
		t.Fatalf("ExecuteRebalancePlan(missing) unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Result == nil || !results[0].Result.PromotedFromLost {
		t.Fatalf("ExecuteRebalancePlan(missing) = %#v, want promoted result", results)
	}
}

func TestStorageOwnershipCoordinatorPlanRebalanceValidation(t *testing.T) {
	t.Parallel()

	coordinator := newRebalanceTestCoordinator(t, "node-a", mustRebalanceTestResolver(t), memory.New(), time.Minute)
	_, err := coordinator.PlanDocumentRebalance(context.Background(), RebalancePlanRequest{})
	if !errors.Is(err, ErrInvalidRebalancePlan) {
		t.Fatalf("PlanDocumentRebalance(empty targets) error = %v, want %v", err, ErrInvalidRebalancePlan)
	}

	_, err = coordinator.ExecuteRebalancePlan(context.Background(), nil, RebalancePlanExecutionOptions{})
	if !errors.Is(err, ErrInvalidRebalancePlan) {
		t.Fatalf("ExecuteRebalancePlan(nil) error = %v, want %v", err, ErrInvalidRebalancePlan)
	}
}

func TestStorageOwnershipCoordinatorExecuteRebalancePlanStopsWhenContextCanceled(t *testing.T) {
	t.Parallel()

	coordinator := newRebalanceTestCoordinator(t, "node-a", mustRebalanceTestResolver(t), memory.New(), time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, err := coordinator.ExecuteRebalancePlan(ctx, &RebalancePlan{
		Moves: []PlannedRebalance{{
			DocumentKey: storage.DocumentKey{Namespace: "tests", DocumentID: "cancelled-rebalance"},
			To:          "node-a",
			Reason:      RebalanceReasonMissingOwner,
		}},
	}, RebalancePlanExecutionOptions{TTL: time.Minute})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecuteRebalancePlan(cancelled) error = %v, want %v", err, context.Canceled)
	}
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want no moves after cancellation", len(results))
	}
}

func distinctRebalanceKeys(t *testing.T, resolver ShardResolver, count int) []storage.DocumentKey {
	t.Helper()

	keys := make([]storage.DocumentKey, 0, count)
	shards := make(map[ShardID]struct{}, count)
	for i := 0; len(keys) < count && i < 10000; i++ {
		key := storage.DocumentKey{Namespace: "tests", DocumentID: fmt.Sprintf("plan-doc-%d", i)}
		shardID, err := resolver.ResolveShard(key)
		if err != nil {
			t.Fatalf("ResolveShard(%#v) unexpected error: %v", key, err)
		}
		if _, ok := shards[shardID]; ok {
			continue
		}
		shards[shardID] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) != count {
		t.Fatalf("found %d distinct shard keys, want %d", len(keys), count)
	}
	return keys
}
