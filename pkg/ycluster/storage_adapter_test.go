package ycluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
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
	if lease.ShardID != 7 || lease.Holder != "node-a" || lease.Token != "lease-token" {
		t.Fatalf("lease = %#v, want shard=7 holder=node-a token=lease-token", lease)
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
	if resolution.Placement.Lease == nil || resolution.Placement.Lease.Token != "lease-b" {
		t.Fatalf("resolution.Placement.Lease = %#v, want token lease-b", resolution.Placement.Lease)
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
			Owner:      storage.OwnerInfo{NodeID: storage.NodeID("node-a")},
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
			Owner:      storage.OwnerInfo{NodeID: storage.NodeID("node-a")},
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
}
