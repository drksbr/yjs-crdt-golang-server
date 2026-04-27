package memory

import (
	"context"
	"fmt"
	"strings"

	"yjs-go-bridge/pkg/storage"
)

var (
	_ storage.UpdateLogStore   = (*Store)(nil)
	_ storage.PlacementStore   = (*Store)(nil)
	_ storage.LeaseStore       = (*Store)(nil)
	_ storage.DistributedStore = (*Store)(nil)
)

// AppendUpdate adiciona um update V1 ao fim do log incremental do documento.
func (s *Store) AppendUpdate(ctx context.Context, key storage.DocumentKey, update []byte) (*storage.UpdateLogRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errNilStore
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if len(update) == 0 {
		return nil, fmt.Errorf("%w: updateV1 obrigatorio", storage.ErrInvalidUpdatePayload)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureStoreInitializedLocked()

	offset := s.updateNext[key] + 1
	record := &storage.UpdateLogRecord{
		Key:      key,
		Offset:   offset,
		UpdateV1: append([]byte(nil), update...),
		StoredAt: s.nowTime(),
	}
	s.updateLogs[key] = append(s.updateLogs[key], record)
	s.updateNext[key] = offset
	return record.Clone(), nil
}

// ListUpdates lista updates com offset estritamente maior que after.
func (s *Store) ListUpdates(ctx context.Context, key storage.DocumentKey, after storage.UpdateOffset, limit int) ([]*storage.UpdateLogRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errNilStore
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.updateLogs[key]
	if len(records) == 0 {
		return nil, nil
	}

	maxResults := len(records)
	if limit > 0 && limit < maxResults {
		maxResults = limit
	}
	result := make([]*storage.UpdateLogRecord, 0, maxResults)
	for _, record := range records {
		if record.Offset <= after {
			continue
		}
		result = append(result, record.Clone())
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

// TrimUpdates remove registros com offset menor ou igual ao limite informado.
func (s *Store) TrimUpdates(ctx context.Context, key storage.DocumentKey, through storage.UpdateOffset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return errNilStore
	}
	if err := key.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureStoreInitializedLocked()

	records := s.updateLogs[key]
	if len(records) == 0 {
		return nil
	}

	firstRemaining := 0
	for firstRemaining < len(records) && records[firstRemaining].Offset <= through {
		firstRemaining++
	}

	switch {
	case firstRemaining == 0:
		return nil
	case firstRemaining >= len(records):
		delete(s.updateLogs, key)
		return nil
	default:
		trimmed := make([]*storage.UpdateLogRecord, len(records)-firstRemaining)
		copy(trimmed, records[firstRemaining:])
		s.updateLogs[key] = trimmed
		return nil
	}
}

// SavePlacement grava ou substitui o placement lógico do documento.
func (s *Store) SavePlacement(ctx context.Context, placement storage.PlacementRecord) (*storage.PlacementRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errNilStore
	}

	record := placement.Clone()
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = s.nowTime()
	}
	if err := record.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureStoreInitializedLocked()
	s.placements[record.Key] = record
	return record.Clone(), nil
}

// LoadPlacement carrega o placement atual associado ao documento.
func (s *Store) LoadPlacement(ctx context.Context, key storage.DocumentKey) (*storage.PlacementRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errNilStore
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.placements[key]
	if !ok {
		return nil, storage.ErrPlacementNotFound
	}
	return record.Clone(), nil
}

// SaveLease grava ou renova a lease atual de ownership para o shard.
func (s *Store) SaveLease(ctx context.Context, lease storage.LeaseRecord) (*storage.LeaseRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errNilStore
	}

	record := lease.Clone()
	if record.AcquiredAt.IsZero() {
		record.AcquiredAt = s.nowTime()
	}
	if err := record.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureStoreInitializedLocked()
	s.leases[record.ShardID] = record
	return record.Clone(), nil
}

// LoadLease carrega a lease atual de ownership do shard.
func (s *Store) LoadLease(ctx context.Context, shardID storage.ShardID) (*storage.LeaseRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errNilStore
	}
	if err := shardID.Validate(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.leases[shardID]
	if !ok {
		return nil, storage.ErrLeaseNotFound
	}
	return record.Clone(), nil
}

// ReleaseLease remove a lease atual quando o token informado corresponde ao owner persistido.
func (s *Store) ReleaseLease(ctx context.Context, shardID storage.ShardID, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return errNilStore
	}
	if err := shardID.Validate(); err != nil {
		return err
	}
	if err := validateLeaseToken(token); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.leases[shardID]
	if !ok || record.Token != token {
		return storage.ErrLeaseNotFound
	}

	delete(s.leases, shardID)
	return nil
}

func validateLeaseToken(token string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("%w: token obrigatorio", storage.ErrInvalidLeaseToken)
	}
	return nil
}
