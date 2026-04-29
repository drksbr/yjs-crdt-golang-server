package ycluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
)

func TestNewStorageOwnershipCoordinatorValidation(t *testing.T) {
	t.Parallel()

	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		cfg     StorageOwnershipCoordinatorConfig
		wantErr error
	}{
		{
			name:    "missing_node",
			cfg:     StorageOwnershipCoordinatorConfig{Resolver: resolver, Placements: store, Leases: store, TTL: time.Second},
			wantErr: ErrNilLocalNode,
		},
		{
			name:    "missing_resolver",
			cfg:     StorageOwnershipCoordinatorConfig{LocalNode: "node-a", Placements: store, Leases: store, TTL: time.Second},
			wantErr: ErrNilShardResolver,
		},
		{
			name:    "missing_placements",
			cfg:     StorageOwnershipCoordinatorConfig{LocalNode: "node-a", Resolver: resolver, Leases: store, TTL: time.Second},
			wantErr: ErrNilPlacementStore,
		},
		{
			name:    "missing_leases",
			cfg:     StorageOwnershipCoordinatorConfig{LocalNode: "node-a", Resolver: resolver, Placements: store, TTL: time.Second},
			wantErr: ErrNilLeaseStore,
		},
		{
			name:    "invalid_ttl",
			cfg:     StorageOwnershipCoordinatorConfig{LocalNode: "node-a", Resolver: resolver, Placements: store, Leases: store},
			wantErr: ErrInvalidLeaseRequest,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewStorageOwnershipCoordinator(tt.cfg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewStorageOwnershipCoordinator() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStorageOwnershipCoordinatorClaimsDocumentAndResolvesFence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	coordinator, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator() unexpected error: %v", err)
	}

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "coordinator-claim"}
	ownership, err := coordinator.ClaimDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "lease-node-a",
		PlacementVersion: 3,
	})
	if err != nil {
		t.Fatalf("ClaimDocument() unexpected error: %v", err)
	}
	if ownership.DocumentKey != key {
		t.Fatalf("ownership.DocumentKey = %#v, want %#v", ownership.DocumentKey, key)
	}
	if ownership.Placement == nil || ownership.Placement.Version != 3 {
		t.Fatalf("ownership.Placement = %#v, want version 3", ownership.Placement)
	}
	if ownership.Lease == nil || ownership.Lease.Holder != "node-a" || ownership.Lease.Token != "lease-node-a" {
		t.Fatalf("ownership.Lease = %#v, want node-a/lease-node-a", ownership.Lease)
	}

	resolution, err := coordinator.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner() unexpected error: %v", err)
	}
	if !resolution.Local {
		t.Fatal("LookupOwner().Local = false, want true")
	}

	fence, err := coordinator.ResolveAuthorityFence(ctx, key)
	if err != nil {
		t.Fatalf("ResolveAuthorityFence() unexpected error: %v", err)
	}
	if fence.Token != "lease-node-a" || fence.Owner.NodeID != storage.NodeID("node-a") {
		t.Fatalf("ResolveAuthorityFence() = %#v, want node-a/lease-node-a", fence)
	}
}

func TestStorageOwnershipCoordinatorAdoptsExistingLocalLease(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}

	first, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(first) unexpected error: %v", err)
	}
	key := storage.DocumentKey{Namespace: "tests", DocumentID: "coordinator-adopt"}
	claimed, err := first.ClaimDocument(ctx, ClaimDocumentRequest{DocumentKey: key, PlacementVersion: 5})
	if err != nil {
		t.Fatalf("first.ClaimDocument() unexpected error: %v", err)
	}

	second, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(second) unexpected error: %v", err)
	}
	adopted, err := second.ClaimDocument(ctx, ClaimDocumentRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("second.ClaimDocument() unexpected error: %v", err)
	}
	if adopted.Lease.Token != claimed.Lease.Token {
		t.Fatalf("adopted.Lease.Token = %q, want %q", adopted.Lease.Token, claimed.Lease.Token)
	}
	if adopted.Lease.Epoch != claimed.Lease.Epoch {
		t.Fatalf("adopted.Lease.Epoch = %d, want %d", adopted.Lease.Epoch, claimed.Lease.Epoch)
	}
	if adopted.Placement == nil || adopted.Placement.Version != 5 {
		t.Fatalf("adopted.Placement = %#v, want version 5", adopted.Placement)
	}
}

