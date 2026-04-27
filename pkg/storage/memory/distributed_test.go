package memory

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"yjs-go-bridge/pkg/storage"
)

func TestStoreAppendListAndTrimUpdates(t *testing.T) {
	t.Parallel()

	store := New()
	store.now = sequenceClock(
		time.Unix(100, 0).UTC(),
		time.Unix(101, 0).UTC(),
		time.Unix(102, 0).UTC(),
		time.Unix(103, 0).UTC(),
	)

	key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"}
	otherKey := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-2"}

	firstPayload := []byte{0x01, 0x02}
	secondPayload := []byte{0x03, 0x04}
	thirdPayload := []byte{0x05, 0x06}

	first, err := store.AppendUpdate(context.Background(), key, firstPayload)
	if err != nil {
		t.Fatalf("AppendUpdate(first) unexpected error: %v", err)
	}
	if first.Offset != 1 {
		t.Fatalf("first.Offset = %d, want 1", first.Offset)
	}
	if !first.StoredAt.Equal(time.Unix(100, 0).UTC()) {
		t.Fatalf("first.StoredAt = %v, want %v", first.StoredAt, time.Unix(100, 0).UTC())
	}
	firstPayload[0] = 0xff
	if first.UpdateV1[0] == 0xff {
		t.Fatal("AppendUpdate() did not defensively copy input payload")
	}

	second, err := store.AppendUpdate(context.Background(), key, secondPayload)
	if err != nil {
		t.Fatalf("AppendUpdate(second) unexpected error: %v", err)
	}
	if second.Offset != 2 {
		t.Fatalf("second.Offset = %d, want 2", second.Offset)
	}

	if _, err := store.AppendUpdate(context.Background(), otherKey, []byte{0x09}); err != nil {
		t.Fatalf("AppendUpdate(other key) unexpected error: %v", err)
	}

	third, err := store.AppendUpdate(context.Background(), key, thirdPayload)
	if err != nil {
		t.Fatalf("AppendUpdate(third) unexpected error: %v", err)
	}
	if third.Offset != 3 {
		t.Fatalf("third.Offset = %d, want 3", third.Offset)
	}

	tests := []struct {
		name        string
		after       storage.UpdateOffset
		limit       int
		wantOffsets []storage.UpdateOffset
		wantPayload [][]byte
	}{
		{
			name:        "all",
			after:       0,
			limit:       0,
			wantOffsets: []storage.UpdateOffset{1, 2, 3},
			wantPayload: [][]byte{{0x01, 0x02}, {0x03, 0x04}, {0x05, 0x06}},
		},
		{
			name:        "after_first",
			after:       1,
			limit:       0,
			wantOffsets: []storage.UpdateOffset{2, 3},
			wantPayload: [][]byte{{0x03, 0x04}, {0x05, 0x06}},
		},
		{
			name:        "limit_one",
			after:       1,
			limit:       1,
			wantOffsets: []storage.UpdateOffset{2},
			wantPayload: [][]byte{{0x03, 0x04}},
		},
		{
			name:        "after_last",
			after:       99,
			limit:       0,
			wantOffsets: nil,
			wantPayload: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			records, err := store.ListUpdates(context.Background(), key, tt.after, tt.limit)
			if err != nil {
				t.Fatalf("ListUpdates() unexpected error: %v", err)
			}
			if len(records) != len(tt.wantOffsets) {
				t.Fatalf("len(records) = %d, want %d", len(records), len(tt.wantOffsets))
			}
			for i, record := range records {
				if record.Offset != tt.wantOffsets[i] {
					t.Fatalf("records[%d].Offset = %d, want %d", i, record.Offset, tt.wantOffsets[i])
				}
				if !bytes.Equal(record.UpdateV1, tt.wantPayload[i]) {
					t.Fatalf("records[%d].UpdateV1 = %v, want %v", i, record.UpdateV1, tt.wantPayload[i])
				}
			}
		})
	}

	records, err := store.ListUpdates(context.Background(), key, 0, 0)
	if err != nil {
		t.Fatalf("ListUpdates() unexpected error: %v", err)
	}
	records[0].UpdateV1[0] = 0xee
	reloaded, err := store.ListUpdates(context.Background(), key, 0, 0)
	if err != nil {
		t.Fatalf("ListUpdates() unexpected error after mutation: %v", err)
	}
	if reloaded[0].UpdateV1[0] == 0xee {
		t.Fatal("ListUpdates() leaked internal payload mutation")
	}

	if err := store.TrimUpdates(context.Background(), key, 1); err != nil {
		t.Fatalf("TrimUpdates(1) unexpected error: %v", err)
	}
	remaining, err := store.ListUpdates(context.Background(), key, 0, 0)
	if err != nil {
		t.Fatalf("ListUpdates() after trim unexpected error: %v", err)
	}
	if len(remaining) != 2 || remaining[0].Offset != 2 || remaining[1].Offset != 3 {
		t.Fatalf("remaining offsets after trim = %#v, want [2 3]", remaining)
	}

	if err := store.TrimUpdates(context.Background(), key, 99); err != nil {
		t.Fatalf("TrimUpdates(99) unexpected error: %v", err)
	}
	empty, err := store.ListUpdates(context.Background(), key, 0, 0)
	if err != nil {
		t.Fatalf("ListUpdates() after trim all unexpected error: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("len(empty) = %d, want 0", len(empty))
	}

	afterTrim, err := store.AppendUpdate(context.Background(), key, []byte{0x07})
	if err != nil {
		t.Fatalf("AppendUpdate() after trim unexpected error: %v", err)
	}
	if afterTrim.Offset != 4 {
		t.Fatalf("offset after trim = %d, want 4", afterTrim.Offset)
	}
}

