package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const postgresTestDSNEnv = "POSTGRES_TEST_DSN"

var testSchemaSeq uint64

func getPostgresTestDSN(t *testing.T) string {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv(postgresTestDSNEnv))
	if dsn == "" {
		t.Skipf("teste de integração postgres ignorado: defina %s", postgresTestDSNEnv)
	}
	return dsn
}

func newTestSchema(prefix string) string {
	seq := atomic.AddUint64(&testSchemaSeq, 1)
	name := fmt.Sprintf("%s_%d", strings.ToLower(prefix), seq)
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return "yjs_bridge_" + name
}

func newTestStore(t *testing.T, skipMigrations bool) (*Store, string) {
	t.Helper()

	schema := newTestSchema(t.Name())
	cfg := Config{
		ConnectionString: getPostgresTestDSN(t),
		Schema:           schema,
		SkipMigrations:   skipMigrations,
	}
	store, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if store != nil && store.pool != nil {
			_, _ = store.pool.Exec(cleanupCtx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", quoteIdentifier(schema)))
			store.Close()
		}
	})

	return store, schema
}