func TestStorageOwnershipCoordinatorDoesNotRewritePlacementWhenForeignLeaseIsHeld(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}

	nodeA, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-a) unexpected error: %v", err)
	}
	key := storage.DocumentKey{Namespace: "tests", DocumentID: "coordinator-held-placement"}
	if _, err := nodeA.ClaimDocument(ctx, ClaimDocumentRequest{DocumentKey: key, PlacementVersion: 7}); err != nil {
		t.Fatalf("nodeA.ClaimDocument() unexpected error: %v", err)
	}

	nodeB, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-b",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-b) unexpected error: %v", err)
	}
	if _, err := nodeB.ClaimDocument(ctx, ClaimDocumentRequest{DocumentKey: key, PlacementVersion: 99}); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("nodeB.ClaimDocument() error = %v, want %v", err, ErrLeaseHeld)
	}

	placement, err := store.LoadPlacement(ctx, key)
	if err != nil {
		t.Fatalf("LoadPlacement() unexpected error: %v", err)
	}
	if placement.Version != 7 {
		t.Fatalf("LoadPlacement().Version = %d, want 7", placement.Version)
	}
}

func TestStorageOwnershipCoordinatorPromotesOnlyWhenRemoteOwnerIsInactive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	now := time.Unix(9000, 0).UTC()
	nowFunc := func() time.Time { return now }

	nodeA, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-a) unexpected error: %v", err)
	}
	nodeA.leases.now = nowFunc
	nodeA.lookup.now = nowFunc

	nodeB, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-b",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-b) unexpected error: %v", err)
	}
	nodeB.leases.now = nowFunc
	nodeB.lookup.now = nowFunc

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "coordinator-promote"}
	remote, err := nodeB.ClaimDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "node-b-token",
		PlacementVersion: 4,
		TTL:              30 * time.Second,
	})
	if err != nil {
		t.Fatalf("nodeB.ClaimDocument() unexpected error: %v", err)
	}

	if _, err := nodeA.PromoteDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "node-a-token",
		PlacementVersion: 5,
	}); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("nodeA.PromoteDocument(active remote) error = %v, want %v", err, ErrLeaseHeld)
	}
	placement, err := store.LoadPlacement(ctx, key)
	if err != nil {
		t.Fatalf("LoadPlacement(active remote) unexpected error: %v", err)
	}
	if placement.Version != 4 {
		t.Fatalf("LoadPlacement(active remote).Version = %d, want 4", placement.Version)
	}

	now = now.Add(31 * time.Second)
	promoted, err := nodeA.PromoteDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "node-a-token",
		PlacementVersion: 5,
	})
	if err != nil {
		t.Fatalf("nodeA.PromoteDocument(expired remote) unexpected error: %v", err)
	}
	if promoted.Lease == nil || promoted.Lease.Holder != "node-a" || promoted.Lease.Epoch != remote.Lease.Epoch+1 {
		t.Fatalf("promoted.Lease = %#v, want node-a epoch %d", promoted.Lease, remote.Lease.Epoch+1)
	}
	if promoted.Placement == nil || promoted.Placement.Version != 5 {
		t.Fatalf("promoted.Placement = %#v, want version 5", promoted.Placement)
	}
}

func TestStorageOwnershipCoordinatorPromoteDocumentClaimsMissingOwner(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	coordinator, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator() unexpected error: %v", err)
	}

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "coordinator-promote-missing"}
	promoted, err := coordinator.PromoteDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "node-a-token",
		PlacementVersion: 2,
	})
	if err != nil {
		t.Fatalf("PromoteDocument(missing owner) unexpected error: %v", err)
	}
	if promoted.Lease == nil || promoted.Lease.Holder != "node-a" || promoted.Lease.Epoch != 1 {
		t.Fatalf("promoted.Lease = %#v, want node-a epoch 1", promoted.Lease)
	}
	if promoted.Placement == nil || promoted.Placement.Version != 2 {
		t.Fatalf("promoted.Placement = %#v, want version 2", promoted.Placement)
	}
}

