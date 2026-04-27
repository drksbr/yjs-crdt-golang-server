package postgres

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"yjs-go-bridge/pkg/storage"
)

func TestStoreAppendListTrimUpdates(t *testing.T) {
	store, _ := newTestStore(t, false)
	ctx := context.Background()
	key := storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-log"}

	first, err := store.AppendUpdate(ctx, key, []byte{0x01})
	if err != nil {
		t.Fatalf("AppendUpdate(first) unexpected error: %v", err)
	}
	second, err := store.AppendUpdate(ctx, key, []byte{0x02})
	if err != nil {
		t.Fatalf("AppendUpdate(second) unexpected error: %v", err)
	}
	third, err := store.AppendUpdate(ctx, key, []byte{0x03})
	if err != nil {
		t.Fatalf("AppendUpdate(third) unexpected error: %v", err)
	}

	if first.Offset != 1 || second.Offset != 2 || third.Offset != 3 {
		t.Fatalf("offsets = [%d %d %d], want [1 2 3]", first.Offset, second.Offset, third.Offset)
	}
	if first.StoredAt.IsZero() || second.StoredAt.IsZero() || third.StoredAt.IsZero() {
		t.Fatal("AppendUpdate() returned zero StoredAt")
	}

	records, err := store.ListUpdates(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("ListUpdates(all) unexpected error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("ListUpdates(all) len = %d, want 3", len(records))
	}

	records[0].UpdateV1[0] = 0xff
	reloaded, err := store.ListUpdates(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("ListUpdates(reload) unexpected error: %v", err)
	}
	if !bytes.Equal(reloaded[0].UpdateV1, []byte{0x01}) {
		t.Fatalf("ListUpdates(reload)[0].UpdateV1 = %v, want %v", reloaded[0].UpdateV1, []byte{0x01})
	}

	afterFirst, err := store.ListUpdates(ctx, key, 1, 1)
	if err != nil {
		t.Fatalf("ListUpdates(afterFirst) unexpected error: %v", err)
	}
	if len(afterFirst) != 1 || afterFirst[0].Offset != 2 {
		t.Fatalf("ListUpdates(afterFirst) = %#v, want single offset 2", afterFirst)
	}

	if err := store.TrimUpdates(ctx, key, 2); err != nil {
		t.Fatalf("TrimUpdates() unexpected error: %v", err)
	}

	trimmed, err := store.ListUpdates(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("ListUpdates(after trim) unexpected error: %v", err)
	}
	if len(trimmed) != 1 || trimmed[0].Offset != 3 {
		t.Fatalf("ListUpdates(after trim) = %#v, want single offset 3", trimmed)
	}

	fourth, err := store.AppendUpdate(ctx, key, []byte{0x04})
	if err != nil {
		t.Fatalf("AppendUpdate(fourth) unexpected error: %v", err)
	}
	if fourth.Offset != 4 {
		t.Fatalf("AppendUpdate(fourth).Offset = %d, want 4", fourth.Offset)
	}
}

func TestStoreSaveAndLoadPlacement(t *testing.T) {
	store, _ := newTestStore(t, false)
	ctx := context.Background()

	placement, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-placement"},
		ShardID: storage.ShardID("shard-a"),
		Version: 2,
	})
	if err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}
	if placement.UpdatedAt.IsZero() {
		t.Fatal("SavePlacement().UpdatedAt is zero")
	}

	loaded, err := store.LoadPlacement(ctx, placement.Key)
	if err != nil {
		t.Fatalf("LoadPlacement() unexpected error: %v", err)
	}
	if *loaded != *placement {
		t.Fatalf("LoadPlacement() = %#v, want %#v", loaded, placement)
	}

	time.Sleep(20 * time.Millisecond)
	updated, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     placement.Key,
		ShardID: storage.ShardID("shard-b"),
		Version: 3,
	})
	if err != nil {
		t.Fatalf("SavePlacement(update) unexpected error: %v", err)
	}
	if updated.ShardID != "shard-b" || updated.Version != 3 {
		t.Fatalf("SavePlacement(update) = %#v, want shard-b/version 3", updated)
	}
	if !updated.UpdatedAt.After(placement.UpdatedAt) {
		t.Fatalf("SavePlacement(update).UpdatedAt = %v, want after %v", updated.UpdatedAt, placement.UpdatedAt)
	}
}

func TestStoreSaveLoadAndReleaseLease(t *testing.T) {
	store, _ := newTestStore(t, false)
	ctx := context.Background()

	lease, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID:   storage.ShardID("shard-lease"),
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-a"), Epoch: 4},
		Token:     "lease-a",
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("SaveLease() unexpected error: %v", err)
	}
	if lease.AcquiredAt.IsZero() {
		t.Fatal("SaveLease().AcquiredAt is zero")
	}

	loaded, err := store.LoadLease(ctx, lease.ShardID)
	if err != nil {
		t.Fatalf("LoadLease() unexpected error: %v", err)
	}
	if *loaded != *lease {
		t.Fatalf("LoadLease() = %#v, want %#v", loaded, lease)
	}

	if err := store.ReleaseLease(ctx, lease.ShardID, "wrong-token"); !errors.Is(err, storage.ErrLeaseNotFound) {
		t.Fatalf("ReleaseLease(wrong-token) error = %v, want %v", err, storage.ErrLeaseNotFound)
	}

	if err := store.ReleaseLease(ctx, lease.ShardID, lease.Token); err != nil {
		t.Fatalf("ReleaseLease() unexpected error: %v", err)
	}
	if _, err := store.LoadLease(ctx, lease.ShardID); !errors.Is(err, storage.ErrLeaseNotFound) {
		t.Fatalf("LoadLease(after release) error = %v, want %v", err, storage.ErrLeaseNotFound)
	}
}
