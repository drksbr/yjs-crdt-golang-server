package ycluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"yjs-go-bridge/pkg/storage"
)

func TestStaticLocalNodeValidate(t *testing.T) {
	t.Parallel()

	if err := (StaticLocalNode{ID: "node-a"}).Validate(); err != nil {
		t.Fatalf("StaticLocalNode.Validate() unexpected error: %v", err)
	}
	if err := (StaticLocalNode{}).Validate(); !errors.Is(err, ErrNilLocalNode) {
		t.Fatalf("StaticLocalNode.Validate() error = %v, want %v", err, ErrNilLocalNode)
	}
}

func TestNewPlacementOwnerLookup(t *testing.T) {
	t.Parallel()

	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	store := placementStoreStub{}

	tests := []struct {
		name      string
		localNode NodeID
		resolver  ShardResolver
		store     PlacementStore
		wantErr   error
	}{
		{name: "valid", localNode: "node-a", resolver: resolver, store: store, wantErr: nil},
		{name: "invalid_local_node", localNode: "", resolver: resolver, store: store, wantErr: ErrNilLocalNode},
		{name: "nil_resolver", localNode: "node-a", resolver: nil, store: store, wantErr: ErrNilShardResolver},
		{name: "nil_store", localNode: "node-a", resolver: resolver, store: nil, wantErr: ErrNilPlacementStore},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lookup, err := NewPlacementOwnerLookup(tt.localNode, tt.resolver, tt.store)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewPlacementOwnerLookup() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && lookup == nil {
				t.Fatal("NewPlacementOwnerLookup() = nil, want non-nil")
			}
		})
	}
}

