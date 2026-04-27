package ycluster

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"yjs-go-bridge/pkg/storage"
)

var (
	_ OwnerLookup = (*StorageOwnerLookup)(nil)
	_ LeaseStore  = (*StorageLeaseStore)(nil)
)

// StorageShardID converte um shard do control plane para o formato persistido.
func StorageShardID(id ShardID) storage.ShardID {
	return storage.ShardID(id.String())
}

// StorageNodeID converte um node id do control plane para o formato persistido.
func StorageNodeID(id NodeID) storage.NodeID {
	return storage.NodeID(id.String())
}

// ParseStorageShardID converte o shard persistido para o identificador do
// control plane, assumindo representação decimal estável.
func ParseStorageShardID(id storage.ShardID) (ShardID, error) {
	if err := id.Validate(); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidPlacement, err)
	}

	value, err := strconv.ParseUint(string(id), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: shard storage %q invalido", ErrInvalidPlacement, id)
	}
	return ShardID(value), nil
}

// ParseStorageNodeID converte o node id persistido para o identificador do
// control plane.
func ParseStorageNodeID(id storage.NodeID) (NodeID, error) {
	nodeID := NodeID(id)
	if err := nodeID.Validate(); err != nil {
		return "", err
	}
	return nodeID, nil
}

// LeaseFromStorageRecord converte uma lease persistida no contrato do control
// plane.
func LeaseFromStorageRecord(record *storage.LeaseRecord) (*Lease, error) {
	if record == nil {
		return nil, fmt.Errorf("%w: lease storage nil", ErrInvalidLease)
	}
	if err := record.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLease, err)
	}

	shardID, err := ParseStorageShardID(record.ShardID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLease, err)
	}
	holder, err := ParseStorageNodeID(record.Owner.NodeID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLease, err)
	}

	lease := &Lease{
		ShardID:    shardID,
		Holder:     holder,
		Epoch:      record.Owner.Epoch,
		Token:      record.Token,
		AcquiredAt: record.AcquiredAt,
		ExpiresAt:  record.ExpiresAt,
	}
	if err := lease.Validate(); err != nil {
		return nil, err
	}
	return lease, nil
}

// StorageOwnerLookup resolve ownership usando `storage.PlacementStore` para o
// mapeamento documento -> shard e `storage.LeaseStore` para o owner corrente do
// shard.
type StorageOwnerLookup struct {
	localNodeID NodeID
	resolver    ShardResolver
	placements  storage.PlacementStore
	leases      storage.LeaseStore
	now         func() time.Time
}

// NewStorageOwnerLookup constrói um lookup de owner sobre contratos de storage.
func NewStorageOwnerLookup(
	localNode NodeID,
	resolver ShardResolver,
	placements storage.PlacementStore,
	leases storage.LeaseStore,
) (*StorageOwnerLookup, error) {
	if err := localNode.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNilLocalNode, err)
	}
	if resolver == nil {
		return nil, ErrNilShardResolver
	}
	if placements == nil {
		return nil, ErrNilPlacementStore
	}
	if leases == nil {
		return nil, ErrNilLeaseStore
	}

	return &StorageOwnerLookup{
		localNodeID: localNode,
		resolver:    resolver,
		placements:  placements,
		leases:      leases,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}, nil
}

// LookupOwner resolve o owner ativo do documento a partir do placement do
// documento e da lease corrente do shard correspondente.
func (l *StorageOwnerLookup) LookupOwner(ctx context.Context, req OwnerLookupRequest) (*OwnerResolution, error) {
	if l == nil {
		return nil, ErrOwnerNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	shardID, err := l.resolver.ResolveShard(req.DocumentKey)
	if err != nil {
		return nil, err
	}

	placementRecord, err := l.placements.LoadPlacement(ctx, req.DocumentKey)
	if err != nil {
		if errors.Is(err, storage.ErrPlacementNotFound) {
			return nil, ErrPlacementNotFound
		}
		return nil, err
	}
	if placementRecord == nil {
		return nil, ErrPlacementNotFound
	}
	if err := placementRecord.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPlacement, err)
	}
	expectedStorageShardID := StorageShardID(shardID)
	if placementRecord.ShardID != expectedStorageShardID {
		return nil, fmt.Errorf("%w: shard %q != %q", ErrInvalidPlacement, placementRecord.ShardID, expectedStorageShardID)
	}

	leaseRecord, err := l.leases.LoadLease(ctx, placementRecord.ShardID)
	if err != nil {
		if errors.Is(err, storage.ErrLeaseNotFound) {
			return nil, ErrOwnerNotFound
		}
		return nil, err
	}
	lease, err := LeaseFromStorageRecord(leaseRecord)
	if err != nil {
		return nil, err
	}
	if lease.ShardID != shardID {
		return nil, fmt.Errorf("%w: lease shard %s != %s", ErrInvalidLease, lease.ShardID, shardID)
	}
	if lease.ExpiredAt(l.nowTime()) {
		return nil, ErrLeaseExpired
	}

	placement := Placement{
		ShardID: shardID,
		NodeID:  lease.Holder,
		Lease:   lease,
		Version: placementRecord.Version,
	}
	if err := placement.Validate(); err != nil {
		return nil, err
	}

	return &OwnerResolution{
		DocumentKey: req.DocumentKey,
		Placement:   placement,
		Local:       lease.Holder == l.localNodeID,
	}, nil
}

