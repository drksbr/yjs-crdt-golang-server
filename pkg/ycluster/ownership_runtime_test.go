package ycluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
)

func TestDocumentOwnershipRuntimeSharesDocumentOwnershipUntilLastRelease(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	runtime, coordinator := newTestDocumentOwnershipRuntime(t, store, time.Minute, LeaseManagerRunConfig{
		RenewWithin: 10 * time.Second,
		Interval:    10 * time.Millisecond,
	})

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "runtime-shared"}
	first, err := runtime.AcquireDocumentOwnership(ctx, ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "runtime-shared-token",
		PlacementVersion: 12,
	})
	if err != nil {
		t.Fatalf("AcquireDocumentOwnership(first) unexpected error: %v", err)
	}
	second, err := runtime.AcquireDocumentOwnership(ctx, ClaimDocumentRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("AcquireDocumentOwnership(second) unexpected error: %v", err)
	}

	firstOwnership := first.Ownership()
	secondOwnership := second.Ownership()
	if firstOwnership.Lease == nil || secondOwnership.Lease == nil || firstOwnership.Lease.Token != secondOwnership.Lease.Token {
		t.Fatalf("shared ownership tokens = %#v/%#v, want same lease", firstOwnership.Lease, secondOwnership.Lease)
	}
	if firstOwnership.Placement == nil || firstOwnership.Placement.Version != 12 {
		t.Fatalf("firstOwnership.Placement = %#v, want version 12", firstOwnership.Placement)
	}

	if err := first.Release(ctx); err != nil {
		t.Fatalf("first.Release() unexpected error: %v", err)
	}
	if _, err := coordinator.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key}); err != nil {
		t.Fatalf("LookupOwner(after first release) unexpected error: %v", err)
	}

	if err := second.Release(ctx); err != nil {
		t.Fatalf("second.Release() unexpected error: %v", err)
	}
	if _, err := coordinator.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrOwnerNotFound) {
		t.Fatalf("LookupOwner(after last release) error = %v, want %v", err, ErrOwnerNotFound)
	}
}

func TestDocumentOwnershipRuntimeUpdatesOwnershipOnRenew(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	changes := make(chan *Lease, 16)
	runtime, _ := newTestDocumentOwnershipRuntime(t, store, 80*time.Millisecond, LeaseManagerRunConfig{
		RenewWithin: 60 * time.Millisecond,
		Interval:    10 * time.Millisecond,
		OnLeaseChange: func(lease *Lease) {
			select {
			case changes <- lease:
			default:
			}
		},
	})

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "runtime-renew"}
	handle, err := runtime.AcquireDocumentOwnership(ctx, ClaimDocumentRequest{
		DocumentKey: key,
		Token:       "runtime-renew-token",
	})
	if err != nil {
		t.Fatalf("AcquireDocumentOwnership() unexpected error: %v", err)
	}
	initial := handle.Ownership().Lease
	renewed := waitRenewedLease(t, changes, initial)

	current := handle.Ownership()
	if current.Lease == nil || !current.Lease.ExpiresAt.Equal(renewed.ExpiresAt) {
		t.Fatalf("handle.Ownership().Lease = %#v, want renewed expiry %v", current.Lease, renewed.ExpiresAt)
	}
	if err := handle.Release(ctx); err != nil {
		t.Fatalf("handle.Release() unexpected error: %v", err)
	}
}

func TestDocumentOwnershipRuntimeCloseStopsOwnershipAndRejectsNewAcquire(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	runtime, coordinator := newTestDocumentOwnershipRuntime(t, store, time.Minute, LeaseManagerRunConfig{
		RenewWithin: 10 * time.Second,
		Interval:    10 * time.Millisecond,
	})

	key := storage.DocumentKey{Namespace: "tests", DocumentID: "runtime-close"}
	if _, err := runtime.AcquireDocumentOwnership(ctx, ClaimDocumentRequest{DocumentKey: key}); err != nil {
		t.Fatalf("AcquireDocumentOwnership() unexpected error: %v", err)
	}
	if err := runtime.Close(ctx); err != nil {
		t.Fatalf("runtime.Close() unexpected error: %v", err)
	}
	if _, err := coordinator.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrOwnerNotFound) {
		t.Fatalf("LookupOwner(after close) error = %v, want %v", err, ErrOwnerNotFound)
	}
	if _, err := runtime.AcquireDocumentOwnership(ctx, ClaimDocumentRequest{DocumentKey: key}); !errors.Is(err, ErrOwnershipRuntimeClosed) {
		t.Fatalf("AcquireDocumentOwnership(after close) error = %v, want %v", err, ErrOwnershipRuntimeClosed)
	}
}

func TestNewDocumentOwnershipRuntimeValidation(t *testing.T) {
	t.Parallel()

	if _, err := NewDocumentOwnershipRuntime(DocumentOwnershipRuntimeConfig{}); !errors.Is(err, ErrNilOwnershipCoordinator) {
		t.Fatalf("NewDocumentOwnershipRuntime(empty) error = %v, want %v", err, ErrNilOwnershipCoordinator)
	}

	store := memory.New()
	_, coordinator := newTestDocumentOwnershipRuntime(t, store, time.Minute, LeaseManagerRunConfig{})
	if _, err := NewDocumentOwnershipRuntime(DocumentOwnershipRuntimeConfig{
		Coordinator:    coordinator,
		ReleaseTimeout: -time.Second,
	}); !errors.Is(err, ErrInvalidLeaseRequest) {
		t.Fatalf("NewDocumentOwnershipRuntime(negative release timeout) error = %v, want %v", err, ErrInvalidLeaseRequest)
	}
}

func newTestDocumentOwnershipRuntime(
	t *testing.T,
	store *memory.Store,
	ttl time.Duration,
	lease LeaseManagerRunConfig,
) (*DocumentOwnershipRuntime, *StorageOwnershipCoordinator) {
	t.Helper()

	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	coordinator, err := NewStorageOwnershipCoordinator(StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        ttl,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator() unexpected error: %v", err)
	}
	runtime, err := NewDocumentOwnershipRuntime(DocumentOwnershipRuntimeConfig{
		Coordinator:    coordinator,
		Lease:          lease,
		ReleaseTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewDocumentOwnershipRuntime() unexpected error: %v", err)
	}
	return runtime, coordinator
}
