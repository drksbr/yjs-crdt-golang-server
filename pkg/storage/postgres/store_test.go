package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/yjsbridge"
)

func TestStoreSaveAndLoadSnapshotRoundTrip(t *testing.T) {
	store, _ := newTestStore(t, false)
	ctx := context.Background()

	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}

	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "save-load-round-trip",
	}

	saved, err := store.SaveSnapshot(ctx, key, snapshot)
	if err != nil {
		t.Fatalf("SaveSnapshot() unexpected error: %v", err)
	}
	if saved.StoredAt.IsZero() {
		t.Fatal("SaveSnapshot().StoredAt is zero")
	}

	loaded, err := store.LoadSnapshot(ctx, key)
	if err != nil {
		t.Fatalf("LoadSnapshot() unexpected error: %v", err)
	}
	if !bytes.Equal(loaded.Snapshot.UpdateV1, snapshot.UpdateV1) {
		t.Fatalf("LoadSnapshot().Snapshot.UpdateV1 = %v, want %v", loaded.Snapshot.UpdateV1, snapshot.UpdateV1)
	}
	if len(loaded.Snapshot.UpdateV1) == 0 {
		t.Fatalf("LoadSnapshot().Snapshot.UpdateV1 is empty")
	}

	loaded.Snapshot.UpdateV1[0] = ^loaded.Snapshot.UpdateV1[0]
	reloaded, err := store.LoadSnapshot(ctx, key)
	if err != nil {
		t.Fatalf("LoadSnapshot() unexpected error after mutation: %v", err)
	}
	if bytes.Equal(reloaded.Snapshot.UpdateV1, loaded.Snapshot.UpdateV1) {
		t.Fatal("mutação vazou do retorno de LoadSnapshot()")
	}

	time.Sleep(20 * time.Millisecond)
	again, err := store.SaveSnapshot(ctx, key, snapshot)
	if err != nil {
		t.Fatalf("SaveSnapshot() second call unexpected error: %v", err)
	}
	if !again.StoredAt.After(saved.StoredAt) {
		t.Fatalf("segunda SaveSnapshot().StoredAt = %v, want after %v", again.StoredAt, saved.StoredAt)
	}
}

func TestStoreLoadMissingSnapshot(t *testing.T) {
	store, _ := newTestStore(t, true)
	ctx := context.Background()

	_, err := store.LoadSnapshot(ctx, storage.DocumentKey{DocumentID: "not-found"})
	if !errors.Is(err, storage.ErrSnapshotNotFound) {
		t.Fatalf("LoadSnapshot() error = %v, want %v", err, storage.ErrSnapshotNotFound)
	}
}

func TestStoreErrorContracts(t *testing.T) {
	t.Parallel()

	store := &Store{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{
			name: "save_respects_context",
			run: func() error {
				_, err := store.SaveSnapshot(ctx, storage.DocumentKey{DocumentID: "doc-1"}, snapshot)
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "load_respects_context",
			run: func() error {
				_, err := store.LoadSnapshot(ctx, storage.DocumentKey{DocumentID: "doc-1"})
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "save_rejects_nil_snapshot",
			run: func() error {
				_, err := store.SaveSnapshot(context.Background(), storage.DocumentKey{DocumentID: "doc-1"}, nil)
				return err
			},
			wantErr: storage.ErrNilPersistedSnapshot,
		},
		{
			name: "save_rejects_invalid_key",
			run: func() error {
				_, err := store.SaveSnapshot(context.Background(), storage.DocumentKey{}, snapshot)
				return err
			},
			wantErr: storage.ErrInvalidDocumentKey,
		},
		{
			name: "load_rejects_invalid_key",
			run: func() error {
				_, err := store.LoadSnapshot(context.Background(), storage.DocumentKey{})
				return err
			},
			wantErr: storage.ErrInvalidDocumentKey,
		},
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

func TestStoreRequiresInitializedPool(t *testing.T) {
	t.Parallel()

	store := &Store{}
	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}

	if _, err := store.SaveSnapshot(context.Background(), storage.DocumentKey{DocumentID: "doc-1"}, snapshot); !errors.Is(err, errUninitializedStore) {
		t.Fatalf("SaveSnapshot() error = %v, want %v", err, errUninitializedStore)
	}
	if _, err := store.LoadSnapshot(context.Background(), storage.DocumentKey{DocumentID: "doc-1"}); !errors.Is(err, errUninitializedStore) {
		t.Fatalf("LoadSnapshot() error = %v, want %v", err, errUninitializedStore)
	}
}

func TestStoreConcurrentSaveLoadSmoke(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t, false)
	ctx := context.Background()

	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}

	const workers = 6
	const iterations = 30
	var wg sync.WaitGroup
	errCh := make(chan error, workers*iterations*2)

	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := storage.DocumentKey{DocumentID: fmt.Sprintf("doc-%d", worker%3)}
			for i := 0; i < iterations; i++ {
				current := snapshot.Clone()
				if _, err := store.SaveSnapshot(ctx, key, current); err != nil {
					errCh <- err
					return
				}
				if _, err := store.LoadSnapshot(ctx, key); err != nil {
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
