package postgres

import (
	"strings"
	"testing"
)

func TestLoadMigrations(t *testing.T) {
	t.Parallel()

	migrations, err := loadMigrations("tenant_app")
	if err != nil {
		t.Fatalf("loadMigrations() unexpected error: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("loadMigrations() returned no migrations")
	}
	if migrations[0].version != 1 {
		t.Fatalf("migrations[0].version = %d, want 1", migrations[0].version)
	}
	if migrations[len(migrations)-1].version < 4 {
		t.Fatalf("last migration version = %d, want at least 4", migrations[len(migrations)-1].version)
	}
	if !strings.Contains(migrations[0].sql, `"tenant_app".document_snapshots`) {
		t.Fatalf("migration sql = %q, want quoted schema substitution", migrations[0].sql)
	}

	var foundGenerationTable bool
	var foundZeroSeed bool
	for _, migration := range migrations {
		if migration.version == 3 &&
			strings.Contains(migration.sql, `"tenant_app".shard_lease_generations`) &&
			strings.Contains(migration.sql, `FROM "tenant_app".shard_leases`) {
			foundGenerationTable = true
		}
		if migration.version == 4 && strings.Contains(migration.sql, "last_epoch >= 0") {
			foundZeroSeed = true
		}
	}
	if !foundGenerationTable {
		t.Fatal("migration version 3 does not define/backfill shard_lease_generations")
	}
	if !foundZeroSeed {
		t.Fatal("migration version 4 does not relax shard_lease_generations last_epoch for zero-seed locking")
	}
}
