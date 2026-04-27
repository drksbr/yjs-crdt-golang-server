package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"yjs-go-bridge/pkg/storage"
	pgstore "yjs-go-bridge/pkg/storage/postgres"
	"yjs-go-bridge/pkg/yjsbridge"
)

func main() {
	dsn := strings.TrimSpace(os.Getenv("YJSBRIDGE_POSTGRES_DSN"))
	if dsn == "" {
		log.Fatal("defina YJSBRIDGE_POSTGRES_DSN para executar este exemplo")
	}

	ctx := context.Background()
	store, err := pgstore.New(ctx, pgstore.Config{
		ConnectionString: dsn,
		Schema:           "yjs_bridge_example",
	})
	if err != nil {
		log.Fatalf("abrindo store postgres: %v", err)
	}
	defer store.Close()

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

	fmt.Printf("pg: %s/%s\n", loaded.Key.Namespace, loaded.Key.DocumentID)
	fmt.Printf("pg: salvo em %s\n", saved.StoredAt.UTC())
	fmt.Printf("pg: update_v1=%d bytes\n", len(loaded.Snapshot.UpdateV1))
	fmt.Printf("pg: snapshot vazio=%v\n", loaded.Snapshot.IsEmpty())
}
