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
	if migrations[len(migrations)-1].version != 2 {
		t.Fatalf("last migration version = %d, want 2", migrations[len(migrations)-1].version)
	}
	if !strings.Contains(migrations[0].sql, `"tenant_app".document_snapshots`) {
		t.Fatalf("migration sql = %q, want quoted schema substitution", migrations[0].sql)
	}
}
