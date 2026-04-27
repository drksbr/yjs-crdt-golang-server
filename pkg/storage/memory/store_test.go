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
	"yjs-go-bridge/pkg/yjsbridge"
)

func TestStoreSaveAndLoadSnapshot(t *testing.T) {
	t.Parallel()

	baseSnapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}

	first := baseSnapshot.Clone()
	first.UpdateV1 = []byte{0x01}
	second := baseSnapshot.Clone()
	second.UpdateV1 = []byte{0x02}

	tests := []struct {
		name       string
		key        storage.DocumentKey
		first      *yjsbridge.PersistedSnapshot
		second     *yjsbridge.PersistedSnapshot
		timestamps []time.Time
	}{
		{
			name:       "grava e carrega snapshot inicial",
			key:        storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"},
			first:      first,
			timestamps: []time.Time{time.Unix(100, 0).UTC()},
		},
		{
			name:       "substitui snapshot com novo valor",
			key:        storage.DocumentKey{Namespace: "team-b", DocumentID: "doc-2"},
			first:      first,
			second:     second,
			timestamps: []time.Time{time.Unix(101, 0).UTC(), time.Unix(102, 0).UTC()},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := New()
			store.now = sequenceClock(tt.timestamps...)

			saved, err := store.SaveSnapshot(context.Background(), tt.key, tt.first)
			if err != nil {
				t.Fatalf("SaveSnapshot() unexpected error: %v", err)
			}
			if !saved.StoredAt.Equal(tt.timestamps[0]) {
				t.Fatalf("SaveSnapshot().StoredAt = %v, want %v", saved.StoredAt, tt.timestamps[0])
			}
			if !bytes.Equal(saved.Snapshot.UpdateV1, tt.first.UpdateV1) {
				t.Fatalf("snapshot gravado = %v, want %v", saved.Snapshot.UpdateV1, tt.first.UpdateV1)
			}

			loaded, err := store.LoadSnapshot(context.Background(), tt.key)
			if err != nil {
				t.Fatalf("LoadSnapshot() unexpected error: %v", err)
			}
			if !bytes.Equal(loaded.Snapshot.UpdateV1, tt.first.UpdateV1) {
				t.Fatalf("snapshot carregado = %v, want %v", loaded.Snapshot.UpdateV1, tt.first.UpdateV1)
			}

			loaded.Snapshot.UpdateV1 = []byte{0xff}
			reloaded, err := store.LoadSnapshot(context.Background(), tt.key)
			if err != nil {
				t.Fatalf("LoadSnapshot() unexpected error after mutation: %v", err)
			}
			if bytes.Equal(reloaded.Snapshot.UpdateV1, loaded.Snapshot.UpdateV1) {
				t.Fatalf("mutacao vazou do retorno de LoadSnapshot: %v", loaded.Snapshot.UpdateV1)
			}

			if tt.second == nil {
				if !bytes.Equal(reloaded.Snapshot.UpdateV1, tt.first.UpdateV1) {
					t.Fatalf("snapshot após re-leitura = %v, want %v", reloaded.Snapshot.UpdateV1, tt.first.UpdateV1)
				}
				return
			}

			saved, err = store.SaveSnapshot(context.Background(), tt.key, tt.second)
			if err != nil {
				t.Fatalf("SaveSnapshot() on second update unexpected error: %v", err)
			}
			if !saved.StoredAt.Equal(tt.timestamps[1]) {
				t.Fatalf("segunda SaveSnapshot().StoredAt = %v, want %v", saved.StoredAt, tt.timestamps[1])
			}
			if !bytes.Equal(saved.Snapshot.UpdateV1, tt.second.UpdateV1) {
				t.Fatalf("snapshot gravado no segundo save = %v, want %v", saved.Snapshot.UpdateV1, tt.second.UpdateV1)
			}

			latest, err := store.LoadSnapshot(context.Background(), tt.key)
			if err != nil {
				t.Fatalf("LoadSnapshot() after second save unexpected error: %v", err)
			}
			if !bytes.Equal(latest.Snapshot.UpdateV1, tt.second.UpdateV1) {
				t.Fatalf("snapshot após sobrescricao = %v, want %v", latest.Snapshot.UpdateV1, tt.second.UpdateV1)
			}
		})
	}
}

