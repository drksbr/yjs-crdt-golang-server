package ycluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
)

func TestStorageIDConversions(t *testing.T) {
	t.Parallel()

	shardID, err := ParseStorageShardID(storage.ShardID("42"))
	if err != nil {
		t.Fatalf("ParseStorageShardID() unexpected error: %v", err)
	}
	if shardID != 42 {
		t.Fatalf("ParseStorageShardID() = %d, want 42", shardID)
	}
	if StorageShardID(42) != storage.ShardID("42") {
		t.Fatalf("StorageShardID(42) = %q, want %q", StorageShardID(42), storage.ShardID("42"))
	}

	nodeID, err := ParseStorageNodeID(storage.NodeID("node-a"))
	if err != nil {
		t.Fatalf("ParseStorageNodeID() unexpected error: %v", err)
	}
	if nodeID != NodeID("node-a") {
		t.Fatalf("ParseStorageNodeID() = %q, want %q", nodeID, NodeID("node-a"))
	}
	if StorageNodeID(NodeID("node-b")) != storage.NodeID("node-b") {
		t.Fatalf("StorageNodeID() = %q, want %q", StorageNodeID(NodeID("node-b")), storage.NodeID("node-b"))
	}
}

func TestParseStorageShardIDRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	if _, err := ParseStorageShardID(storage.ShardID("shard-a")); !errors.Is(err, ErrInvalidPlacement) {
		t.Fatalf("ParseStorageShardID() error = %v, want %v", err, ErrInvalidPlacement)
	}
}

func TestLeaseFromStorageRecord(t *testing.T) {
	t.Parallel()

	record := &storage.LeaseRecord{
		ShardID:    storage.ShardID("7"),
		Owner:      storage.OwnerInfo{NodeID: storage.NodeID("node-a"), Epoch: 3},
		Token:      "lease-token",
		AcquiredAt: time.Unix(100, 0).UTC(),
		ExpiresAt:  time.Unix(130, 0).UTC(),
	}

	lease, err := LeaseFromStorageRecord(record)
	if err != nil {
		t.Fatalf("LeaseFromStorageRecord() unexpected error: %v", err)
	}
	if lease.ShardID != 7 || lease.Holder != "node-a" || lease.Epoch != 3 || lease.Token != "lease-token" {
		t.Fatalf("lease = %#v, want shard=7 holder=node-a epoch=3 token=lease-token", lease)
	}
}

func TestNewStorageOwnerLookupValidation(t *testing.T) {
	t.Parallel()

	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	store := memory.New()

	tests := []struct {
		name    string
		local   NodeID
		resolve ShardResolver
		place   storage.PlacementStore
		lease   storage.LeaseStore
		wantErr error
	}{
		{name: "missing_local_node", resolve: resolver, place: store, lease: store, wantErr: ErrNilLocalNode},
		{name: "missing_resolver", local: "node-a", place: store, lease: store, wantErr: ErrNilShardResolver},
		{name: "missing_placements", local: "node-a", resolve: resolver, lease: store, wantErr: ErrNilPlacementStore},
		{name: "missing_leases", local: "node-a", resolve: resolver, place: store, wantErr: ErrNilLeaseStore},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStorageOwnerLookup(tt.local, tt.resolve, tt.place, tt.lease)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewStorageOwnerLookup() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStorageOwnerLookup(t *testing.T) {
	t.Parallel()

	store := memory.New()
	resolver, err := NewDeterministicShardResolver(64)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}

	key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}
	if _, err := store.SavePlacement(context.Background(), storage.PlacementRecord{
		Key:     key,
		ShardID: StorageShardID(shardID),
		Version: 4,
	}); err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}
	if _, err := store.SaveLease(context.Background(), storage.LeaseRecord{
		ShardID:    StorageShardID(shardID),
		Owner:      storage.OwnerInfo{NodeID: storage.NodeID("node-b"), Epoch: 9},
		Token:      "lease-b",
		AcquiredAt: time.Unix(100, 0).UTC(),
		ExpiresAt:  time.Unix(130, 0).UTC(),
	}); err != nil {
		t.Fatalf("SaveLease() unexpected error: %v", err)
	}

	lookup, err := NewStorageOwnerLookup("node-a", resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup() unexpected error: %v", err)
	}
	lookup.now = func() time.Time { return time.Unix(110, 0).UTC() }

	resolution, err := lookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner() unexpected error: %v", err)
	}
	if resolution.Local {
		t.Fatal("resolution.Local = true, want false")
	}
	if resolution.Placement.ShardID != shardID || resolution.Placement.NodeID != NodeID("node-b") {
		t.Fatalf("resolution.Placement = %#v, want shard=%d node=node-b", resolution.Placement, shardID)
	}
	if resolution.Placement.Version != 4 {
		t.Fatalf("resolution.Placement.Version = %d, want 4", resolution.Placement.Version)
	}
	if resolution.Placement.Lease == nil || resolution.Placement.Lease.Token != "lease-b" || resolution.Placement.Lease.Epoch != 9 {
		t.Fatalf("resolution.Placement.Lease = %#v, want token=lease-b epoch=9", resolution.Placement.Lease)
	}
}

