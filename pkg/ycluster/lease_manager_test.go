package ycluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
)

func TestNewLeaseManagerValidation(t *testing.T) {
	t.Parallel()

	store := memory.New()

	tests := []struct {
		name    string
		cfg     LeaseManagerConfig
		wantErr error
	}{
		{
			name:    "missing_store",
			cfg:     LeaseManagerConfig{Holder: "node-a", TTL: time.Second},
			wantErr: ErrNilLeaseStore,
		},
		{
			name:    "missing_holder",
			cfg:     LeaseManagerConfig{Store: mustStorageLeaseStore(t, store), TTL: time.Second},
			wantErr: ErrInvalidLeaseRequest,
		},
		{
			name:    "invalid_ttl",
			cfg:     LeaseManagerConfig{Store: mustStorageLeaseStore(t, store), Holder: "node-a"},
			wantErr: ErrInvalidLeaseRequest,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewLeaseManager(tt.cfg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewLeaseManager() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLeaseManagerEnsureAcquiresCachesAndReturnsCurrentLease(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases := mustStorageLeaseStore(t, store)
	now := time.Unix(1000, 0).UTC()
	leases.now = func() time.Time { return now }

	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   leases,
		ShardID: 7,
		Holder:  "node-a",
		TTL:     30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLeaseManager() unexpected error: %v", err)
	}
	manager.now = leases.now

	first, changed, err := manager.Ensure(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("manager.Ensure(first) unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("manager.Ensure(first).changed = false, want true")
	}
	if first.Epoch != 1 {
		t.Fatalf("manager.Ensure(first).Epoch = %d, want 1", first.Epoch)
	}

	second, changed, err := manager.Ensure(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("manager.Ensure(second) unexpected error: %v", err)
	}
	if changed {
		t.Fatal("manager.Ensure(second).changed = true, want false")
	}
	if second.Token != first.Token || second.Epoch != first.Epoch {
		t.Fatalf("manager.Ensure(second) = %#v, want same token/epoch as first %#v", second, first)
	}

	current := manager.Current()
	if current == nil {
		t.Fatal("manager.Current() = nil, want cached lease")
	}
	current.Token = "mutated"
	if manager.Current().Token != first.Token {
		t.Fatal("manager.Current() returned mutable cache reference")
	}
}

func TestLeaseManagerEnsureRenewsWhenLeaseIsDue(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases := mustStorageLeaseStore(t, store)
	now := time.Unix(2000, 0).UTC()
	leases.now = func() time.Time { return now }

	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   leases,
		ShardID: 9,
		Holder:  "node-a",
		TTL:     30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLeaseManager() unexpected error: %v", err)
	}
	manager.now = leases.now

	first, changed, err := manager.Ensure(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("manager.Ensure(first) unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("manager.Ensure(first).changed = false, want true")
	}

	now = now.Add(25 * time.Second)
	renewed, changed, err := manager.Ensure(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("manager.Ensure(renew) unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("manager.Ensure(renew).changed = false, want true")
	}
	if renewed.Token != first.Token {
		t.Fatalf("renewed.Token = %q, want %q", renewed.Token, first.Token)
	}
	if renewed.Epoch != first.Epoch {
		t.Fatalf("renewed.Epoch = %d, want %d", renewed.Epoch, first.Epoch)
	}
	if !renewed.ExpiresAt.Equal(now.Add(30 * time.Second)) {
		t.Fatalf("renewed.ExpiresAt = %v, want %v", renewed.ExpiresAt, now.Add(30*time.Second))
	}
}

func TestLeaseManagerEnsureReacquiresAfterExpiry(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases := mustStorageLeaseStore(t, store)
	now := time.Unix(3000, 0).UTC()
	leases.now = func() time.Time { return now }

	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   leases,
		ShardID: 11,
		Holder:  "node-a",
		TTL:     20 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLeaseManager() unexpected error: %v", err)
	}
	manager.now = leases.now

	first, _, err := manager.Ensure(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatalf("manager.Ensure(first) unexpected error: %v", err)
	}

	now = now.Add(21 * time.Second)
	second, changed, err := manager.Ensure(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatalf("manager.Ensure(reacquire) unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("manager.Ensure(reacquire).changed = false, want true")
	}
	if second.Epoch != first.Epoch+1 {
		t.Fatalf("second.Epoch = %d, want %d", second.Epoch, first.Epoch+1)
	}
	if second.Token == first.Token {
		t.Fatalf("second.Token = %q, want new token after reacquire", second.Token)
	}
}

func TestLeaseManagerEnsureClearsCacheWhenForeignHolderTakesOver(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leasesA := mustStorageLeaseStore(t, store)
	leasesB := mustStorageLeaseStore(t, store)
	now := time.Unix(4000, 0).UTC()
	leasesA.now = func() time.Time { return now }
	leasesB.now = leasesA.now

	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   leasesA,
		ShardID: 13,
		Holder:  "node-a",
		TTL:     15 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLeaseManager() unexpected error: %v", err)
	}
	manager.now = leasesA.now

	first, _, err := manager.Ensure(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatalf("manager.Ensure(first) unexpected error: %v", err)
	}
	if first == nil {
		t.Fatal("manager.Ensure(first) = nil, want lease")
	}

	now = now.Add(16 * time.Second)
	if _, err := leasesB.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 13,
		Holder:  "node-b",
		TTL:     time.Minute,
	}); err != nil {
		t.Fatalf("leasesB.AcquireLease() unexpected error: %v", err)
	}

	if _, changed, err := manager.Ensure(context.Background(), 5*time.Second); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("manager.Ensure(after takeover) error = %v, want %v", err, ErrLeaseHeld)
	} else if changed {
		t.Fatal("manager.Ensure(after takeover).changed = true, want false")
	}
	if manager.Current() != nil {
		t.Fatal("manager.Current() != nil after foreign takeover, want cleared cache")
	}
}

func TestLeaseManagerRunRenewsUntilContextCancelled(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases := mustStorageLeaseStore(t, store)

	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   leases,
		ShardID: 21,
		Holder:  "node-a",
		TTL:     80 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewLeaseManager() unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := make(chan *Lease, 16)
	errCh := make(chan error, 1)
	go func() {
		errCh <- manager.Run(ctx, LeaseManagerRunConfig{
			RenewWithin: 50 * time.Millisecond,
			Interval:    10 * time.Millisecond,
			OnLeaseChange: func(lease *Lease) {
				select {
				case changes <- lease:
				default:
				}
			},
		})
	}()

	first := waitLeaseChange(t, changes)
	if first.Epoch != 1 {
		t.Fatalf("first.Epoch = %d, want 1", first.Epoch)
	}

	var renewed *Lease
	deadline := time.After(2 * time.Second)
	for renewed == nil {
		select {
		case lease := <-changes:
			if lease.Token == first.Token && lease.ExpiresAt.After(first.ExpiresAt) {
				renewed = lease
			}
		case <-deadline:
			t.Fatal("timed out waiting for lease renewal")
		}
	}
	if renewed.Epoch != first.Epoch {
		t.Fatalf("renewed.Epoch = %d, want %d", renewed.Epoch, first.Epoch)
	}

	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("manager.Run() error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("manager.Run() did not stop after context cancellation")
	}
}

func TestLeaseManagerRunRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	store := memory.New()
	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   mustStorageLeaseStore(t, store),
		ShardID: 23,
		Holder:  "node-a",
		TTL:     time.Second,
	})
	if err != nil {
		t.Fatalf("NewLeaseManager() unexpected error: %v", err)
	}

	tests := []struct {
		name string
		cfg  LeaseManagerRunConfig
	}{
		{name: "negative_renew_within", cfg: LeaseManagerRunConfig{RenewWithin: -time.Second}},
		{name: "negative_interval", cfg: LeaseManagerRunConfig{Interval: -time.Second}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := manager.Run(context.Background(), tt.cfg); !errors.Is(err, ErrInvalidLeaseRequest) {
				t.Fatalf("manager.Run() error = %v, want %v", err, ErrInvalidLeaseRequest)
			}
		})
	}
}

