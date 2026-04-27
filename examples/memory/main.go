package main

import (
	"context"
	"fmt"
	"log"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/yjsbridge"
)

func main() {
	ctx := context.Background()
	store := memory.New()

	key := storage.DocumentKey{
		Namespace:  "notes",
		DocumentID: "document-1",
	}

	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates()
	if err != nil {
		log.Fatalf("criando snapshot inicial: %v", err)
	}

	saved, err := store.SaveSnapshot(ctx, key, snapshot)
	if err != nil {
		log.Fatalf("salvando snapshot: %v", err)
	}

	loaded, err := store.LoadSnapshot(ctx, key)
	if err != nil {
		log.Fatalf("carregando snapshot: %v", err)
	}

	fmt.Printf("mem: %s/%s\n", loaded.Key.Namespace, loaded.Key.DocumentID)
	fmt.Printf("mem: salvo em %s\n", saved.StoredAt.UTC())
	fmt.Printf("mem: update_v1=%d bytes\n", len(loaded.Snapshot.UpdateV1))
	fmt.Printf("mem: snapshot vazio=%v\n", loaded.Snapshot.IsEmpty())
}
