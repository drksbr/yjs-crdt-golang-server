package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"yjs-go-bridge/pkg/storage"
)

func TestStoreDistributedNotFoundMappings(t *testing.T) {
	store, _ := newTestStore(t, false)
	ctx := context.Background()

	if _, err := store.LoadPlacement(ctx, storage.DocumentKey{DocumentID: "missing-placement"}); !errors.Is(err, storage.ErrPlacementNotFound) {
		t.Fatalf("LoadPlacement() error = %v, want %v", err, storage.ErrPlacementNotFound)
	}
	if _, err := store.LoadLease(ctx, storage.ShardID("missing-shard")); !errors.Is(err, storage.ErrLeaseNotFound) {
		t.Fatalf("LoadLease() error = %v, want %v", err, storage.ErrLeaseNotFound)
	}
	if err := store.ReleaseLease(ctx, storage.ShardID("missing-shard"), "lease-token"); !errors.Is(err, storage.ErrLeaseNotFound) {
		t.Fatalf("ReleaseLease() error = %v, want %v", err, storage.ErrLeaseNotFound)
	}
}

func TestStoreDistributedErrorContracts(t *testing.T) {
	t.Parallel()

	store := &Store{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	validKey := storage.DocumentKey{DocumentID: "doc-1"}
	validPlacement := storage.PlacementRecord{
		Key:     validKey,
		ShardID: storage.ShardID("shard-a"),
	}
	validLease := storage.LeaseRecord{
		ShardID:   storage.ShardID("shard-a"),
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-a"), Epoch: 1},
		Token:     "lease-token",
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{name: "append_respects_context", run: func() error { _, err := store.AppendUpdate(ctx, validKey, []byte{0x01}); return err }, wantErr: context.Canceled},
		{name: "append_rejects_invalid_key", run: func() error {
			_, err := store.AppendUpdate(context.Background(), storage.DocumentKey{}, []byte{0x01})
			return err
		}, wantErr: storage.ErrInvalidDocumentKey},
		{name: "append_rejects_empty_payload", run: func() error { _, err := store.AppendUpdate(context.Background(), validKey, nil); return err }, wantErr: storage.ErrInvalidUpdatePayload},
		{name: "list_rejects_invalid_key", run: func() error {
			_, err := store.ListUpdates(context.Background(), storage.DocumentKey{}, 0, 0)
			return err
		}, wantErr: storage.ErrInvalidDocumentKey},
		{name: "trim_rejects_invalid_key", run: func() error { return store.TrimUpdates(context.Background(), storage.DocumentKey{}, 0) }, wantErr: storage.ErrInvalidDocumentKey},
		{name: "save_placement_rejects_invalid_record", run: func() error {
			_, err := store.SavePlacement(context.Background(), storage.PlacementRecord{})
			return err
		}, wantErr: storage.ErrInvalidDocumentKey},
		{name: "load_placement_rejects_invalid_key", run: func() error { _, err := store.LoadPlacement(context.Background(), storage.DocumentKey{}); return err }, wantErr: storage.ErrInvalidDocumentKey},
		{name: "save_lease_rejects_invalid_record", run: func() error { _, err := store.SaveLease(context.Background(), storage.LeaseRecord{}); return err }, wantErr: storage.ErrInvalidShardID},
		{name: "load_lease_rejects_invalid_shard", run: func() error { _, err := store.LoadLease(context.Background(), storage.ShardID("")); return err }, wantErr: storage.ErrInvalidShardID},
		{name: "release_lease_rejects_invalid_shard", run: func() error { return store.ReleaseLease(context.Background(), storage.ShardID(""), "lease-token") }, wantErr: storage.ErrInvalidShardID},
		{name: "release_lease_rejects_empty_token", run: func() error { return store.ReleaseLease(context.Background(), storage.ShardID("shard-a"), "   ") }, wantErr: storage.ErrInvalidLeaseToken},
		{name: "append_requires_initialized_pool", run: func() error { _, err := store.AppendUpdate(context.Background(), validKey, []byte{0x01}); return err }, wantErr: errUninitializedStore},
		{name: "list_requires_initialized_pool", run: func() error { _, err := store.ListUpdates(context.Background(), validKey, 0, 0); return err }, wantErr: errUninitializedStore},
		{name: "trim_requires_initialized_pool", run: func() error { return store.TrimUpdates(context.Background(), validKey, 0) }, wantErr: errUninitializedStore},
		{name: "save_placement_requires_initialized_pool", run: func() error { _, err := store.SavePlacement(context.Background(), validPlacement); return err }, wantErr: errUninitializedStore},
		{name: "load_placement_requires_initialized_pool", run: func() error { _, err := store.LoadPlacement(context.Background(), validKey); return err }, wantErr: errUninitializedStore},
		{name: "save_lease_requires_initialized_pool", run: func() error { _, err := store.SaveLease(context.Background(), validLease); return err }, wantErr: errUninitializedStore},
		{name: "load_lease_requires_initialized_pool", run: func() error { _, err := store.LoadLease(context.Background(), storage.ShardID("shard-a")); return err }, wantErr: errUninitializedStore},
		{name: "release_lease_requires_initialized_pool", run: func() error {
			return store.ReleaseLease(context.Background(), storage.ShardID("shard-a"), "lease-token")
		}, wantErr: errUninitializedStore},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.run(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("erro = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