func TestStorageOwnerLookupErrors(t *testing.T) {
	t.Parallel()

	store := memory.New()
	resolver, err := NewDeterministicShardResolver(64)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	lookup, err := NewStorageOwnerLookup("node-a", resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup() unexpected error: %v", err)
	}
	key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}

	t.Run("missing_placement", func(t *testing.T) {
		_, err := lookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key})
		if !errors.Is(err, ErrPlacementNotFound) {
			t.Fatalf("LookupOwner() error = %v, want %v", err, ErrPlacementNotFound)
		}
	})

	t.Run("missing_lease", func(t *testing.T) {
		if _, err := store.SavePlacement(context.Background(), storage.PlacementRecord{
			Key:     key,
			ShardID: StorageShardID(shardID),
			Version: 1,
		}); err != nil {
			t.Fatalf("SavePlacement() unexpected error: %v", err)
		}

		_, err := lookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key})
		if !errors.Is(err, ErrOwnerNotFound) {
			t.Fatalf("LookupOwner() error = %v, want %v", err, ErrOwnerNotFound)
		}
	})

	t.Run("placement_shard_mismatch", func(t *testing.T) {
		if _, err := store.SavePlacement(context.Background(), storage.PlacementRecord{
			Key:     key,
			ShardID: storage.ShardID("999"),
			Version: 2,
		}); err != nil {
			t.Fatalf("SavePlacement() unexpected error: %v", err)
		}
		if _, err := store.SaveLease(context.Background(), storage.LeaseRecord{
			ShardID:    storage.ShardID("999"),
			Owner:      storage.OwnerInfo{NodeID: storage.NodeID("node-a"), Epoch: 1},
			Token:      "lease-a",
			AcquiredAt: time.Unix(120, 0).UTC(),
			ExpiresAt:  time.Unix(130, 0).UTC(),
		}); err != nil {
			t.Fatalf("SaveLease() unexpected error: %v", err)
		}

		_, err := lookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key})
		if !errors.Is(err, ErrInvalidPlacement) {
			t.Fatalf("LookupOwner() error = %v, want %v", err, ErrInvalidPlacement)
		}
	})

	t.Run("expired_lease", func(t *testing.T) {
		if _, err := store.SavePlacement(context.Background(), storage.PlacementRecord{
			Key:     key,
			ShardID: StorageShardID(shardID),
			Version: 3,
		}); err != nil {
			t.Fatalf("SavePlacement() unexpected error: %v", err)
		}
		if _, err := store.SaveLease(context.Background(), storage.LeaseRecord{
			ShardID:    StorageShardID(shardID),
			Owner:      storage.OwnerInfo{NodeID: storage.NodeID("node-a"), Epoch: 1},
			Token:      "lease-expired",
			AcquiredAt: time.Unix(120, 0).UTC(),
			ExpiresAt:  time.Unix(130, 0).UTC(),
		}); err != nil {
			t.Fatalf("SaveLease() unexpected error: %v", err)
		}
		lookup.now = func() time.Time { return time.Unix(131, 0).UTC() }

		_, err := lookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key})
		if !errors.Is(err, ErrLeaseExpired) {
			t.Fatalf("LookupOwner() error = %v, want %v", err, ErrLeaseExpired)
		}
	})

	t.Run("invalid_lease_epoch", func(t *testing.T) {
		invalidLeaseStore := storageLeaseStoreStub{
			loadLease: func(context.Context, storage.ShardID) (*storage.LeaseRecord, error) {
				return &storage.LeaseRecord{
					ShardID:    StorageShardID(shardID),
					Owner:      storage.OwnerInfo{NodeID: storage.NodeID("node-a")},
					Token:      "lease-no-epoch",
					AcquiredAt: time.Unix(120, 0).UTC(),
					ExpiresAt:  time.Unix(130, 0).UTC(),
				}, nil
			},
		}
		lookup, err := NewStorageOwnerLookup("node-a", resolver, store, invalidLeaseStore)
		if err != nil {
			t.Fatalf("NewStorageOwnerLookup() unexpected error: %v", err)
		}
		if _, err := store.SavePlacement(context.Background(), storage.PlacementRecord{
			Key:     key,
			ShardID: StorageShardID(shardID),
			Version: 4,
		}); err != nil {
			t.Fatalf("SavePlacement() unexpected error: %v", err)
		}
		lookup.now = func() time.Time { return time.Unix(121, 0).UTC() }

		_, err = lookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key})
		if !errors.Is(err, ErrInvalidLease) {
			t.Fatalf("LookupOwner() error = %v, want %v", err, ErrInvalidLease)
		}
	})
}

