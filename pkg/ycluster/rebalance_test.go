package ycluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
)

func TestStorageOwnershipCoordinatorRebalanceDocumentHandsOffActiveOwner(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	now := time.Unix(12000, 0).UTC()
	nowFunc := func() time.Time { return now }

	nodeA := newRebalanceTestCoordinator(t, "node-a", resolver, store, time.Minute)
	nodeA.leases.now = nowFunc
	nodeA.lookup.now = nowFunc
	nodeB := newRebalanceTestCoordinator(t, "node-b", resolver, store, time.Minute)
	nodeB.leases.now = nowFunc
	nodeB.lookup.now = nowFunc

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "rebalance-handoff"}
	claimed, err := nodeA.ClaimDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "node-a-token",
		PlacementVersion: 12,
		TTL:              30 * time.Second,
	})
	if err != nil {
		t.Fatalf("nodeA.ClaimDocument() unexpected error: %v", err)
	}

	now = now.Add(10 * time.Second)
	result, err := nodeB.RebalanceDocument(ctx, RebalanceDocumentRequest{
		DocumentKey:  key,
		TargetHolder: "node-b",
		TTL:          time.Minute,
		Token:        "node-b-token",
	})
	if err != nil {
		t.Fatalf("nodeB.RebalanceDocument() unexpected error: %v", err)
	}
	if !result.Changed {
		t.Fatal("RebalanceDocument().Changed = false, want true")
	}
	if result.From != "node-a" || result.To != "node-b" {
		t.Fatalf("RebalanceDocument() moved %q -> %q, want node-a -> node-b", result.From, result.To)
	}
	if result.PromotedFromLost {
		t.Fatal("RebalanceDocument().PromotedFromLost = true, want false")
	}
	if result.Previous == nil || result.Previous.Placement.NodeID != "node-a" {
		t.Fatalf("RebalanceDocument().Previous = %#v, want node-a", result.Previous)
	}
	if result.Ownership == nil || result.Ownership.Lease == nil {
		t.Fatalf("RebalanceDocument().Ownership = %#v, want lease", result.Ownership)
	}
	if result.Ownership.Lease.Holder != "node-b" ||
		result.Ownership.Lease.Epoch != claimed.Lease.Epoch+1 ||
		result.Ownership.Lease.Token != "node-b-token" {
		t.Fatalf("RebalanceDocument().Ownership.Lease = %#v, want node-b epoch %d token node-b-token", result.Ownership.Lease, claimed.Lease.Epoch+1)
	}
	if result.Ownership.Placement == nil || result.Ownership.Placement.Version != 12 {
		t.Fatalf("RebalanceDocument().Ownership.Placement = %#v, want version 12", result.Ownership.Placement)
	}
	if current := result.Ownership.Manager.Current(); current == nil || current.Token != "node-b-token" {
		t.Fatalf("RebalanceDocument().Ownership.Manager.Current() = %#v, want node-b-token", current)
	}

	resolution, err := nodeA.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("nodeA.LookupOwner(after rebalance) unexpected error: %v", err)
	}
	if resolution.Local || resolution.Placement.NodeID != "node-b" {
		t.Fatalf("nodeA.LookupOwner(after rebalance) = %#v, want remote node-b", resolution)
	}
}

func TestStorageOwnershipCoordinatorRebalanceDocumentNoopsWhenTargetAlreadyOwns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	nodeA := newRebalanceTestCoordinator(t, "node-a", resolver, store, time.Minute)

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "rebalance-noop"}
	claimed, err := nodeA.ClaimDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "node-a-token",
		PlacementVersion: 14,
	})
	if err != nil {
		t.Fatalf("nodeA.ClaimDocument() unexpected error: %v", err)
	}

	result, err := nodeA.RebalanceDocument(ctx, RebalanceDocumentRequest{
		DocumentKey:  key,
		TargetHolder: "node-a",
		TTL:          time.Minute,
		Token:        "ignored-token",
	})
	if err != nil {
		t.Fatalf("nodeA.RebalanceDocument() unexpected error: %v", err)
	}
	if result.Changed {
		t.Fatal("RebalanceDocument().Changed = true, want false")
	}
	if result.From != "node-a" || result.To != "node-a" {
		t.Fatalf("RebalanceDocument() moved %q -> %q, want node-a -> node-a", result.From, result.To)
	}
	if result.Ownership == nil || result.Ownership.Lease == nil {
		t.Fatalf("RebalanceDocument().Ownership = %#v, want lease", result.Ownership)
	}
	if result.Ownership.Lease.Epoch != claimed.Lease.Epoch || result.Ownership.Lease.Token != claimed.Lease.Token {
		t.Fatalf("RebalanceDocument().Ownership.Lease = %#v, want original lease %#v", result.Ownership.Lease, claimed.Lease)
	}
	if result.Ownership.Placement == nil || result.Ownership.Placement.Version != 14 {
		t.Fatalf("RebalanceDocument().Ownership.Placement = %#v, want version 14", result.Ownership.Placement)
	}
}