func (l *StorageOwnerLookup) nowTime() time.Time {
	if l == nil || l.now == nil {
		return time.Now().UTC()
	}
	return l.now()
}

// StorageLeaseStore adapta um `storage.LeaseStore` ao contrato de leases do
// control plane.
type StorageLeaseStore struct {
	store storage.LeaseStore
	now   func() time.Time
}

// NewStorageLeaseStore constrói um adapter de lease store sobre `pkg/storage`.
func NewStorageLeaseStore(store storage.LeaseStore) (*StorageLeaseStore, error) {
	if store == nil {
		return nil, ErrNilLeaseStore
	}
	return &StorageLeaseStore{
		store: store,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}, nil
}

// AcquireLease tenta adquirir ou substituir a lease persistida do shard.
func (s *StorageLeaseStore) AcquireLease(ctx context.Context, req LeaseRequest) (*Lease, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilLeaseStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := s.nowTime()
	token := strings.TrimSpace(req.Token)
	if token == "" {
		token = fmt.Sprintf("%s-%s-%d", req.Holder, req.ShardID, now.UnixNano())
	}
	epoch := uint64(1)

	existing, err := s.store.LoadLease(ctx, StorageShardID(req.ShardID))
	switch {
	case err == nil:
		if existing == nil {
			return nil, ErrOwnerNotFound
		}
		if existing.Owner.NodeID == StorageNodeID(req.Holder) && !existing.ExpiresAt.IsZero() && now.Before(existing.ExpiresAt) {
			return nil, fmt.Errorf("%w: shard %s ja esta leased para %q", ErrInvalidLeaseRequest, req.ShardID, req.Holder)
		}
		if !existing.ExpiresAt.IsZero() && now.Before(existing.ExpiresAt) {
			return nil, fmt.Errorf("%w: shard %s esta leased para %q ate %s", ErrLeaseHeld, req.ShardID, existing.Owner.NodeID, existing.ExpiresAt.UTC().Format(time.RFC3339Nano))
		}
		if existing.Owner.Epoch > 0 {
			epoch = existing.Owner.Epoch + 1
		}
	case errors.Is(err, storage.ErrLeaseNotFound):
		// first acquire starts at epoch 1
	default:
		return nil, err
	}

	record, err := s.store.SaveLease(ctx, storage.LeaseRecord{
		ShardID: StorageShardID(req.ShardID),
		Owner: storage.OwnerInfo{
			NodeID: StorageNodeID(req.Holder),
			Epoch:  epoch,
		},
		Token:      token,
		AcquiredAt: now,
		ExpiresAt:  now.Add(req.TTL),
	})
	if err != nil {
		return nil, err
	}
	return LeaseFromStorageRecord(record)
}

// RenewLease renova a lease atual do shard, reaproveitando o token existente
// quando a request não explicitar um token.
func (s *StorageLeaseStore) RenewLease(ctx context.Context, req LeaseRequest) (*Lease, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilLeaseStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	existing, err := s.store.LoadLease(ctx, StorageShardID(req.ShardID))
	if err != nil {
		if errors.Is(err, storage.ErrLeaseNotFound) {
			return nil, ErrOwnerNotFound
		}
		return nil, err
	}
	if existing == nil {
		return nil, ErrOwnerNotFound
	}
	if existing.Owner.NodeID != StorageNodeID(req.Holder) {
		return nil, fmt.Errorf("%w: holder %q nao corresponde ao owner persistido %q", ErrInvalidLeaseRequest, req.Holder, existing.Owner.NodeID)
	}

	now := s.nowTime()
	if !existing.ExpiresAt.IsZero() && !now.Before(existing.ExpiresAt) {
		return nil, ErrLeaseExpired
	}
	token := strings.TrimSpace(req.Token)
	switch {
	case token == "":
		token = existing.Token
	case token != existing.Token:
		return nil, ErrLeaseTokenMismatch
	}
	record, err := s.store.SaveLease(ctx, storage.LeaseRecord{
		ShardID: StorageShardID(req.ShardID),
		Owner: storage.OwnerInfo{
			NodeID: existing.Owner.NodeID,
			Epoch:  existing.Owner.Epoch,
		},
		Token:      token,
		AcquiredAt: existing.AcquiredAt,
		ExpiresAt:  now.Add(req.TTL),
	})
	if err != nil {
		return nil, err
	}
	return LeaseFromStorageRecord(record)
}

// ReleaseLease libera explicitamente a lease persistida do shard.
func (s *StorageLeaseStore) ReleaseLease(ctx context.Context, lease Lease) error {
	if s == nil || s.store == nil {
		return ErrNilLeaseStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := lease.Validate(); err != nil {
		return err
	}
	return s.store.ReleaseLease(ctx, StorageShardID(lease.ShardID), lease.Token)
}

func (s *StorageLeaseStore) nowTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now().UTC()
	}
	return s.now()
}