func TestStoreSaveAndLoadPlacement(t *testing.T) {
	t.Parallel()

	store := New()
	stamp := time.Unix(200, 0).UTC()
	store.now = sequenceClock(stamp, stamp.Add(time.Second))

	key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-placement"}
	placement := storage.PlacementRecord{
		Key:     key,
		ShardID: storage.ShardID("shard-a"),
		Version: 7,
	}

	saved, err := store.SavePlacement(context.Background(), placement)
	if err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}
	if !saved.UpdatedAt.Equal(stamp) {
		t.Fatalf("saved.UpdatedAt = %v, want %v", saved.UpdatedAt, stamp)
	}

	loaded, err := store.LoadPlacement(context.Background(), key)
	if err != nil {
		t.Fatalf("LoadPlacement() unexpected error: %v", err)
	}
	if *loaded != *saved {
		t.Fatalf("loaded = %#v, want %#v", loaded, saved)
	}

	loaded.ShardID = storage.ShardID("other")
	reloaded, err := store.LoadPlacement(context.Background(), key)
	if err != nil {
		t.Fatalf("LoadPlacement() unexpected error after mutation: %v", err)
	}
	if reloaded.ShardID != saved.ShardID {
		t.Fatalf("reloaded.ShardID = %q, want %q", reloaded.ShardID, saved.ShardID)
	}

	replacement := storage.PlacementRecord{
		Key:       key,
		ShardID:   storage.ShardID("shard-b"),
		Version:   8,
		UpdatedAt: stamp.Add(10 * time.Second),
	}
	savedAgain, err := store.SavePlacement(context.Background(), replacement)
	if err != nil {
		t.Fatalf("SavePlacement(replacement) unexpected error: %v", err)
	}
	if !savedAgain.UpdatedAt.Equal(replacement.UpdatedAt) {
		t.Fatalf("savedAgain.UpdatedAt = %v, want %v", savedAgain.UpdatedAt, replacement.UpdatedAt)
	}
	if savedAgain.ShardID != replacement.ShardID {
		t.Fatalf("savedAgain.ShardID = %q, want %q", savedAgain.ShardID, replacement.ShardID)
	}
}