func TestLeaseManagerRelease(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases := mustStorageLeaseStore(t, store)
	now := time.Unix(5000, 0).UTC()
	leases.now = func() time.Time { return now }

	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   leases,
		ShardID: 15,
		Holder:  "node-a",
		TTL:     45 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLeaseManager() unexpected error: %v", err)
	}
	manager.now = leases.now

	if err := manager.Release(context.Background()); !errors.Is(err, ErrOwnerNotFound) {
		t.Fatalf("manager.Release(without lease) error = %v, want %v", err, ErrOwnerNotFound)
	}

	acquired, _, err := manager.Ensure(context.Background(), 10*time.Second)
	if err != nil {
		t.Fatalf("manager.Ensure() unexpected error: %v", err)
	}
	if err := manager.Release(context.Background()); err != nil {
		t.Fatalf("manager.Release() unexpected error: %v", err)
	}
	if manager.Current() != nil {
		t.Fatal("manager.Current() != nil after release, want nil")
	}
	if _, err := store.LoadLease(context.Background(), StorageShardID(acquired.ShardID)); !errors.Is(err, storage.ErrLeaseNotFound) {
		t.Fatalf("store.LoadLease() after release error = %v, want %v", err, storage.ErrLeaseNotFound)
	}
}

func waitLeaseChange(t *testing.T, changes <-chan *Lease) *Lease {
	t.Helper()

	select {
	case lease := <-changes:
		if lease == nil {
			t.Fatal("lease change = nil, want lease")
		}
		return lease
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for lease change")
		return nil
	}
}

func mustStorageLeaseStore(t *testing.T, store storage.LeaseStore) *StorageLeaseStore {
	t.Helper()

	leases, err := NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}
	return leases
}
