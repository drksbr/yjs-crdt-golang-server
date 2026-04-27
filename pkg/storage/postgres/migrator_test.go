package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"yjs-go-bridge/pkg/storage"
)

func TestAutoMigrateRequiresInitializedStore(t *testing.T) {
	t.Parallel()

	store := &Store{}
	if err := store.AutoMigrate(context.Background()); !errors.Is(err, errUninitializedStore) {
		t.Fatalf("AutoMigrate() error = %v, want %v", err, errUninitializedStore)
	}
}

func TestAutoMigrateUsesAdvisoryLock(t *testing.T) {
	store, _ := newTestStore(t, true)
	ctx := context.Background()
	migrations, err := loadMigrations(store.schema)
	if err != nil {
		t.Fatalf("loadMigrations() unexpected error: %v", err)
	}

	lockConn, err := pgx.Connect(ctx, getPostgresTestDSN(t))
	if err != nil {
		t.Fatalf("pgx.Connect() unexpected error: %v", err)
	}
	defer func() {
		_ = lockConn.Close(ctx)
	}()

	_, err = lockConn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey)
	if err != nil {
		t.Fatalf("acquiring lock failed: %v", err)
	}

	lockCtx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = store.AutoMigrate(lockCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("AutoMigrate() error = %v, want %v", err, context.DeadlineExceeded)
	}
	if time.Since(start) < 200*time.Millisecond {
		t.Fatalf("AutoMigrate() retornou em %v, esperava aguardar lock de advisory", time.Since(start))
	}

	if _, err := lockConn.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey); err != nil {
		t.Fatalf("release advisory lock failed: %v", err)
	}

	if err := store.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("AutoMigrate() post-lock error = %v", err)
	}

	var count int
	query := fmt.Sprintf("SELECT count(*) FROM %s.schema_migrations", quoteIdentifier(store.schema))
	row := store.pool.QueryRow(ctx, query)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count schema_migrations query error: %v", err)
	}
	if count != len(migrations) {
		t.Fatalf("schema_migrations rows = %d, want %d", count, len(migrations))
	}
}

func TestAutoMigrateIsIdempotent(t *testing.T) {
	store, _ := newTestStore(t, true)
	ctx := context.Background()
	migrations, err := loadMigrations(store.schema)
	if err != nil {
		t.Fatalf("loadMigrations() unexpected error: %v", err)
	}

	queryCount := func() int {
		var count int
		query := fmt.Sprintf("SELECT count(*) FROM %s.schema_migrations", quoteIdentifier(store.schema))
		row := store.pool.QueryRow(ctx, query)
		if err := row.Scan(&count); err != nil {
			t.Fatalf("schema_migrations count() unexpected error: %v", err)
		}
		return count
	}

	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() first call error = %v", err)
	}
	first := queryCount()
	if first != len(migrations) {
		t.Fatalf("schema_migrations rows after first AutoMigrate = %d, want %d", first, len(migrations))
	}

	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() second call error = %v", err)
	}
	second := queryCount()
	if second != len(migrations) {
		t.Fatalf("schema_migrations rows after second AutoMigrate = %d, want %d", second, len(migrations))
	}
}

func TestAutoMigrateUpgradesLeaseGenerationSeedConstraint(t *testing.T) {
	store, _ := newTestStore(t, true)
	ctx := context.Background()
	schema := quoteIdentifier(store.schema)

	if _, err := store.pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		t.Fatalf("create schema unexpected error: %v", err)
	}

	migrations, err := loadMigrations(store.schema)
	if err != nil {
		t.Fatalf("loadMigrations() unexpected error: %v", err)
	}
	if len(migrations) < 4 {
		t.Fatalf("len(migrations) = %d, want at least 4", len(migrations))
	}
	for _, migration := range migrations[:3] {
		if err := store.applyMigration(ctx, schema, migration); err != nil {
			t.Fatalf("applyMigration(%s) unexpected error: %v", migration.name, err)
		}
	}

	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() upgrade unexpected error: %v", err)
	}

	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID:   storage.ShardID("fresh-shard"),
		Owner:     storage.OwnerInfo{NodeID: storage.NodeID("node-a"), Epoch: 1},
		Token:     "lease-a",
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}); err != nil {
		t.Fatalf("SaveLease() after v4 migration unexpected error: %v", err)
	}
}
