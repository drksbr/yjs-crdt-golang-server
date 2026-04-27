package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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
	if count != 1 {
		t.Fatalf("schema_migrations rows = %d, want 1", count)
	}
}

func TestAutoMigrateIsIdempotent(t *testing.T) {
	store, _ := newTestStore(t, true)
	ctx := context.Background()

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
	if first != 1 {
		t.Fatalf("schema_migrations rows after first AutoMigrate = %d, want 1", first)
	}

	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() second call error = %v", err)
	}
	second := queryCount()
	if second != 1 {
		t.Fatalf("schema_migrations rows after second AutoMigrate = %d, want 1", second)
	}
}
