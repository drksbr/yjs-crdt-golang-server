package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/yjsbridge"
)

var (
	errUninitializedStore = errors.New("postgres: store nao inicializado")
)

// Store persiste snapshots em PostgreSQL.
type Store struct {
	pool   *pgxpool.Pool
	schema string
}

var _ storage.SnapshotStore = (*Store)(nil)

// New cria um store PostgreSQL e executa automigration por padrão.
func New(ctx context.Context, cfg Config) (*Store, error) {
	cfg = cfg.normalized()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString)
	if err != nil {
		return nil, err
	}
	poolConfig.ConnConfig.RuntimeParams["application_name"] = cfg.ApplicationName
	if cfg.MinConns > 0 {
		poolConfig.MinConns = cfg.MinConns
	}
	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = cfg.MaxConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}

	store := &Store{
		pool:   pool,
		schema: cfg.Schema,
	}
	if !cfg.SkipMigrations {
		if err := store.AutoMigrate(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return store, nil
}

// Close libera o pool de conexões.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) requirePool() (*pgxpool.Pool, error) {
	if s == nil || s.pool == nil {
		return nil, errUninitializedStore
	}
	return s.pool, nil
}

// SaveSnapshot grava ou substitui o snapshot de um documento.
func (s *Store) SaveSnapshot(ctx context.Context, key storage.DocumentKey, snapshot *yjsbridge.PersistedSnapshot) (*storage.SnapshotRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, storage.ErrNilPersistedSnapshot
	}
	pool, err := s.requirePool()
	if err != nil {
		return nil, err
	}

	payload, err := yjsbridge.EncodePersistedSnapshotV1(snapshot)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
INSERT INTO %s.document_snapshots(namespace, document_id, snapshot_v1, stored_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (namespace, document_id)
DO UPDATE SET snapshot_v1 = EXCLUDED.snapshot_v1, stored_at = now()
RETURNING stored_at
`, quoteIdentifier(s.schema))

	var storedAt time.Time
	if err := pool.QueryRow(ctx, query, key.Namespace, key.DocumentID, payload).Scan(&storedAt); err != nil {
		return nil, err
	}

	return &storage.SnapshotRecord{
		Key:      key,
		Snapshot: snapshot.Clone(),
		StoredAt: storedAt,
	}, nil
}

// LoadSnapshot carrega o snapshot persistido do documento.
func (s *Store) LoadSnapshot(ctx context.Context, key storage.DocumentKey) (*storage.SnapshotRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	pool, err := s.requirePool()
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
SELECT snapshot_v1, stored_at
FROM %s.document_snapshots
WHERE namespace = $1 AND document_id = $2
`, quoteIdentifier(s.schema))

	var payload []byte
	var storedAt time.Time
	if err := pool.QueryRow(ctx, query, key.Namespace, key.DocumentID).Scan(&payload, &storedAt); err != nil {
		if isNoRows(err) {
			return nil, storage.ErrSnapshotNotFound
		}
		return nil, err
	}

	snapshot, err := yjsbridge.DecodePersistedSnapshotV1(payload)
	if err != nil {
		return nil, err
	}

	return &storage.SnapshotRecord{
		Key:      key,
		Snapshot: snapshot,
		StoredAt: storedAt,
	}, nil
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