func TestStorageLeaseStore(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases, err := NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	now := time.Unix(500, 0).UTC()
	leases.now = func() time.Time { return now }

	acquired, err := leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 7,
		Holder:  "node-a",
		TTL:     30 * time.Second,
	})
	if err != nil {
		t.Fatalf("AcquireLease() unexpected error: %v", err)
	}
	if acquired.Token == "" {
		t.Fatal("AcquireLease().Token is empty")
	}
	if acquired.Epoch != 1 {
		t.Fatalf("AcquireLease().Epoch = %d, want 1", acquired.Epoch)
	}
	if !acquired.ExpiresAt.Equal(now.Add(30 * time.Second)) {
		t.Fatalf("AcquireLease().ExpiresAt = %v, want %v", acquired.ExpiresAt, now.Add(30*time.Second))
	}

	now = now.Add(10 * time.Second)
	renewed, err := leases.RenewLease(context.Background(), LeaseRequest{
		ShardID: 7,
		Holder:  "node-a",
		TTL:     time.Minute,
	})
	if err != nil {
		t.Fatalf("RenewLease() unexpected error: %v", err)
	}
	if renewed.Token != acquired.Token {
		t.Fatalf("RenewLease().Token = %q, want %q", renewed.Token, acquired.Token)
	}
	if renewed.Epoch != acquired.Epoch {
		t.Fatalf("RenewLease().Epoch = %d, want %d", renewed.Epoch, acquired.Epoch)
	}
	if !renewed.AcquiredAt.Equal(acquired.AcquiredAt) {
		t.Fatalf("RenewLease().AcquiredAt = %v, want %v", renewed.AcquiredAt, acquired.AcquiredAt)
	}
	if !renewed.ExpiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("RenewLease().ExpiresAt = %v, want %v", renewed.ExpiresAt, now.Add(time.Minute))
	}

	if err := leases.ReleaseLease(context.Background(), *renewed); err != nil {
		t.Fatalf("ReleaseLease() unexpected error: %v", err)
	}
	if _, err := store.LoadLease(context.Background(), StorageShardID(renewed.ShardID)); !errors.Is(err, storage.ErrLeaseNotFound) {
		t.Fatalf("LoadLease() after release error = %v, want %v", err, storage.ErrLeaseNotFound)
	}
}

func TestStorageLeaseStoreRejectsAcquireWhenForeignHolderIsActive(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases, err := NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	now := time.Unix(700, 0).UTC()
	leases.now = func() time.Time { return now }

	acquired, err := leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 7,
		Holder:  "node-a",
		TTL:     time.Minute,
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-a) unexpected error: %v", err)
	}

	if _, err := leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 7,
		Holder:  "node-b",
		TTL:     time.Minute,
	}); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("AcquireLease(node-b) error = %v, want %v", err, ErrLeaseHeld)
	}

	stillHeld, err := store.LoadLease(context.Background(), StorageShardID(7))
	if err != nil {
		t.Fatalf("LoadLease() unexpected error: %v", err)
	}
	if stillHeld.Owner.NodeID != storage.NodeID("node-a") || stillHeld.Owner.Epoch != 1 || stillHeld.Token != acquired.Token {
		t.Fatalf("LoadLease() = %#v, want owner=node-a epoch=1 token=%q", stillHeld, acquired.Token)
	}
}