func TestStorageOwnershipCoordinatorHandoffDocumentTransfersLeaseAtomically(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	now := time.Unix(9700, 0).UTC()
	nowFunc := func() time.Time { return now }

	nodeA, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-a) unexpected error: %v", err)
	}
	nodeA.leases.now = nowFunc
	nodeA.lookup.now = nowFunc

	nodeB, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-b",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-b) unexpected error: %v", err)
	}
	nodeB.leases.now = nowFunc
	nodeB.lookup.now = nowFunc

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "coordinator-handoff"}
	claimed, err := nodeA.ClaimDocument(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "node-a-token",
		PlacementVersion: 11,
		TTL:              30 * time.Second,
	})
	if err != nil {
		t.Fatalf("nodeA.ClaimDocument() unexpected error: %v", err)
	}

	now = now.Add(10 * time.Second)
	handoff, err := nodeB.HandoffDocument(ctx, HandoffDocumentRequest{
		DocumentKey: key,
		Current:     *claimed.Lease,
		NextHolder:  "node-b",
		TTL:         time.Minute,
		Token:       "node-b-token",
	})
	if err != nil {
		t.Fatalf("nodeB.HandoffDocument() unexpected error: %v", err)
	}
	if handoff.Lease == nil || handoff.Lease.Holder != "node-b" || handoff.Lease.Epoch != claimed.Lease.Epoch+1 || handoff.Lease.Token != "node-b-token" {
		t.Fatalf("handoff.Lease = %#v, want node-b epoch %d token node-b-token", handoff.Lease, claimed.Lease.Epoch+1)
	}
	if handoff.Placement == nil || handoff.Placement.Version != 11 {
		t.Fatalf("handoff.Placement = %#v, want version 11", handoff.Placement)
	}
	var managerLease *Lease
	if handoff.Manager != nil {
		managerLease = handoff.Manager.Current()
	}
	if managerLease == nil || managerLease.Token != "node-b-token" {
		t.Fatalf("handoff.Manager.Current() = %#v, want node-b-token lease", managerLease)
	}

	nodeBResolution, err := nodeB.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("nodeB.LookupOwner() unexpected error: %v", err)
	}
	if !nodeBResolution.Local || nodeBResolution.Placement.NodeID != "node-b" {
		t.Fatalf("nodeB.LookupOwner() = %#v, want local node-b", nodeBResolution)
	}

	nodeAResolution, err := nodeA.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("nodeA.LookupOwner() unexpected error: %v", err)
	}
	if nodeAResolution.Local || nodeAResolution.Placement.NodeID != "node-b" {
		t.Fatalf("nodeA.LookupOwner() = %#v, want remote node-b", nodeAResolution)
	}

	fence, err := nodeB.ResolveAuthorityFence(ctx, key)
	if err != nil {
		t.Fatalf("nodeB.ResolveAuthorityFence() unexpected error: %v", err)
	}
	if fence.Owner.NodeID != storage.NodeID("node-b") || fence.Owner.Epoch != handoff.Lease.Epoch || fence.Token != "node-b-token" {
		t.Fatalf("ResolveAuthorityFence() = %#v, want node-b epoch %d token node-b-token", fence, handoff.Lease.Epoch)
	}

	if _, err := nodeB.HandoffDocument(ctx, HandoffDocumentRequest{
		DocumentKey: key,
		Current:     *claimed.Lease,
		NextHolder:  "node-c",
		TTL:         time.Minute,
		Token:       "node-c-token",
	}); !errors.Is(err, ErrLeaseTokenMismatch) {
		t.Fatalf("HandoffDocument(stale current) error = %v, want %v", err, ErrLeaseTokenMismatch)
	}
}

func TestStorageOwnershipCoordinatorBuildsDocumentLeaseManager(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	coordinator, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator() unexpected error: %v", err)
	}
	key := storage.DocumentKey{Namespace: "tests", DocumentID: "coordinator-manager"}
	if _, err := coordinator.ClaimDocument(ctx, ClaimDocumentRequest{DocumentKey: key}); err != nil {
		t.Fatalf("ClaimDocument() unexpected error: %v", err)
	}

	manager, err := coordinator.LeaseManagerForDocument(key, "", time.Minute, "")
	if err != nil {
		t.Fatalf("LeaseManagerForDocument() unexpected error: %v", err)
	}
	lease, changed, err := manager.Ensure(ctx, 10*time.Second)
	if err != nil {
		t.Fatalf("manager.Ensure() unexpected error: %v", err)
	}
	if changed {
		t.Fatal("manager.Ensure().changed = true, want false for adopted active lease")
	}
	if lease == nil || lease.Holder != "node-a" {
		t.Fatalf("manager.Ensure().lease = %#v, want node-a lease", lease)
	}
}

