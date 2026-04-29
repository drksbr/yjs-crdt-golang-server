package ycluster

import (
	"context"
	"errors"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
)

func TestStoragePlacementDocumentSourceListsPlacementKeys(t *testing.T) {
	t.Parallel()

	store := memory.New()
	ctx := context.Background()
	placements := []storage.PlacementRecord{
		{Key: storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-2"}, ShardID: "2"},
		{Key: storage.DocumentKey{Namespace: "tenant-b", DocumentID: "doc-1"}, ShardID: "3"},
		{Key: storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-1"}, ShardID: "1"},
	}
	for _, placement := range placements {
		if _, err := store.SavePlacement(ctx, placement); err != nil {
			t.Fatalf("SavePlacement(%#v) unexpected error: %v", placement.Key, err)
		}
	}

	source, err := NewStoragePlacementDocumentSource(StoragePlacementDocumentSourceConfig{
		Placements: store,
		Namespace:  "tenant-a",
	})
	if err != nil {
		t.Fatalf("NewStoragePlacementDocumentSource() unexpected error: %v", err)
	}
	keys, err := source.RebalanceDocuments(ctx)
	if err != nil {
		t.Fatalf("RebalanceDocuments() unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2", len(keys))
	}
	if keys[0] != (storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-1"}) ||
		keys[1] != (storage.DocumentKey{Namespace: "tenant-a", DocumentID: "doc-2"}) {
		t.Fatalf("keys = %#v, want sorted tenant-a docs", keys)
	}
}

func TestStoragePlacementDocumentSourceValidation(t *testing.T) {
	t.Parallel()

	_, err := NewStoragePlacementDocumentSource(StoragePlacementDocumentSourceConfig{})
	if !errors.Is(err, ErrNilPlacementStore) {
		t.Fatalf("NewStoragePlacementDocumentSource(empty) error = %v, want %v", err, ErrNilPlacementStore)
	}

	_, err = NewStoragePlacementDocumentSource(StoragePlacementDocumentSourceConfig{
		Placements: memory.New(),
		Limit:      -1,
	})
	if !errors.Is(err, ErrInvalidRebalancePlan) {
		t.Fatalf("NewStoragePlacementDocumentSource(negative limit) error = %v, want %v", err, ErrInvalidRebalancePlan)
	}
}
