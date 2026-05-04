package ycluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

func TestAuthorityFenceFromResolution(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().UTC().Add(time.Minute)
	resolution := &OwnerResolution{
		DocumentKey: storage.DocumentKey{Namespace: "tests", DocumentID: "doc-fence"},
		Local:       true,
		Placement: Placement{
			ShardID: 7,
			NodeID:  "node-a",
			Lease: &Lease{
				ShardID:   7,
				Holder:    "node-a",
				Epoch:     3,
				Token:     "lease-a",
				ExpiresAt: expiresAt,
			},
			Version: 2,
		},
	}

	fence, err := AuthorityFenceFromResolution(resolution)
	if err != nil {
		t.Fatalf("AuthorityFenceFromResolution() unexpected error: %v", err)
	}
	if fence.ShardID != StorageShardID(7) {
		t.Fatalf("fence.ShardID = %q, want %q", fence.ShardID, StorageShardID(7))
	}
	if fence.Owner.NodeID != storage.NodeID("node-a") || fence.Owner.Epoch != 3 {
		t.Fatalf("fence.Owner = %#v, want node-a/3", fence.Owner)
	}
	if fence.Token != "lease-a" {
		t.Fatalf("fence.Token = %q, want %q", fence.Token, "lease-a")
	}
}

func TestAuthorityFenceFromResolutionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		resolution *OwnerResolution
		wantErr    error
	}{
		{
			name:    "nil resolution",
			wantErr: storage.ErrAuthorityLost,
		},
		{
			name: "remote owner",
			resolution: &OwnerResolution{
				Placement: Placement{ShardID: 7, NodeID: "node-b"},
			},
			wantErr: storage.ErrAuthorityLost,
		},
		{
			name: "missing lease",
			resolution: &OwnerResolution{
				Local:     true,
				Placement: Placement{ShardID: 7, NodeID: "node-a"},
			},
			wantErr: storage.ErrAuthorityLost,
		},
		{
			name: "invalid placement",
			resolution: &OwnerResolution{
				Local: true,
				Placement: Placement{
					ShardID: 7,
					NodeID:  "node-a",
					Lease: &Lease{
						ShardID:   8,
						Holder:    "node-a",
						Epoch:     1,
						Token:     "lease-a",
						ExpiresAt: time.Now().UTC().Add(time.Minute),
					},
				},
			},
			wantErr: ErrInvalidPlacement,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := AuthorityFenceFromResolution(tt.resolution)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("AuthorityFenceFromResolution() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveStorageAuthorityFence(t *testing.T) {
	t.Parallel()

	lookup := ownerLookupFunc(func(ctx context.Context, req OwnerLookupRequest) (*OwnerResolution, error) {
		if ctx == nil {
			t.Fatal("LookupOwner() recebeu nil context")
		}
		if req.DocumentKey.DocumentID != "doc-fence" {
			t.Fatalf("LookupOwner().DocumentKey = %#v, want doc-fence", req.DocumentKey)
		}
		return &OwnerResolution{
			DocumentKey: req.DocumentKey,
			Local:       true,
			Placement: Placement{
				ShardID: 9,
				NodeID:  "node-a",
				Lease: &Lease{
					ShardID:   9,
					Holder:    "node-a",
					Epoch:     4,
					Token:     "lease-b",
					ExpiresAt: time.Now().UTC().Add(time.Minute),
				},
			},
		}, nil
	})

	fence, err := ResolveStorageAuthorityFence(context.Background(), lookup, storage.DocumentKey{DocumentID: "doc-fence"})
	if err != nil {
		t.Fatalf("ResolveStorageAuthorityFence() unexpected error: %v", err)
	}
	if fence.Owner.Epoch != 4 || fence.Token != "lease-b" {
		t.Fatalf("ResolveStorageAuthorityFence() = %#v, want epoch=4 token=lease-b", fence)
	}
}

func TestResolveStorageAuthorityFenceMapsUnavailableOwnerToAuthorityLost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lookup  OwnerLookup
		wantErr error
	}{
		{
			name:    "nil lookup",
			wantErr: storage.ErrAuthorityLost,
		},
		{
			name: "owner not found",
			lookup: ownerLookupFunc(func(context.Context, OwnerLookupRequest) (*OwnerResolution, error) {
				return nil, ErrOwnerNotFound
			}),
			wantErr: storage.ErrAuthorityLost,
		},
		{
			name: "lookup failure propagates",
			lookup: ownerLookupFunc(func(context.Context, OwnerLookupRequest) (*OwnerResolution, error) {
				return nil, context.Canceled
			}),
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ResolveStorageAuthorityFence(context.Background(), tt.lookup, storage.DocumentKey{DocumentID: "doc-fence"})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ResolveStorageAuthorityFence() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

type ownerLookupFunc func(ctx context.Context, req OwnerLookupRequest) (*OwnerResolution, error)

func (f ownerLookupFunc) LookupOwner(ctx context.Context, req OwnerLookupRequest) (*OwnerResolution, error) {
	return f(ctx, req)
}
