package ycluster

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

var (
	_ OwnerLookup       = (*StorageOwnerLookup)(nil)
	_ LeaseStore        = (*StorageLeaseStore)(nil)
	_ LeaseHandoffStore = (*StorageLeaseStore)(nil)
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
	metrics     Metrics
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
		metrics:     normalizeMetrics(nil),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}, nil
}

// WithMetrics injeta hooks opcionais de observabilidade no lookup.
func (l *StorageOwnerLookup) WithMetrics(metrics Metrics) *StorageOwnerLookup {
	if l == nil {
		return nil
	}
	l.metrics = normalizeMetrics(metrics)
	return l
}

// LookupOwner resolve o owner ativo do documento a partir do placement do
// documento e da lease corrente do shard correspondente.
func (l *StorageOwnerLookup) LookupOwner(ctx context.Context, req OwnerLookupRequest) (*OwnerResolution, error) {
	start := time.Now()
	if l == nil {
		err := ErrOwnerNotFound
		observeOwnerLookup(nil, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.Validate(); err != nil {
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}

	shardID, err := l.resolver.ResolveShard(req.DocumentKey)
	if err != nil {
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}

	placementRecord, err := l.placements.LoadPlacement(ctx, req.DocumentKey)
	if err != nil {
		if errors.Is(err, storage.ErrPlacementNotFound) {
			err = ErrPlacementNotFound
		}
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}
	if placementRecord == nil {
		err := ErrPlacementNotFound
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}
	if err := placementRecord.Validate(); err != nil {
		err = fmt.Errorf("%w: %v", ErrInvalidPlacement, err)
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}
	expectedStorageShardID := StorageShardID(shardID)
	if placementRecord.ShardID != expectedStorageShardID {
		err := fmt.Errorf("%w: shard %q != %q", ErrInvalidPlacement, placementRecord.ShardID, expectedStorageShardID)
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}

	leaseRecord, err := l.leases.LoadLease(ctx, placementRecord.ShardID)
	if err != nil {
		if errors.Is(err, storage.ErrLeaseNotFound) {
			err = ErrOwnerNotFound
		}
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}
	lease, err := LeaseFromStorageRecord(leaseRecord)
	if err != nil {
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}
	if lease.ShardID != shardID {
		err := fmt.Errorf("%w: lease shard %s != %s", ErrInvalidLease, lease.ShardID, shardID)
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}
	if lease.ExpiredAt(l.nowTime()) {
		err := ErrLeaseExpired
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}

	placement := Placement{
		ShardID: shardID,
		NodeID:  lease.Holder,
		Lease:   lease,
		Version: placementRecord.Version,
	}
	if err := placement.Validate(); err != nil {
		observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(nil, err))
		return nil, err
	}

	resolution := &OwnerResolution{
		DocumentKey: req.DocumentKey,
		Placement:   placement,
		Local:       lease.Holder == l.localNodeID,
	}
	observeOwnerLookup(l.metrics, time.Since(start), ownerLookupResultLabel(resolution, nil))
	return resolution, nil
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
	store   storage.LeaseStore
	metrics Metrics
	now     func() time.Time
}

// NewStorageLeaseStore constrói um adapter de lease store sobre `pkg/storage`.
func NewStorageLeaseStore(store storage.LeaseStore) (*StorageLeaseStore, error) {
	if store == nil {
		return nil, ErrNilLeaseStore
	}
	return &StorageLeaseStore{
		store:   store,
		metrics: normalizeMetrics(nil),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}, nil
}

// WithMetrics injeta hooks opcionais de observabilidade no adapter de lease.
func (s *StorageLeaseStore) WithMetrics(metrics Metrics) *StorageLeaseStore {
	if s == nil {
		return nil
	}
	s.metrics = normalizeMetrics(metrics)
	return s
}

// LoadLease carrega a lease persistida do shard e a converte para o contrato do
// control plane.
func (s *StorageLeaseStore) LoadLease(ctx context.Context, shardID ShardID) (*Lease, error) {
	if s == nil || s.store == nil {
		return nil, ErrNilLeaseStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	record, err := s.store.LoadLease(ctx, StorageShardID(shardID))
	if err != nil {
		if errors.Is(err, storage.ErrLeaseNotFound) {
			return nil, ErrOwnerNotFound
		}
		return nil, err
	}
	lease, err := LeaseFromStorageRecord(record)
	if err != nil {
		return nil, err
	}
	if lease.ExpiredAt(s.nowTime()) {
		return nil, ErrLeaseExpired
	}
	return lease, nil
}

// AcquireLease tenta adquirir ou substituir a lease persistida do shard.
func (s *StorageLeaseStore) AcquireLease(ctx context.Context, req LeaseRequest) (lease *Lease, err error) {
	start := time.Now()
	metrics := Metrics(nil)
	if s != nil {
		metrics = s.metrics
	}
	defer func() {
		observeLeaseOperation(metrics, req.ShardID, leaseOperationAcquire, time.Since(start), leaseResultLabel(err))
	}()
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
		var staleEpoch *storage.LeaseStaleEpochError
		if errors.As(err, &staleEpoch) && staleEpoch.Current >= epoch {
			record, err = s.store.SaveLease(ctx, storage.LeaseRecord{
				ShardID: StorageShardID(req.ShardID),
				Owner: storage.OwnerInfo{
					NodeID: StorageNodeID(req.Holder),
					Epoch:  staleEpoch.Current + 1,
				},
				Token:      token,
				AcquiredAt: now,
				ExpiresAt:  now.Add(req.TTL),
			})
			if err == nil {
				return LeaseFromStorageRecord(record)
			}
		}
		if errors.Is(err, storage.ErrLeaseConflict) {
			return nil, ErrLeaseHeld
		}
		return nil, err
	}
	return LeaseFromStorageRecord(record)
}

// RenewLease renova a lease atual do shard, reaproveitando o token existente
// quando a request não explicitar um token.
func (s *StorageLeaseStore) RenewLease(ctx context.Context, req LeaseRequest) (lease *Lease, err error) {
	start := time.Now()
	metrics := Metrics(nil)
	if s != nil {
		metrics = s.metrics
	}
	defer func() {
		observeLeaseOperation(metrics, req.ShardID, leaseOperationRenew, time.Since(start), leaseResultLabel(err))
	}()
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

// HandoffLease troca a lease ativa para o próximo holder em uma operação
// atômica do storage, exigindo que a lease atual e seu token ainda estejam
// válidos.
func (s *StorageLeaseStore) HandoffLease(ctx context.Context, current Lease, req LeaseRequest) (lease *Lease, err error) {
	start := time.Now()
	metrics := Metrics(nil)
	if s != nil {
		metrics = s.metrics
	}
	defer func() {
		observeLeaseOperation(metrics, current.ShardID, leaseOperationHandoff, time.Since(start), leaseResultLabel(err))
	}()
	if s == nil || s.store == nil {
		return nil, ErrNilLeaseStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := current.Validate(); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.ShardID != 0 && req.ShardID != current.ShardID {
		return nil, fmt.Errorf("%w: next shard %s difere do atual %s", ErrInvalidLeaseRequest, req.ShardID, current.ShardID)
	}
	now := s.nowTime()
	if current.ExpiredAt(now) {
		return nil, ErrLeaseExpired
	}

	handoffStore, ok := s.store.(storage.LeaseHandoffStore)
	if !ok {
		return nil, ErrLeaseHandoffUnsupported
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		token = fmt.Sprintf("%s-%s-%d", req.Holder, current.ShardID, now.UnixNano())
	}
	record, err := handoffStore.HandoffLease(ctx, StorageShardID(current.ShardID), current.Token, storage.LeaseRecord{
		ShardID: StorageShardID(current.ShardID),
		Owner: storage.OwnerInfo{
			NodeID: StorageNodeID(req.Holder),
			Epoch:  current.Epoch + 1,
		},
		Token:      token,
		AcquiredAt: now,
		ExpiresAt:  now.Add(req.TTL),
	})
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrLeaseNotFound):
			return nil, ErrOwnerNotFound
		case errors.Is(err, storage.ErrLeaseConflict):
			return nil, ErrLeaseTokenMismatch
		default:
			return nil, err
		}
	}
	return LeaseFromStorageRecord(record)
}

// ReleaseLease libera explicitamente a lease persistida do shard.
func (s *StorageLeaseStore) ReleaseLease(ctx context.Context, lease Lease) (err error) {
	start := time.Now()
	metrics := Metrics(nil)
	if s != nil {
		metrics = s.metrics
	}
	defer func() {
		observeLeaseOperation(metrics, lease.ShardID, leaseOperationRelease, time.Since(start), leaseResultLabel(err))
	}()
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
