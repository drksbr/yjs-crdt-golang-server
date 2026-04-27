package postgres

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"yjs-go-bridge/pkg/storage"
)

var _ storage.DistributedStore = (*Store)(nil)

func (s *Store) AppendUpdate(ctx context.Context, key storage.DocumentKey, update []byte) (*storage.UpdateLogRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if len(update) == 0 {
		return nil, storage.ErrInvalidUpdatePayload
	}
	pool, err := s.requirePool()
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
WITH allocated AS (
	INSERT INTO %s.document_update_log_heads AS heads (namespace, document_id, next_offset)
	VALUES ($1, $2, 2)
	ON CONFLICT (namespace, document_id)
	DO UPDATE SET next_offset = heads.next_offset + 1
	RETURNING next_offset - 1 AS log_offset
)
INSERT INTO %s.document_update_logs(namespace, document_id, log_offset, update_v1, stored_at)
SELECT $1, $2, allocated.log_offset, $3, now()
FROM allocated
RETURNING log_offset, stored_at
`, quoteIdentifier(s.schema), quoteIdentifier(s.schema))

	var offset int64
	var storedAt time.Time
	if err := pool.QueryRow(ctx, query, key.Namespace, key.DocumentID, update).Scan(&offset, &storedAt); err != nil {
		return nil, err
	}

	logOffset, err := int64ToOffset(offset)
	if err != nil {
		return nil, err
	}

	return &storage.UpdateLogRecord{
		Key:      key,
		Offset:   logOffset,
		UpdateV1: append([]byte(nil), update...),
		StoredAt: storedAt,
	}, nil
}

func (s *Store) ListUpdates(ctx context.Context, key storage.DocumentKey, after storage.UpdateOffset, limit int) ([]*storage.UpdateLogRecord, error) {
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

	afterOffset, err := offsetToInt64(after)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
SELECT log_offset, update_v1, stored_at
FROM %s.document_update_logs
WHERE namespace = $1 AND document_id = $2 AND log_offset > $3
ORDER BY log_offset ASC
`, quoteIdentifier(s.schema))

	args := []any{key.Namespace, key.DocumentID, afterOffset}
	if limit > 0 {
		query += "LIMIT $4"
		args = append(args, limit)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*storage.UpdateLogRecord
	for rows.Next() {
		var offset int64
		var payload []byte
		var storedAt time.Time
		if err := rows.Scan(&offset, &payload, &storedAt); err != nil {
			return nil, err
		}

		logOffset, err := int64ToOffset(offset)
		if err != nil {
			return nil, err
		}

		records = append(records, &storage.UpdateLogRecord{
			Key:      key,
			Offset:   logOffset,
			UpdateV1: append([]byte(nil), payload...),
			StoredAt: storedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) TrimUpdates(ctx context.Context, key storage.DocumentKey, through storage.UpdateOffset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := key.Validate(); err != nil {
		return err
	}
	pool, err := s.requirePool()
	if err != nil {
		return err
	}

	throughOffset, err := offsetToInt64(through)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
DELETE FROM %s.document_update_logs
WHERE namespace = $1 AND document_id = $2 AND log_offset <= $3
`, quoteIdentifier(s.schema))
	_, err = pool.Exec(ctx, query, key.Namespace, key.DocumentID, throughOffset)
	return err
}

func (s *Store) SavePlacement(ctx context.Context, placement storage.PlacementRecord) (*storage.PlacementRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := placement.Validate(); err != nil {
		return nil, err
	}
	pool, err := s.requirePool()
	if err != nil {
		return nil, err
	}

	version, err := uint64ToInt64("version", placement.Version)
	if err != nil {
		return nil, err
	}
	updatedAt := normalizeTime(placement.UpdatedAt)

	query := fmt.Sprintf(`
INSERT INTO %s.document_placements(namespace, document_id, shard_id, version, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (namespace, document_id)
DO UPDATE SET shard_id = EXCLUDED.shard_id, version = EXCLUDED.version, updated_at = EXCLUDED.updated_at
RETURNING version, updated_at
`, quoteIdentifier(s.schema))

	var storedVersion int64
	var storedAt time.Time
	if err := pool.QueryRow(ctx, query, placement.Key.Namespace, placement.Key.DocumentID, placement.ShardID, version, updatedAt).Scan(&storedVersion, &storedAt); err != nil {
		return nil, err
	}

	normalizedVersion, err := int64ToUint64("version", storedVersion)
	if err != nil {
		return nil, err
	}

	return &storage.PlacementRecord{
		Key:       placement.Key,
		ShardID:   placement.ShardID,
		Version:   normalizedVersion,
		UpdatedAt: storedAt,
	}, nil
}

func (s *Store) LoadPlacement(ctx context.Context, key storage.DocumentKey) (*storage.PlacementRecord, error) {
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
SELECT shard_id, version, updated_at
FROM %s.document_placements
WHERE namespace = $1 AND document_id = $2
`, quoteIdentifier(s.schema))

	var shardID string
	var version int64
	var updatedAt time.Time
	if err := pool.QueryRow(ctx, query, key.Namespace, key.DocumentID).Scan(&shardID, &version, &updatedAt); err != nil {
		if isNoRows(err) {
			return nil, storage.ErrPlacementNotFound
		}
		return nil, err
	}

	normalizedVersion, err := int64ToUint64("version", version)
	if err != nil {
		return nil, err
	}

	return &storage.PlacementRecord{
		Key:       key,
		ShardID:   storage.ShardID(shardID),
		Version:   normalizedVersion,
		UpdatedAt: updatedAt,
	}, nil
}

func (s *Store) SaveLease(ctx context.Context, lease storage.LeaseRecord) (*storage.LeaseRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := lease.Validate(); err != nil {
		return nil, err
	}
	pool, err := s.requirePool()
	if err != nil {
		return nil, err
	}

	epoch, err := uint64ToInt64("owner epoch", lease.Owner.Epoch)
	if err != nil {
		return nil, err
	}
	acquiredAt := normalizeTime(lease.AcquiredAt)

	query := fmt.Sprintf(`
INSERT INTO %s.shard_leases(shard_id, owner_node_id, owner_epoch, token, acquired_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (shard_id)
DO UPDATE SET owner_node_id = EXCLUDED.owner_node_id, owner_epoch = EXCLUDED.owner_epoch, token = EXCLUDED.token, acquired_at = EXCLUDED.acquired_at, expires_at = EXCLUDED.expires_at
RETURNING owner_epoch, acquired_at, expires_at
`, quoteIdentifier(s.schema))

	var storedEpoch int64
	var storedAcquiredAt time.Time
	var storedExpiresAt time.Time
	if err := pool.QueryRow(ctx, query, lease.ShardID, lease.Owner.NodeID, epoch, lease.Token, acquiredAt, lease.ExpiresAt).Scan(&storedEpoch, &storedAcquiredAt, &storedExpiresAt); err != nil {
		return nil, err
	}

	normalizedEpoch, err := int64ToUint64("owner epoch", storedEpoch)
	if err != nil {
		return nil, err
	}

	return &storage.LeaseRecord{
		ShardID: lease.ShardID,
		Owner: storage.OwnerInfo{
			NodeID: lease.Owner.NodeID,
			Epoch:  normalizedEpoch,
		},
		Token:      lease.Token,
		AcquiredAt: storedAcquiredAt,
		ExpiresAt:  storedExpiresAt,
	}, nil
}

func (s *Store) LoadLease(ctx context.Context, shardID storage.ShardID) (*storage.LeaseRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := shardID.Validate(); err != nil {
		return nil, err
	}
	pool, err := s.requirePool()
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
SELECT owner_node_id, owner_epoch, token, acquired_at, expires_at
FROM %s.shard_leases
WHERE shard_id = $1
`, quoteIdentifier(s.schema))

	var nodeID string
	var epoch int64
	var token string
	var acquiredAt time.Time
	var expiresAt time.Time
	if err := pool.QueryRow(ctx, query, shardID).Scan(&nodeID, &epoch, &token, &acquiredAt, &expiresAt); err != nil {
		if isNoRows(err) {
			return nil, storage.ErrLeaseNotFound
		}
		return nil, err
	}

	normalizedEpoch, err := int64ToUint64("owner epoch", epoch)
	if err != nil {
		return nil, err
	}

	return &storage.LeaseRecord{
		ShardID: shardID,
		Owner: storage.OwnerInfo{
			NodeID: storage.NodeID(nodeID),
			Epoch:  normalizedEpoch,
		},
		Token:      token,
		AcquiredAt: acquiredAt,
		ExpiresAt:  expiresAt,
	}, nil
}

func (s *Store) ReleaseLease(ctx context.Context, shardID storage.ShardID, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := shardID.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("%w: token obrigatorio", storage.ErrInvalidLeaseToken)
	}
	pool, err := s.requirePool()
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`DELETE FROM %s.shard_leases WHERE shard_id = $1 AND token = $2`, quoteIdentifier(s.schema))
	tag, err := pool.Exec(ctx, query, shardID, token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrLeaseNotFound
	}
	return nil
}

func normalizeTime(ts time.Time) time.Time {
	if ts.IsZero() {
		return time.Now().UTC()
	}
	return ts.UTC()
}

func offsetToInt64(offset storage.UpdateOffset) (int64, error) {
	return uint64ToInt64("update offset", uint64(offset))
}

func int64ToOffset(offset int64) (storage.UpdateOffset, error) {
	value, err := int64ToUint64("update offset", offset)
	if err != nil {
		return 0, err
	}
	return storage.UpdateOffset(value), nil
}

func uint64ToInt64(name string, value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("postgres: %s fora do intervalo bigint: %d", name, value)
	}
	return int64(value), nil
}

func int64ToUint64(name string, value int64) (uint64, error) {
	if value < 0 {
		return 0, fmt.Errorf("postgres: %s negativo no banco: %d", name, value)
	}
	return uint64(value), nil
}