func TestStoreSaveLoadAndReleaseLease(t *testing.T) {
	t.Parallel()

	store := New()
	acquiredAt := time.Unix(300, 0).UTC()
	store.now = sequenceClock(acquiredAt, acquiredAt.Add(time.Second))

	shardID := storage.ShardID("shard-lease")
	lease := storage.LeaseRecord{
		ShardID:   shardID,
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-a"), Epoch: 2},
		Token:     "lease-token",
		ExpiresAt: acquiredAt.Add(30 * time.Second),
	}

	saved, err := store.SaveLease(context.Background(), lease)
	if err != nil {
		t.Fatalf("SaveLease() unexpected error: %v", err)
	}
	if !saved.AcquiredAt.Equal(acquiredAt) {
		t.Fatalf("saved.AcquiredAt = %v, want %v", saved.AcquiredAt, acquiredAt)
	}

	loaded, err := store.LoadLease(context.Background(), shardID)
	if err != nil {
		t.Fatalf("LoadLease() unexpected error: %v", err)
	}
	if *loaded != *saved {
		t.Fatalf("loaded = %#v, want %#v", loaded, saved)
	}

	loaded.Token = "other"
	reloaded, err := store.LoadLease(context.Background(), shardID)
	if err != nil {
		t.Fatalf("LoadLease() unexpected error after mutation: %v", err)
	}
	if reloaded.Token != saved.Token {
		t.Fatalf("reloaded.Token = %q, want %q", reloaded.Token, saved.Token)
	}

	if err := store.ReleaseLease(context.Background(), shardID, "wrong-token"); !errors.Is(err, storage.ErrLeaseNotFound) {
		t.Fatalf("ReleaseLease(wrong token) error = %v, want %v", err, storage.ErrLeaseNotFound)
	}

	if err := store.ReleaseLease(context.Background(), shardID, saved.Token); err != nil {
		t.Fatalf("ReleaseLease(valid token) unexpected error: %v", err)
	}
	if _, err := store.LoadLease(context.Background(), shardID); !errors.Is(err, storage.ErrLeaseNotFound) {
		t.Fatalf("LoadLease() after release error = %v, want %v", err, storage.ErrLeaseNotFound)
	}
}