func TestStorageOwnershipCoordinatorRunDocumentOwnershipRenewsAndReleasesOnStop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	coordinator, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator() unexpected error: %v", err)
	}

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "coordinator-run-release"}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	claimedCh := make(chan *DocumentOwnership, 1)
	leaseChanges := make(chan *Lease, 16)
	errCh := make(chan error, 1)
	go func() {
		errCh <- coordinator.RunDocumentOwnership(runCtx, DocumentOwnershipRunConfig{
			Claim: ClaimDocumentRequest{
				DocumentKey:      key,
				Token:            "lease-node-a",
				PlacementVersion: 8,
			},
			Lease: LeaseManagerRunConfig{
				RenewWithin: 1900 * time.Millisecond,
				Interval:    10 * time.Millisecond,
				OnLeaseChange: func(lease *Lease) {
					select {
					case leaseChanges <- lease:
					default:
					}
				},
			},
			ReleaseOnStop:  true,
			ReleaseTimeout: time.Second,
			OnClaimed: func(ownership *DocumentOwnership) {
				claimedCh <- ownership
			},
		})
	}()

	claimed := waitDocumentOwnership(t, claimedCh)
	if claimed.Lease == nil || claimed.Lease.Token != "lease-node-a" || claimed.Placement == nil || claimed.Placement.Version != 8 {
		t.Fatalf("claimed = %#v, want token lease-node-a and placement version 8", claimed)
	}

	renewed := waitRenewedLease(t, leaseChanges, claimed.Lease)
	if renewed.Epoch != claimed.Lease.Epoch {
		t.Fatalf("renewed.Epoch = %d, want %d", renewed.Epoch, claimed.Lease.Epoch)
	}

	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunDocumentOwnership() error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunDocumentOwnership() did not return after cancellation")
	}

	if _, err := coordinator.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrOwnerNotFound) {
		t.Fatalf("LookupOwner(after release) error = %v, want %v", err, ErrOwnerNotFound)
	}
}

func TestStorageOwnershipCoordinatorRunDocumentOwnershipLeavesLeaseWhenReleaseDisabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	coordinator, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator() unexpected error: %v", err)
	}

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "coordinator-run-keep"}
	runCtx, cancel := context.WithCancel(ctx)
	claimedCh := make(chan *DocumentOwnership, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- coordinator.RunDocumentOwnership(runCtx, DocumentOwnershipRunConfig{
			Claim: ClaimDocumentRequest{DocumentKey: key, Token: "keep-lease"},
			Lease: LeaseManagerRunConfig{
				RenewWithin: 10 * time.Second,
				Interval:    10 * time.Millisecond,
			},
			OnClaimed: func(ownership *DocumentOwnership) {
				claimedCh <- ownership
			},
		})
	}()

	claimed := waitDocumentOwnership(t, claimedCh)
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunDocumentOwnership() error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunDocumentOwnership() did not return after cancellation")
	}

	resolution, err := coordinator.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner(after stop without release) unexpected error: %v", err)
	}
	if resolution.Placement.Lease == nil || resolution.Placement.Lease.Token != claimed.Lease.Token {
		t.Fatalf("LookupOwner(after stop without release).Lease = %#v, want token %q", resolution.Placement.Lease, claimed.Lease.Token)
	}
}

func TestStorageOwnershipCoordinatorRunDocumentOwnershipRejectsInvalidReleaseTimeout(t *testing.T) {
	t.Parallel()

	store := memory.New()
	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	coordinator, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator() unexpected error: %v", err)
	}

	err = coordinator.RunDocumentOwnership(context.Background(), DocumentOwnershipRunConfig{
		Claim:          ClaimDocumentRequest{DocumentKey: storage.DocumentKey{Namespace: "tests", DocumentID: "invalid-release-timeout"}},
		ReleaseTimeout: -time.Second,
	})
	if !errors.Is(err, ErrInvalidLeaseRequest) {
		t.Fatalf("RunDocumentOwnership() error = %v, want %v", err, ErrInvalidLeaseRequest)
	}
}

func waitDocumentOwnership(t *testing.T, claims <-chan *DocumentOwnership) *DocumentOwnership {
	t.Helper()

	select {
	case ownership := <-claims:
		if ownership == nil {
			t.Fatal("ownership = nil, want value")
		}
		return ownership
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for document ownership")
		return nil
	}
}

func waitRenewedLease(t *testing.T, changes <-chan *Lease, initial *Lease) *Lease {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case lease := <-changes:
			if lease != nil && initial != nil && lease.Token == initial.Token && lease.ExpiresAt.After(initial.ExpiresAt) {
				return lease
			}
		case <-deadline:
			t.Fatal("timed out waiting for renewed lease")
			return nil
		}
	}
}
