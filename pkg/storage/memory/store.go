package memory

import (
	"context"
	"sync"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

// Store mantém snapshots persistidos apenas em memória.
type Store struct {
	mu         sync.RWMutex
	now        func() time.Time
	items      map[storage.DocumentKey]*storage.SnapshotRecord
	updateLogs map[storage.DocumentKey][]*storage.UpdateLogRecord
	updateNext map[storage.DocumentKey]storage.UpdateOffset
	placements map[storage.DocumentKey]*storage.PlacementRecord
	leases     map[storage.ShardID]*storage.LeaseRecord
	leaseLast  map[storage.ShardID]uint64
}

var _ storage.SnapshotStore = (*Store)(nil)
var _ storage.SnapshotCheckpointStore = (*Store)(nil)
var _ storage.SnapshotCheckpointEpochStore = (*Store)(nil)
var _ storage.AuthoritativeSnapshotStore = (*Store)(nil)
var _ storage.AuthoritativeSnapshotCheckpointStore = (*Store)(nil)

// New cria um store em memória pronto para uso.
func New() *Store {
	return &Store{
		items:      make(map[storage.DocumentKey]*storage.SnapshotRecord),
		updateLogs: make(map[storage.DocumentKey][]*storage.UpdateLogRecord),
		updateNext: make(map[storage.DocumentKey]storage.UpdateOffset),
		placements: make(map[storage.DocumentKey]*storage.PlacementRecord),
		leases:     make(map[storage.ShardID]*storage.LeaseRecord),
		leaseLast:  make(map[storage.ShardID]uint64),
		now:        func() time.Time { return time.Now().UTC() },
	}
}

// SaveSnapshot grava ou substitui o snapshot associado à chave.
func (s *Store) SaveSnapshot(ctx context.Context, key storage.DocumentKey, snapshot *yjsbridge.PersistedSnapshot) (*storage.SnapshotRecord, error) {
	return s.SaveSnapshotCheckpoint(ctx, key, snapshot, 0)
}

// SaveSnapshotCheckpoint grava ou substitui o snapshot associado à chave,
// persistindo o high-water mark que ele já cobre.
func (s *Store) SaveSnapshotCheckpoint(ctx context.Context, key storage.DocumentKey, snapshot *yjsbridge.PersistedSnapshot, through storage.UpdateOffset) (*storage.SnapshotRecord, error) {
	return s.SaveSnapshotCheckpointEpoch(ctx, key, snapshot, through, 0)
}

// SaveSnapshotCheckpointEpoch grava ou substitui o snapshot associado à chave,
// persistindo o high-water mark e o epoch observados no checkpoint.
func (s *Store) SaveSnapshotCheckpointEpoch(ctx context.Context, key storage.DocumentKey, snapshot *yjsbridge.PersistedSnapshot, through storage.UpdateOffset, epoch uint64) (*storage.SnapshotRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errNilStore
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, storage.ErrNilPersistedSnapshot
	}

	record := &storage.SnapshotRecord{
		Key:      key,
		Snapshot: snapshot.Clone(),
		Through:  through,
		Epoch:    epoch,
		StoredAt: s.nowTime(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureStoreInitializedLocked()
	s.items[key] = record
	return record.Clone(), nil
}

// SaveSnapshotAuthoritative grava ou substitui o snapshot associado à chave,
// exigindo que o placement + lease persistidos ainda correspondam ao fence.
func (s *Store) SaveSnapshotAuthoritative(
	ctx context.Context,
	key storage.DocumentKey,
	snapshot *yjsbridge.PersistedSnapshot,
	fence storage.AuthorityFence,
) (*storage.SnapshotRecord, error) {
	return s.SaveSnapshotCheckpointAuthoritative(ctx, key, snapshot, 0, fence)
}

// SaveSnapshotCheckpointAuthoritative grava ou substitui o snapshot associado à
// chave, persistindo o checkpoint `through` e exigindo fencing autoritativo.
func (s *Store) SaveSnapshotCheckpointAuthoritative(
	ctx context.Context,
	key storage.DocumentKey,
	snapshot *yjsbridge.PersistedSnapshot,
	through storage.UpdateOffset,
	fence storage.AuthorityFence,
) (*storage.SnapshotRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errNilStore
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, storage.ErrNilPersistedSnapshot
	}
	if err := fence.Validate(); err != nil {
		return nil, err
	}

	record := &storage.SnapshotRecord{
		Key:      key,
		Snapshot: snapshot.Clone(),
		Through:  through,
		Epoch:    fence.Owner.Epoch,
		StoredAt: s.nowTime(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureStoreInitializedLocked()
	if err := s.validateAuthorityLocked(key, fence, record.StoredAt); err != nil {
		return nil, err
	}
	s.items[key] = record
	return record.Clone(), nil
}

// LoadSnapshot carrega o snapshot atual associado à chave.
func (s *Store) LoadSnapshot(ctx context.Context, key storage.DocumentKey) (*storage.SnapshotRecord, error) {
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

	record, ok := s.items[key]
	if !ok {
		return nil, storage.ErrSnapshotNotFound
	}
	return record.Clone(), nil
}

func (s *Store) ensureStoreInitializedLocked() {
	if s.items == nil {
		s.items = make(map[storage.DocumentKey]*storage.SnapshotRecord)
	}
	if s.updateLogs == nil {
		s.updateLogs = make(map[storage.DocumentKey][]*storage.UpdateLogRecord)
	}
	if s.updateNext == nil {
		s.updateNext = make(map[storage.DocumentKey]storage.UpdateOffset)
	}
	if s.placements == nil {
		s.placements = make(map[storage.DocumentKey]*storage.PlacementRecord)
	}
	if s.leases == nil {
		s.leases = make(map[storage.ShardID]*storage.LeaseRecord)
	}
	if s.leaseLast == nil {
		s.leaseLast = make(map[storage.ShardID]uint64)
	}
	if s.now == nil {
		s.now = func() time.Time { return time.Now().UTC() }
	}
}

func (s *Store) nowTime() time.Time {
	if s == nil {
		return time.Now().UTC()
	}
	if s.now != nil {
		return s.now()
	}
	return time.Now().UTC()
}