func TestStorageLeaseStoreExpiredTakeoverIncrementsEpoch(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases, err := NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	now := time.Unix(900, 0).UTC()
	leases.now = func() time.Time { return now }

	acquired, err := leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 7,
		Holder:  "node-a",
		TTL:     30 * time.Second,
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-a) unexpected error: %v", err)
	}

	now = now.Add(31 * time.Second)
	takeover, err := leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 7,
		Holder:  "node-b",
		TTL:     45 * time.Second,
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-b takeover) unexpected error: %v", err)
	}
	if takeover.Holder != "node-b" {
		t.Fatalf("AcquireLease(node-b takeover).Holder = %q, want %q", takeover.Holder, "node-b")
	}
	if takeover.Epoch != acquired.Epoch+1 {
		t.Fatalf("AcquireLease(node-b takeover).Epoch = %d, want %d", takeover.Epoch, acquired.Epoch+1)
	}
	if !takeover.AcquiredAt.Equal(now) {
		t.Fatalf("AcquireLease(node-b takeover).AcquiredAt = %v, want %v", takeover.AcquiredAt, now)
	}
	if !takeover.ExpiresAt.Equal(now.Add(45 * time.Second)) {
		t.Fatalf("AcquireLease(node-b takeover).ExpiresAt = %v, want %v", takeover.ExpiresAt, now.Add(45*time.Second))
	}

	stored, err := store.LoadLease(context.Background(), StorageShardID(7))
	if err != nil {
		t.Fatalf("LoadLease() unexpected error: %v", err)
	}
	if stored.Owner.NodeID != storage.NodeID("node-b") || stored.Owner.Epoch != 2 {
		t.Fatalf("LoadLease() = %#v, want owner=node-b epoch=2", stored)
	}
}

func TestStorageLeaseStoreReacquireAfterReleaseIncrementsStoredGeneration(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases, err := NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	now := time.Unix(1100, 0).UTC()
	leases.now = func() time.Time { return now }

	acquired, err := leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 11,
		Holder:  "node-a",
		TTL:     time.Minute,
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-a) unexpected error: %v", err)
	}
	if err := leases.ReleaseLease(context.Background(), *acquired); err != nil {
		t.Fatalf("ReleaseLease(node-a) unexpected error: %v", err)
	}

	now = now.Add(time.Second)
	reacquired, err := leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 11,
		Holder:  "node-b",
		TTL:     time.Minute,
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-b after release) unexpected error: %v", err)
	}
	if reacquired.Epoch != acquired.Epoch+1 {
		t.Fatalf("AcquireLease(node-b after release).Epoch = %d, want %d", reacquired.Epoch, acquired.Epoch+1)
	}
	if reacquired.Holder != "node-b" {
		t.Fatalf("AcquireLease(node-b after release).Holder = %q, want %q", reacquired.Holder, "node-b")
	}
}

func TestStorageLeaseStoreHandoffLeaseTransfersActiveLease(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases, err := NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	now := time.Unix(1300, 0).UTC()
	leases.now = func() time.Time { return now }

	acquired, err := leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 7,
		Holder:  "node-a",
		TTL:     30 * time.Second,
		Token:   "lease-a",
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-a) unexpected error: %v", err)
	}

	now = now.Add(10 * time.Second)
	handoff, err := leases.HandoffLease(context.Background(), *acquired, LeaseRequest{
		Holder: "node-b",
		TTL:    45 * time.Second,
		Token:  "lease-b",
	})
	if err != nil {
		t.Fatalf("HandoffLease(node-b) unexpected error: %v", err)
	}
	if handoff.Holder != "node-b" || handoff.Epoch != acquired.Epoch+1 || handoff.Token != "lease-b" {
		t.Fatalf("HandoffLease(node-b) = %#v, want node-b epoch %d token lease-b", handoff, acquired.Epoch+1)
	}
	if !handoff.AcquiredAt.Equal(now) {
		t.Fatalf("HandoffLease(node-b).AcquiredAt = %v, want %v", handoff.AcquiredAt, now)
	}
	if !handoff.ExpiresAt.Equal(now.Add(45 * time.Second)) {
		t.Fatalf("HandoffLease(node-b).ExpiresAt = %v, want %v", handoff.ExpiresAt, now.Add(45*time.Second))
	}

	stored, err := store.LoadLease(context.Background(), StorageShardID(7))
	if err != nil {
		t.Fatalf("LoadLease() unexpected error: %v", err)
	}
	if stored.Owner.NodeID != storage.NodeID("node-b") || stored.Owner.Epoch != 2 || stored.Token != "lease-b" {
		t.Fatalf("LoadLease() = %#v, want owner=node-b epoch=2 token=lease-b", stored)
	}

	if _, err := leases.HandoffLease(context.Background(), *acquired, LeaseRequest{
		Holder: "node-c",
		TTL:    time.Minute,
		Token:  "lease-c",
	}); !errors.Is(err, ErrLeaseTokenMismatch) {
		t.Fatalf("HandoffLease(stale current) error = %v, want %v", err, ErrLeaseTokenMismatch)
	}
}