func TestStoreErrors(t *testing.T) {
	t.Parallel()

	store := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	key := storage.DocumentKey{DocumentID: "doc-1"}
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
				_, err := store.SaveSnapshot(ctx, key, snapshot)
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "load_respects_context",
			run: func() error {
				_, err := store.LoadSnapshot(ctx, key)
				return err
			},
			wantErr: context.Canceled,
		},
		{
			name: "save_rejects_nil_snapshot",
			run: func() error {
				_, err := store.SaveSnapshot(context.Background(), key, nil)
				return err
			},
			wantErr: storage.ErrNilPersistedSnapshot,
		},
		{
			name: "load_rejects_missing_snapshot",
			run: func() error {
				_, err := store.LoadSnapshot(context.Background(), key)
				return err
			},
			wantErr: storage.ErrSnapshotNotFound,
		},
		{
			name: "load_rejects_invalid_key",
			run: func() error {
				_, err := store.LoadSnapshot(context.Background(), storage.DocumentKey{})
				return err
			},
			wantErr: storage.ErrInvalidDocumentKey,
		},
		{
			name: "save_rejects_invalid_key",
			run: func() error {
				_, err := store.SaveSnapshot(context.Background(), storage.DocumentKey{}, snapshot)
				return err
			},
			wantErr: storage.ErrInvalidDocumentKey,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.run()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("erro = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNilStoreErrors(t *testing.T) {
	t.Parallel()

	var store *Store
	key := storage.DocumentKey{DocumentID: "doc-1"}
	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}

	if _, err := store.SaveSnapshot(context.Background(), key, snapshot); !errors.Is(err, errNilStore) {
		t.Fatalf("SaveSnapshot() error = %v, want %v", err, errNilStore)
	}
	if _, err := store.LoadSnapshot(context.Background(), key); !errors.Is(err, errNilStore) {
		t.Fatalf("LoadSnapshot() error = %v, want %v", err, errNilStore)
	}
}

func TestStoreZeroValueStore(t *testing.T) {
	t.Parallel()

	var store Store
	key := storage.DocumentKey{DocumentID: "doc-zero"}
	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}

	_, err = store.LoadSnapshot(context.Background(), key)
	if !errors.Is(err, storage.ErrSnapshotNotFound) {
		t.Fatalf("LoadSnapshot() error = %v, want %v", err, storage.ErrSnapshotNotFound)
	}

	saved, err := store.SaveSnapshot(context.Background(), key, snapshot)
	if err != nil {
		t.Fatalf("SaveSnapshot() on zero-value store unexpected error: %v", err)
	}
	if saved.StoredAt.IsZero() {
		t.Fatal("saved.StoredAt is zero for zero-value store")
	}

	loaded, err := store.LoadSnapshot(context.Background(), key)
	if err != nil {
		t.Fatalf("LoadSnapshot() unexpected error after save: %v", err)
	}
	if !bytes.Equal(loaded.Snapshot.UpdateV1, snapshot.UpdateV1) {
		t.Fatalf("loaded.Snapshot.UpdateV1 = %v, want %v", loaded.Snapshot.UpdateV1, snapshot.UpdateV1)
	}

	loaded.Snapshot.UpdateV1 = []byte{0x99}
	reloaded, err := store.LoadSnapshot(context.Background(), key)
	if err != nil {
		t.Fatalf("LoadSnapshot() unexpected error after mutation: %v", err)
	}
	if bytes.Equal(reloaded.Snapshot.UpdateV1, loaded.Snapshot.UpdateV1) {
		t.Fatalf("zero-value store returned mutated snapshot payload: %v", reloaded.Snapshot.UpdateV1)
	}
}

func TestStoreConcurrentSaveAndLoad(t *testing.T) {
	t.Parallel()

	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}

	tests := []struct {
		name       string
		workers    int
		iterations int
	}{
		{
			name:       "concorrencia_moderada",
			workers:    6,
			iterations: 120,
		},
		{
			name:       "concorrencia_mais_alta",
			workers:    12,
			iterations: 80,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := New()
			errCh := make(chan error, tt.workers*tt.iterations*2)
			var wg sync.WaitGroup

			for worker := 0; worker < tt.workers; worker++ {
				worker := worker
				wg.Add(1)
				go func() {
					defer wg.Done()
					key := storage.DocumentKey{DocumentID: "doc-" + strconv.Itoa(worker%3)}
					for iteration := 0; iteration < tt.iterations; iteration++ {
						current := snapshot.Clone()
						current.UpdateV1 = []byte{byte(worker), byte(iteration)}
						if _, err := store.SaveSnapshot(context.Background(), key, current); err != nil {
							errCh <- err
							return
						}
						if _, err := store.LoadSnapshot(context.Background(), key); err != nil {
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

			for doc := 0; doc < 3; doc++ {
				key := storage.DocumentKey{DocumentID: "doc-" + strconv.Itoa(doc)}
				record, err := store.LoadSnapshot(context.Background(), key)
				if err != nil {
					t.Fatalf("LoadSnapshot(%q) unexpected error: %v", key.DocumentID, err)
				}
				if record.StoredAt.IsZero() {
					t.Fatalf("StoredAt vazio para %s", key.DocumentID)
				}
				if len(record.Snapshot.UpdateV1) == 0 {
					t.Fatalf("UpdateV1 vazio para %s", key.DocumentID)
				}
			}
		})
	}
}

func sequenceClock(times ...time.Time) func() time.Time {
	index := 0
	return func() time.Time {
		if len(times) == 0 {
			return time.Time{}
		}
		if index >= len(times) {
			return times[len(times)-1]
		}
		value := times[index]
		index++
		return value
	}
}
