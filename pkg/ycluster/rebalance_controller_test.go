package ycluster

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
)

func TestRebalanceControllerRunOnceExecutesPlannedMoves(t *testing.T) {
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
		if _, err := nodeA.ClaimDocument(ctx, ClaimDocumentRequest{DocumentKey: keys[i], Token: "node-a"}); err != nil {
			t.Fatalf("nodeA.ClaimDocument(%d) unexpected error: %v", i, err)
		}
	}
	if _, err := nodeB.ClaimDocument(ctx, ClaimDocumentRequest{DocumentKey: keys[3], Token: "node-b"}); err != nil {
		t.Fatalf("nodeB.ClaimDocument() unexpected error: %v", err)
	}

	var observedPlanMoves int
	controller, err := NewRebalanceController(RebalanceControllerConfig{
		Coordinator:   nodeB,
		Source:        RebalanceDocumentSourceFunc(func(context.Context) ([]storage.DocumentKey, error) { return keys, nil }),
		TargetHolders: []NodeID{"node-a", "node-b"},
		Interval:      time.Minute,
		MaxMoves:      1,
		TTL:           time.Minute,
		OnPlan: func(plan *RebalancePlan) {
			observedPlanMoves = len(plan.Moves)
		},
	})
	if err != nil {
		t.Fatalf("NewRebalanceController() unexpected error: %v", err)
	}

	result, err := controller.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce() unexpected error: %v", err)
	}
	if result.Documents != len(keys) {
		t.Fatalf("RunOnce().Documents = %d, want %d", result.Documents, len(keys))
	}
	if observedPlanMoves != 1 || result.Plan == nil || len(result.Plan.Moves) != 1 || len(result.Results) != 1 {
		t.Fatalf("RunOnce() = %#v observedPlanMoves=%d, want one move/result", result, observedPlanMoves)
	}
	moved := result.Plan.Moves[0].DocumentKey
	resolution, err := nodeB.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: moved})
	if err != nil {
		t.Fatalf("LookupOwner(moved) unexpected error: %v", err)
	}
	if !resolution.Local || resolution.Placement.NodeID != "node-b" {
		t.Fatalf("LookupOwner(moved) = %#v, want local node-b", resolution)
	}
}

func TestRebalanceControllerRunOnceUsesDynamicHealthyTargets(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(128)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	nodeA := newRebalanceTestCoordinator(t, "node-a", resolver, store, time.Minute)
	nodeB := newRebalanceTestCoordinator(t, "node-b", resolver, store, time.Minute)
	keys := distinctRebalanceKeys(t, resolver, 2)

	if _, err := nodeA.ClaimDocument(ctx, ClaimDocumentRequest{DocumentKey: keys[0], Token: "node-a"}); err != nil {
		t.Fatalf("nodeA.ClaimDocument() unexpected error: %v", err)
	}
	if _, err := nodeB.ClaimDocument(ctx, ClaimDocumentRequest{DocumentKey: keys[1], Token: "node-b"}); err != nil {
		t.Fatalf("nodeB.ClaimDocument() unexpected error: %v", err)
	}

	targetSource, err := NewHealthyRebalanceTargetSource(HealthyRebalanceTargetSourceConfig{
		Source: NewStaticNodeHealthSource([]NodeHealth{
			{NodeID: "node-a", State: NodeHealthDraining, LastSeen: now},
			{NodeID: "node-b", State: NodeHealthReady, LastSeen: now},
		}),
		MaxStaleness: time.Minute,
		Now:          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewHealthyRebalanceTargetSource() unexpected error: %v", err)
	}

	controller, err := NewRebalanceController(RebalanceControllerConfig{
		Coordinator:  nodeB,
		Source:       RebalanceDocumentSourceFunc(func(context.Context) ([]storage.DocumentKey, error) { return keys, nil }),
		TargetSource: targetSource,
		Interval:     time.Minute,
		MaxMoves:     1,
		TTL:          time.Minute,
	})
	if err != nil {
		t.Fatalf("NewRebalanceController() unexpected error: %v", err)
	}

	result, err := controller.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce() unexpected error: %v", err)
	}
	if len(result.Targets) != 1 || result.Targets[0] != "node-b" {
		t.Fatalf("RunOnce().Targets = %#v, want node-b only", result.Targets)
	}
	if result.Plan == nil || len(result.Plan.Moves) != 1 {
		t.Fatalf("RunOnce().Plan = %#v, want one move", result.Plan)
	}
	move := result.Plan.Moves[0]
	if move.From != "node-a" || move.To != "node-b" || move.Reason != RebalanceReasonOwnerOutsideTargets {
		t.Fatalf("RunOnce().Plan.Moves[0] = %#v, want node-a -> node-b outside targets", move)
	}

	resolution, err := nodeB.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: move.DocumentKey})
	if err != nil {
		t.Fatalf("LookupOwner(moved) unexpected error: %v", err)
	}
	if !resolution.Local || resolution.Placement.NodeID != "node-b" {
		t.Fatalf("LookupOwner(moved) = %#v, want local node-b", resolution)
	}
}

func TestRebalanceControllerRunStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	coordinator := newRebalanceTestCoordinator(t, "node-a", mustRebalanceTestResolver(t), memory.New(), time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32

	controller, err := NewRebalanceController(RebalanceControllerConfig{
		Coordinator:   coordinator,
		Source:        RebalanceDocumentSourceFunc(func(context.Context) ([]storage.DocumentKey, error) { return nil, nil }),
		TargetHolders: []NodeID{"node-a"},
		Interval:      time.Hour,
		OnResult: func(RebalanceControllerRunResult, error) {
			calls.Add(1)
			cancel()
		},
	})
	if err != nil {
		t.Fatalf("NewRebalanceController() unexpected error: %v", err)
	}

	err = controller.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want %v", err, context.Canceled)
	}
	if calls.Load() != 1 {
		t.Fatalf("source calls = %d, want 1", calls.Load())
	}
}

func TestRebalanceControllerValidation(t *testing.T) {
	t.Parallel()

	_, err := NewRebalanceController(RebalanceControllerConfig{})
	if !errors.Is(err, ErrNilOwnershipCoordinator) {
		t.Fatalf("NewRebalanceController(empty) error = %v, want %v", err, ErrNilOwnershipCoordinator)
	}

	coordinator := newRebalanceTestCoordinator(t, "node-a", mustRebalanceTestResolver(t), memory.New(), time.Minute)
	_, err = NewRebalanceController(RebalanceControllerConfig{
		Coordinator:   coordinator,
		Source:        RebalanceDocumentSourceFunc(func(context.Context) ([]storage.DocumentKey, error) { return nil, nil }),
		TargetHolders: []NodeID{"node-a"},
	})
	if !errors.Is(err, ErrInvalidRebalancePlan) {
		t.Fatalf("NewRebalanceController(no interval) error = %v, want %v", err, ErrInvalidRebalancePlan)
	}

	_, err = NewRebalanceController(RebalanceControllerConfig{
		Coordinator:  coordinator,
		Source:       RebalanceDocumentSourceFunc(func(context.Context) ([]storage.DocumentKey, error) { return nil, nil }),
		TargetSource: RebalanceTargetSourceFunc(func(context.Context) ([]NodeID, error) { return nil, nil }),
		Interval:     time.Minute,
	})
	if err != nil {
		t.Fatalf("NewRebalanceController(dynamic targets) unexpected error: %v", err)
	}
}