func TestStorageLeaseStoreMapsStorageLeaseConflictToHeld(t *testing.T) {
	t.Parallel()

	store := storageLeaseStoreStub{
		loadLease: func(context.Context, storage.ShardID) (*storage.LeaseRecord, error) {
			return nil, storage.ErrLeaseNotFound
		},
		saveLease: func(context.Context, storage.LeaseRecord) (*storage.LeaseRecord, error) {
			return nil, storage.ErrLeaseConflict
		},
	}
	leases, err := NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	_, err = leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: 13,
		Holder:  "node-a",
		TTL:     time.Minute,
	})
	if !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("AcquireLease() error = %v, want %v", err, ErrLeaseHeld)
	}
}

func TestStorageLeaseStoreErrors(t *testing.T) {
	t.Parallel()

	if _, err := NewStorageLeaseStore(nil); !errors.Is(err, ErrNilLeaseStore) {
		t.Fatalf("NewStorageLeaseStore(nil) error = %v, want %v", err, ErrNilLeaseStore)
	}

	store := memory.New()
	leases, err := NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	if _, err := leases.RenewLease(context.Background(), LeaseRequest{ShardID: 7, Holder: "node-a", TTL: time.Second}); !errors.Is(err, ErrOwnerNotFound) {
		t.Fatalf("RenewLease() error = %v, want %v", err, ErrOwnerNotFound)
	}
	if err := leases.ReleaseLease(context.Background(), Lease{}); !errors.Is(err, ErrInvalidLease) {
		t.Fatalf("ReleaseLease() error = %v, want %v", err, ErrInvalidLease)
	}

	unsupportedLeases, err := NewStorageLeaseStore(storageLeaseStoreStub{})
	if err != nil {
		t.Fatalf("NewStorageLeaseStore(unsupported) unexpected error: %v", err)
	}
	current := Lease{
		ShardID:   7,
		Holder:    "node-a",
		Epoch:     1,
		Token:     "lease-a",
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	if _, err := unsupportedLeases.HandoffLease(context.Background(), current, LeaseRequest{
		Holder: "node-b",
		TTL:    time.Minute,
		Token:  "lease-b",
	}); !errors.Is(err, ErrLeaseHandoffUnsupported) {
		t.Fatalf("HandoffLease(unsupported) error = %v, want %v", err, ErrLeaseHandoffUnsupported)
	}
}

type storageLeaseStoreStub struct {
	saveLease    func(ctx context.Context, lease storage.LeaseRecord) (*storage.LeaseRecord, error)
	loadLease    func(ctx context.Context, shardID storage.ShardID) (*storage.LeaseRecord, error)
	releaseLease func(ctx context.Context, shardID storage.ShardID, token string) error
}

func (s storageLeaseStoreStub) SaveLease(ctx context.Context, lease storage.LeaseRecord) (*storage.LeaseRecord, error) {
	if s.saveLease == nil {
		return nil, storage.ErrLeaseNotFound
	}
	return s.saveLease(ctx, lease)
}

func (s storageLeaseStoreStub) LoadLease(ctx context.Context, shardID storage.ShardID) (*storage.LeaseRecord, error) {
	if s.loadLease == nil {
		return nil, storage.ErrLeaseNotFound
	}
	return s.loadLease(ctx, shardID)
}

func (s storageLeaseStoreStub) ReleaseLease(ctx context.Context, shardID storage.ShardID, token string) error {
	if s.releaseLease == nil {
		return storage.ErrLeaseNotFound
	}
	return s.releaseLease(ctx, shardID, token)
}
