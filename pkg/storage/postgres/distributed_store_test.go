package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID:   lease.ShardID,
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-b"), Epoch: lease.Owner.Epoch},
		Token:     "lease-b-stale",
		ExpiresAt: time.Now().UTC().Add(90 * time.Second),
	}); !errors.Is(err, storage.ErrLeaseStaleEpoch) {
		t.Fatalf("SaveLease(stale after release) error = %v, want %v", err, storage.ErrLeaseStaleEpoch)
	}

	reacquired, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID:   lease.ShardID,
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-b"), Epoch: 5},
		Token:     "lease-b",
		ExpiresAt: time.Now().UTC().Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("SaveLease(reacquire) unexpected error: %v", err)
	}
	if reacquired.Owner.Epoch != 5 {
		t.Fatalf("SaveLease(reacquire).Owner.Epoch = %d, want 5", reacquired.Owner.Epoch)
	}
}

func TestStoreSaveLeaseConcurrentFirstAcquireFencesConflicts(t *testing.T) {
	store, _ := newTestStore(t, false)
	ctx := context.Background()

	const contenders = 8
	type result struct {
		token string
		err   error
	}

	results := make(chan result, contenders)
	start := make(chan struct{})

	for i := 0; i < contenders; i++ {
		i := i
		go func() {
			<-start
			token := fmt.Sprintf("lease-%d", i)
			_, err := store.SaveLease(ctx, storage.LeaseRecord{
				ShardID:   storage.ShardID("shard-race"),
				Owner:     storage.OwnerInfo{NodeID: storage.NodeID(fmt.Sprintf("node-%d", i)), Epoch: 1},
				Token:     token,
				ExpiresAt: time.Now().UTC().Add(time.Minute),
			})
			results <- result{token: token, err: err}
		}()
	}

	close(start)

	successes := 0
	var winnerToken string
	for i := 0; i < contenders; i++ {
		result := <-results
		switch {
		case result.err == nil:
			successes++
			winnerToken = result.token
		case errors.Is(result.err, storage.ErrLeaseConflict):
			// expected for losing contenders racing on the first acquire
		default:
			t.Fatalf("SaveLease(concurrent contender %q) error = %v, want nil or %v", result.token, result.err, storage.ErrLeaseConflict)
		}
	}

	if successes != 1 {
		t.Fatalf("successful first acquires = %d, want 1", successes)
	}

	lease, err := store.LoadLease(ctx, storage.ShardID("shard-race"))
	if err != nil {
		t.Fatalf("LoadLease() unexpected error: %v", err)
	}
	if lease.Token != winnerToken {
		t.Fatalf("LoadLease().Token = %q, want winner token %q", lease.Token, winnerToken)
	}
}

func TestStoreSaveLeaseRejectsConflictAndStaleEpoch(t *testing.T) {
	store, _ := newTestStore(t, false)
	ctx := context.Background()

	active, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID:   storage.ShardID("shard-fencing"),
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-a"), Epoch: 7},
		Token:     "lease-a",
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("SaveLease(active) unexpected error: %v", err)
	}

	renewed, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID:    active.ShardID,
		Owner:      active.Owner,
		Token:      active.Token,
		AcquiredAt: active.AcquiredAt,
		ExpiresAt:  time.Now().UTC().Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("SaveLease(renew) unexpected error: %v", err)
	}
	if renewed.Owner.Epoch != 7 || renewed.Token != active.Token {
		t.Fatalf("SaveLease(renew) = %#v, want epoch 7 token %q", renewed, active.Token)
	}

	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID:   active.ShardID,
		Owner:     active.Owner,
		Token:     "lease-conflict",
		ExpiresAt: time.Now().UTC().Add(3 * time.Minute),
	}); !errors.Is(err, storage.ErrLeaseConflict) {
		t.Fatalf("SaveLease(conflict same epoch) error = %v, want %v", err, storage.ErrLeaseConflict)
	}

	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID:   active.ShardID,
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-b"), Epoch: 8},
		Token:     "lease-b",
		ExpiresAt: time.Now().UTC().Add(3 * time.Minute),
	}); !errors.Is(err, storage.ErrLeaseConflict) {
		t.Fatalf("SaveLease(conflict higher epoch) error = %v, want %v", err, storage.ErrLeaseConflict)
	}

	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID:   active.ShardID,
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-a"), Epoch: 6},
		Token:     "lease-old",
		ExpiresAt: time.Now().UTC().Add(3 * time.Minute),
	}); !errors.Is(err, storage.ErrLeaseStaleEpoch) {
		t.Fatalf("SaveLease(stale) error = %v, want %v", err, storage.ErrLeaseStaleEpoch)
	}
}