func TestStorageOwnershipCoordinatorRebalanceDocumentRequiresOwnerInStrictMode(t *testing.T) {
	t.Parallel()

	coordinator := newRebalanceTestCoordinator(t, "node-a", mustRebalanceTestResolver(t), memory.New(), time.Minute)
	key := storage.DocumentKey{Namespace: "tests", DocumentID: "rebalance-strict-missing"}

	_, err := coordinator.RebalanceDocument(context.Background(), RebalanceDocumentRequest{
		DocumentKey:  key,
		TargetHolder: "node-a",
	})
	if !errors.Is(err, ErrOwnerNotFound) {
		t.Fatalf("RebalanceDocument(strict missing) error = %v, want %v", err, ErrOwnerNotFound)
	}
}

func TestStorageOwnershipCoordinatorRebalanceDocumentPromotesMissingOwnerWhenAllowed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver := mustRebalanceTestResolver(t)
	nodeB := newRebalanceTestCoordinator(t, "node-b", resolver, store, time.Minute)
	key := storage.DocumentKey{Namespace: "tests", DocumentID: "rebalance-promote-missing"}

	result, err := nodeB.RebalanceDocument(ctx, RebalanceDocumentRequest{
		DocumentKey:           key,
		TargetHolder:          "node-b",
		Token:                 "node-b-token",
		PromoteIfOwnerMissing: true,
		PlacementVersion:      9,
	})
	if err != nil {
		t.Fatalf("RebalanceDocument(promote missing) unexpected error: %v", err)
	}
	if !result.Changed || !result.PromotedFromLost {
		t.Fatalf("RebalanceDocument(promote missing) Changed/PromotedFromLost = %v/%v, want true/true", result.Changed, result.PromotedFromLost)
	}
	if result.From != "" || result.To != "node-b" {
		t.Fatalf("RebalanceDocument(promote missing) moved %q -> %q, want empty -> node-b", result.From, result.To)
	}
	if result.Ownership == nil || result.Ownership.Lease == nil || result.Ownership.Lease.Holder != "node-b" || result.Ownership.Lease.Epoch != 1 {
		t.Fatalf("RebalanceDocument(promote missing).Ownership = %#v, want node-b epoch 1", result.Ownership)
	}
	if result.Ownership.Placement == nil || result.Ownership.Placement.Version != 9 {
		t.Fatalf("RebalanceDocument(promote missing).Placement = %#v, want version 9", result.Ownership.Placement)
	}
}

func TestStorageOwnershipCoordinatorRebalanceDocumentPromotesExpiredOwnerWhenAllowed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver := mustRebalanceTestResolver(t)
	now := time.Unix(13000, 0).UTC()
	nowFunc := func() time.Time { return now }

	nodeA := newRebalanceTestCoordinator(t, "node-a", resolver, store, time.Minute)
	nodeA.leases.now = nowFunc
	nodeA.lookup.now = nowFunc
	nodeB := newRebalanceTestCoordinator(t, "node-b", resolver, store, time.Minute)
	nodeB.leases.now = nowFunc
	nodeB.lookup.now = nowFunc

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "rebalance-promote-expired"}
	claimed, err := nodeA.ClaimDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "node-a-token",
		PlacementVersion: 21,
		TTL:              10 * time.Second,
	})
	if err != nil {
		t.Fatalf("nodeA.ClaimDocument() unexpected error: %v", err)
	}

	now = now.Add(11 * time.Second)
	result, err := nodeB.RebalanceDocument(ctx, RebalanceDocumentRequest{
		DocumentKey:           key,
		TargetHolder:          "node-b",
		Token:                 "node-b-token",
		PromoteIfOwnerMissing: true,
	})
	if err != nil {
		t.Fatalf("RebalanceDocument(promote expired) unexpected error: %v", err)
	}
	if !result.Changed || !result.PromotedFromLost {
		t.Fatalf("RebalanceDocument(promote expired) Changed/PromotedFromLost = %v/%v, want true/true", result.Changed, result.PromotedFromLost)
	}
	if result.Ownership == nil || result.Ownership.Lease == nil {
		t.Fatalf("RebalanceDocument(promote expired).Ownership = %#v, want lease", result.Ownership)
	}
	if result.Ownership.Lease.Holder != "node-b" || result.Ownership.Lease.Epoch != claimed.Lease.Epoch+1 {
		t.Fatalf("RebalanceDocument(promote expired).Lease = %#v, want node-b epoch %d", result.Ownership.Lease, claimed.Lease.Epoch+1)
	}
	if result.Ownership.Placement == nil || result.Ownership.Placement.Version != 21 {
		t.Fatalf("RebalanceDocument(promote expired).Placement = %#v, want version 21", result.Ownership.Placement)
	}
}

func mustRebalanceTestResolver(t *testing.T) ShardResolver {
	t.Helper()

	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	return resolver
}

func newRebalanceTestCoordinator(
	t *testing.T,
	node NodeID,
	resolver ShardResolver,
	store *memory.Store,
	ttl time.Duration,
) *StorageOwnershipCoordinator {
	t.Helper()

	coordinator, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  node,
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        ttl,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(%s) unexpected error: %v", node, err)
	}
	return coordinator
}