func TestPlacementOwnerLookupLookupOwner(t *testing.T) {
	t.Parallel()

	key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"}
	resolver, err := NewDeterministicShardResolver(64)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("resolver.ResolveShard() unexpected error: %v", err)
	}

	lookup, err := NewPlacementOwnerLookup("node-a", resolver, placementStoreStub{
		loadPlacement: func(ctx context.Context, incoming ShardID) (*Placement, error) {
			if ctx == nil {
				t.Fatal("LoadPlacement() received nil context")
			}
			if incoming != shardID {
				t.Fatalf("LoadPlacement() shard = %v, want %v", incoming, shardID)
			}
			return &Placement{
				ShardID: shardID,
				NodeID:  "node-a",
				Lease: &Lease{
					ShardID:    shardID,
					Holder:     "node-a",
					Epoch:      4,
					Token:      "lease-node-a",
					AcquiredAt: time.Unix(100, 0).UTC(),
					ExpiresAt:  time.Unix(130, 0).UTC(),
				},
				Version: 3,
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewPlacementOwnerLookup() unexpected error: %v", err)
	}
	lookup.now = func() time.Time { return time.Unix(110, 0).UTC() }

	resolution, err := lookup.LookupOwner(nil, OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner() unexpected error: %v", err)
	}
	if resolution == nil {
		t.Fatal("LookupOwner() = nil, want non-nil")
	}
	if !resolution.Local {
		t.Fatal("LookupOwner().Local = false, want true")
	}
	if resolution.Placement.NodeID != "node-a" {
		t.Fatalf("LookupOwner().Placement.NodeID = %q, want %q", resolution.Placement.NodeID, "node-a")
	}
	if resolution.Placement.Lease == nil || resolution.Placement.Lease.Epoch != 4 {
		t.Fatalf("LookupOwner().Placement.Lease = %#v, want epoch 4", resolution.Placement.Lease)
	}
}

func TestPlacementOwnerLookupLookupOwnerErrors(t *testing.T) {
	t.Parallel()

	key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"}
	resolver, err := NewDeterministicShardResolver(64)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("resolver.ResolveShard() unexpected error: %v", err)
	}

	lookup, err := NewPlacementOwnerLookup("node-a", resolver, placementStoreStub{
		loadPlacement: func(context.Context, ShardID) (*Placement, error) {
			return nil, ErrPlacementNotFound
		},
	})
	if err != nil {
		t.Fatalf("NewPlacementOwnerLookup() unexpected error: %v", err)
	}
	if _, err := lookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrPlacementNotFound) {
		t.Fatalf("LookupOwner() error = %v, want %v", err, ErrPlacementNotFound)
	}

	missingLeaseLookup, err := NewPlacementOwnerLookup("node-a", resolver, placementStoreStub{
		loadPlacement: func(context.Context, ShardID) (*Placement, error) {
			return &Placement{ShardID: shardID, NodeID: "node-a", Version: 1}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewPlacementOwnerLookup() unexpected error: %v", err)
	}
	if _, err := missingLeaseLookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrOwnerNotFound) {
		t.Fatalf("LookupOwner() missing lease error = %v, want %v", err, ErrOwnerNotFound)
	}

	mismatchLookup, err := NewPlacementOwnerLookup("node-a", resolver, placementStoreStub{
		loadPlacement: func(context.Context, ShardID) (*Placement, error) {
			return &Placement{ShardID: shardID + 1, NodeID: "node-b", Version: 1}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewPlacementOwnerLookup() unexpected error: %v", err)
	}
	if _, err := mismatchLookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrInvalidPlacement) {
		t.Fatalf("LookupOwner() mismatch error = %v, want %v", err, ErrInvalidPlacement)
	}

	invalidLeaseLookup, err := NewPlacementOwnerLookup("node-a", resolver, placementStoreStub{
		loadPlacement: func(context.Context, ShardID) (*Placement, error) {
			return &Placement{
				ShardID: shardID,
				NodeID:  "node-a",
				Lease: &Lease{
					ShardID:    shardID,
					Holder:     "node-a",
					Token:      "lease-node-a",
					AcquiredAt: time.Unix(100, 0).UTC(),
					ExpiresAt:  time.Unix(130, 0).UTC(),
				},
				Version: 1,
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewPlacementOwnerLookup() unexpected error: %v", err)
	}
	if _, err := invalidLeaseLookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrInvalidPlacement) {
		t.Fatalf("LookupOwner() invalid lease error = %v, want %v", err, ErrInvalidPlacement)
	}

	expiredLeaseLookup, err := NewPlacementOwnerLookup("node-a", resolver, placementStoreStub{
		loadPlacement: func(context.Context, ShardID) (*Placement, error) {
			return &Placement{
				ShardID: shardID,
				NodeID:  "node-a",
				Lease: &Lease{
					ShardID:    shardID,
					Holder:     "node-a",
					Epoch:      2,
					Token:      "lease-node-a",
					AcquiredAt: time.Unix(100, 0).UTC(),
					ExpiresAt:  time.Unix(130, 0).UTC(),
				},
				Version: 1,
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewPlacementOwnerLookup() unexpected error: %v", err)
	}
	expiredLeaseLookup.now = func() time.Time { return time.Unix(131, 0).UTC() }
	if _, err := expiredLeaseLookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("LookupOwner() expired lease error = %v, want %v", err, ErrLeaseExpired)
	}

	if _, err := lookup.LookupOwner(context.Background(), OwnerLookupRequest{}); !errors.Is(err, ErrInvalidOwnerLookupRequest) {
		t.Fatalf("LookupOwner() invalid request error = %v, want %v", err, ErrInvalidOwnerLookupRequest)
	}

	var nilLookup *PlacementOwnerLookup
	if _, err := nilLookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrOwnerNotFound) {
		t.Fatalf("nil lookup error = %v, want %v", err, ErrOwnerNotFound)
	}
}

type placementStoreStub struct {
	loadPlacement func(ctx context.Context, shardID ShardID) (*Placement, error)
}

func (s placementStoreStub) SavePlacement(context.Context, Placement) error {
	return nil
}

func (s placementStoreStub) LoadPlacement(ctx context.Context, shardID ShardID) (*Placement, error) {
	if s.loadPlacement == nil {
		return nil, ErrPlacementNotFound
	}
	return s.loadPlacement(ctx, shardID)
}
