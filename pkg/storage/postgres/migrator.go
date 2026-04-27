package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const migrationLockKey int64 = 0x796a7362

// AutoMigrate aplica migrations SQL explícitas do pacote.
func (s *Store) AutoMigrate(ctx context.Context) error {
	pool, err := s.requirePool()
	if err != nil {
		return err
	}

	schema := quoteIdentifier(s.schema)

	if _, err := pool.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return err
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()

	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.schema_migrations (
    version bigint PRIMARY KEY,
    name text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
)`, schema)); err != nil {
		return err
	}

	applied, err := s.appliedMigrationVersions(ctx, schema)
	if err != nil {
		return err
	}

	migrations, err := loadMigrations(s.schema)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if applied[migration.version] {
			continue
		}
		if err := s.applyMigration(ctx, schema, migration); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) appliedMigrationVersions(ctx context.Context, schema string) (map[int]bool, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf("SELECT version FROM %s.schema_migrations", schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func (s *Store) applyMigration(ctx context.Context, schema string, migration migration) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(context.Background())
		}
	}()

	if _, err = tx.Exec(ctx, migration.sql); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		fmt.Sprintf("INSERT INTO %s.schema_migrations(version, name) VALUES ($1, $2)", schema),
		migration.version,
		migration.name,
	); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil && err != pgx.ErrTxClosed {
		return err
	}
	return nil
}