func TestStoreDistributedErrors(t *testing.T) {
	t.Parallel()

	store := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	key := storage.DocumentKey{DocumentID: "doc-1"}
	placement := storage.PlacementRecord{
		Key:     key,
		ShardID: storage.ShardID("shard-a"),
	}
	lease := storage.LeaseRecord{
		ShardID:   storage.ShardID("shard-a"),
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-a")},
		Token:     "lease-token",
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{
			name: "append_respects_context",
			run: func() error {
				_, err := store.AppendUpdate(ctx, key, []byte{0x01})
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "list_respects_context",
			run: func() error {
				_, err := store.ListUpdates(ctx, key, 0, 0)
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "trim_respects_context",
			run: func() error {
				return store.TrimUpdates(ctx, key, 0)
			},
			wantErr: context.Canceled,
		},
		{
			name: "save_placement_respects_context",
			run: func() error {
				_, err := store.SavePlacement(ctx, placement)
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "load_placement_respects_context",
			run: func() error {
				_, err := store.LoadPlacement(ctx, key)
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "save_lease_respects_context",
			run: func() error {
				_, err := store.SaveLease(ctx, lease)
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "load_lease_respects_context",
			run: func() error {
				_, err := store.LoadLease(ctx, storage.ShardID("shard-a"))
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "release_lease_respects_context",
			run: func() error {
				return store.ReleaseLease(ctx, storage.ShardID("shard-a"), "lease-token")
			},
			wantErr: context.Canceled,
		},
		{
			name: "append_rejects_invalid_key",
			run: func() error {
				_, err := store.AppendUpdate(context.Background(), storage.DocumentKey{}, []byte{0x01})
				return err
			},
			wantErr: storage.ErrInvalidDocumentKey,
		},
		{
			name: "append_rejects_empty_payload",
			run: func() error {
				_, err := store.AppendUpdate(context.Background(), key, nil)
				return err
			},
			wantErr: storage.ErrInvalidUpdatePayload,
		},
		{
			name: "list_rejects_invalid_key",
			run: func() error {
				_, err := store.ListUpdates(context.Background(), storage.DocumentKey{}, 0, 0)
				return err
			},
			wantErr: storage.ErrInvalidDocumentKey,
		},
		{
			name: "trim_rejects_invalid_key",
			run: func() error {
				return store.TrimUpdates(context.Background(), storage.DocumentKey{}, 0)
			},
			wantErr: storage.ErrInvalidDocumentKey,
		},
		{
			name: "load_placement_rejects_invalid_key",
			run: func() error {
				_, err := store.LoadPlacement(context.Background(), storage.DocumentKey{})
				return err
			},
			wantErr: storage.ErrInvalidDocumentKey,
		},
		{
			name: "save_placement_rejects_invalid_placement",
			run: func() error {
				_, err := store.SavePlacement(context.Background(), storage.PlacementRecord{})
				return err
			},
			wantErr: storage.ErrInvalidDocumentKey,
		},
		{
			name: "load_placement_rejects_missing_placement",
			run: func() error {
				_, err := store.LoadPlacement(context.Background(), key)
				return err
			},
			wantErr: storage.ErrPlacementNotFound,
		},
		{
			name: "save_lease_rejects_invalid_lease",
			run: func() error {
				_, err := store.SaveLease(context.Background(), storage.LeaseRecord{})
				return err
			},
			wantErr: storage.ErrInvalidShardID,
		},
		{
			name: "load_lease_rejects_invalid_shard",
			run: func() error {
				_, err := store.LoadLease(context.Background(), storage.ShardID(""))
				return err
			},
			wantErr: storage.ErrInvalidShardID,
		},
		{
			name: "load_lease_rejects_missing_lease",
			run: func() error {
				_, err := store.LoadLease(context.Background(), storage.ShardID("missing"))
				return err
			},
			wantErr: storage.ErrLeaseNotFound,
		},
		{
			name: "release_lease_rejects_invalid_shard",
			run: func() error {
				return store.ReleaseLease(context.Background(), storage.ShardID(""), "lease-token")
			},
			wantErr: storage.ErrInvalidShardID,
		},
		{
			name: "release_lease_rejects_empty_token",
			run: func() error {
				return store.ReleaseLease(context.Background(), storage.ShardID("shard-a"), "")
			},
			wantErr: storage.ErrInvalidLeaseToken,
		},
		{
			name: "release_lease_rejects_missing_lease",
			run: func() error {
				return store.ReleaseLease(context.Background(), storage.ShardID("shard-a"), "lease-token")
			},
			wantErr: storage.ErrLeaseNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNilStoreDistributedErrors(t *testing.T) {
	t.Parallel()

	var store *Store
	key := storage.DocumentKey{DocumentID: "doc-1"}
	shardID := storage.ShardID("shard-a")

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{
			name: "append",
			run: func() error {
				_, err := store.AppendUpdate(context.Background(), key, []byte{0x01})
				return err
			},
			wantErr: errNilStore,
		},
		{
			name: "list",
			run: func() error {
				_, err := store.ListUpdates(context.Background(), key, 0, 0)
				return err
			},
			wantErr: errNilStore,
		},
		{
			name: "trim",
			run: func() error {
				return store.TrimUpdates(context.Background(), key, 0)
			},
			wantErr: errNilStore,
		},
		{
			name: "save_placement",
			run: func() error {
				_, err := store.SavePlacement(context.Background(), storage.PlacementRecord{
					Key:     key,
					ShardID: storage.ShardID("shard-a"),
				})
				return err
			},
			wantErr: errNilStore,
		},
		{
			name: "load_placement",
			run: func() error {
				_, err := store.LoadPlacement(context.Background(), key)
				return err
			},
			wantErr: errNilStore,
		},
		{
			name: "save_lease",
			run: func() error {
				_, err := store.SaveLease(context.Background(), storage.LeaseRecord{
					ShardID:   shardID,
					Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-a")},
					Token:     "lease-token",
					ExpiresAt: time.Now().UTC().Add(time.Minute),
				})
				return err
			},
			wantErr: errNilStore,
		},
		{
			name: "load_lease",
			run: func() error {
				_, err := store.LoadLease(context.Background(), shardID)
				return err
			},
			wantErr: errNilStore,
		},
		{
			name: "release_lease",
			run: func() error {
				return store.ReleaseLease(context.Background(), shardID, "lease-token")
			},
			wantErr: errNilStore,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestZeroValueStoreDistributedContracts(t *testing.T) {
	t.Parallel()

	var store Store
	store.now = sequenceClock(
		time.Unix(400, 0).UTC(),
		time.Unix(401, 0).UTC(),
	)

	key := storage.DocumentKey{DocumentID: "doc-zero"}
	shardID := storage.ShardID("shard-zero")

	update, err := store.AppendUpdate(context.Background(), key, []byte{0x01})
	if err != nil {
		t.Fatalf("AppendUpdate() on zero-value store unexpected error: %v", err)
	}
	if update.Offset != 1 {
		t.Fatalf("update.Offset = %d, want 1", update.Offset)
	}

	placement, err := store.SavePlacement(context.Background(), storage.PlacementRecord{
		Key:     key,
		ShardID: storage.ShardID("shard-a"),
	})
	if err != nil {
		t.Fatalf("SavePlacement() on zero-value store unexpected error: %v", err)
	}
	if placement.UpdatedAt.IsZero() {
		t.Fatal("placement.UpdatedAt is zero")
	}

	lease, err := store.SaveLease(context.Background(), storage.LeaseRecord{
		ShardID:   shardID,
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-a")},
		Token:     "lease-token",
		ExpiresAt: time.Unix(500, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("SaveLease() on zero-value store unexpected error: %v", err)
	}
	if lease.AcquiredAt.IsZero() {
		t.Fatal("lease.AcquiredAt is zero")
	}
}

func TestStoreConcurrentDistributedOperations(t *testing.T) {
	t.Parallel()

	store := New()
	const workers = 8
	const iterations = 60

	var wg sync.WaitGroup
	errCh := make(chan error, workers*iterations*3)

	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()

			key := storage.DocumentKey{DocumentID: "doc-" + strconv.Itoa(worker%3)}
			shardID := storage.ShardID("shard-" + strconv.Itoa(worker%2))

			for iteration := 0; iteration < iterations; iteration++ {
				if _, err := store.AppendUpdate(context.Background(), key, []byte{byte(worker), byte(iteration)}); err != nil {
					errCh <- err
					return
				}
				if _, err := store.SavePlacement(context.Background(), storage.PlacementRecord{
					Key:     key,
					ShardID: shardID,
					Version: uint64(iteration),
				}); err != nil {
					errCh <- err
					return
				}
				lease := storage.LeaseRecord{
					ShardID:   shardID,
					Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-" + strconv.Itoa(worker))},
					Token:     "lease-" + strconv.Itoa(worker),
					ExpiresAt: time.Now().UTC().Add(time.Minute),
				}
				if _, err := store.SaveLease(context.Background(), lease); err != nil {
					errCh <- err
					return
				}
				if _, err := store.ListUpdates(context.Background(), key, 0, 5); err != nil {
					errCh <- err
					return
				}
				if _, err := store.LoadPlacement(context.Background(), key); err != nil {
					errCh <- err
					return
				}
				if _, err := store.LoadLease(context.Background(), shardID); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("erro concorrente: %v", err)
	}
}
