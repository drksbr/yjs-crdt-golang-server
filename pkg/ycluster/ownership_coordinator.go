package ycluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

// StorageOwnershipCoordinatorConfig define o wiring storage-backed para
// coordenar placement, lease e lookup de ownership.
type StorageOwnershipCoordinatorConfig struct {
	LocalNode  NodeID
	Resolver   ShardResolver
	Placements storage.PlacementStore
	Leases     storage.LeaseStore
	TTL        time.Duration
	Metrics    Metrics
}

// Validate confirma se a configuração contém as dependências obrigatórias.
func (c StorageOwnershipCoordinatorConfig) Validate() error {
	if err := c.LocalNode.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrNilLocalNode, err)
	}
	if c.Resolver == nil {
		return ErrNilShardResolver
	}
	if c.Placements == nil {
		return ErrNilPlacementStore
	}
	if c.Leases == nil {
		return ErrNilLeaseStore
	}
	if c.TTL <= 0 {
		return fmt.Errorf("%w: ttl obrigatorio", ErrInvalidLeaseRequest)
	}
	return nil
}

// StorageOwnershipCoordinator agrega os contratos storage-backed usados para
// materializar ownership local ou remoto de documentos.
type StorageOwnershipCoordinator struct {
	localNode  NodeID
	resolver   ShardResolver
	placements storage.PlacementStore
	leases     *StorageLeaseStore
	lookup     *StorageOwnerLookup
	ttl        time.Duration
	metrics    Metrics
}

var _ OwnerLookup = (*StorageOwnershipCoordinator)(nil)

// NewStorageOwnershipCoordinator constrói um coordenador storage-backed de
// ownership para documentos.
func NewStorageOwnershipCoordinator(cfg StorageOwnershipCoordinatorConfig) (*StorageOwnershipCoordinator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	leaseStore, err := NewStorageLeaseStore(cfg.Leases)
	if err != nil {
		return nil, err
	}
	leaseStore.WithMetrics(cfg.Metrics)

	lookup, err := NewStorageOwnerLookup(cfg.LocalNode, cfg.Resolver, cfg.Placements, cfg.Leases)
	if err != nil {
		return nil, err
	}
	lookup.WithMetrics(cfg.Metrics)

	return &StorageOwnershipCoordinator{
		localNode:  cfg.LocalNode,
		resolver:   cfg.Resolver,
		placements: cfg.Placements,
		leases:     leaseStore,
		lookup:     lookup,
		ttl:        cfg.TTL,
		metrics:    normalizeMetrics(cfg.Metrics),
	}, nil
}

// ClaimDocumentRequest descreve uma tentativa de materializar ownership para
// um documento.
type ClaimDocumentRequest struct {
	DocumentKey      storage.DocumentKey
	Holder           NodeID
	TTL              time.Duration
	Token            string
	PlacementVersion uint64
}

// HandoffDocumentRequest descreve a troca coordenada de uma lease ativa para o
// próximo holder do mesmo shard/documento.
type HandoffDocumentRequest struct {
	DocumentKey storage.DocumentKey
	Current     Lease
	NextHolder  NodeID
	TTL         time.Duration
	Token       string
}

// DocumentOwnershipRunConfig configura a execução bloqueante de ownership de
// um documento.
type DocumentOwnershipRunConfig struct {
	Claim ClaimDocumentRequest
	Lease LeaseManagerRunConfig

	// ReleaseOnStop libera a lease no encerramento do loop. O release usa um
	// contexto próprio para continuar funcionando quando ctx já foi cancelado.
	ReleaseOnStop bool

	// ReleaseTimeout limita o tempo gasto no release de shutdown. Quando zero,
	// usa um default conservador.
	ReleaseTimeout time.Duration

	// OnClaimed é chamado logo após o claim/adoption inicial do documento.
	OnClaimed func(*DocumentOwnership)
}

// DocumentOwnership representa a posse materializada para um documento.
type DocumentOwnership struct {
	DocumentKey storage.DocumentKey
	ShardID     ShardID
	Placement   *storage.PlacementRecord
	Lease       *Lease
	Manager     *LeaseManager
}

// Clone retorna uma cópia dos dados de ownership. O Manager é reaproveitado por
// ser o controlador de lifecycle associado a esta posse.
func (o *DocumentOwnership) Clone() *DocumentOwnership {
	if o == nil {
		return nil
	}
	return &DocumentOwnership{
		DocumentKey: o.DocumentKey,
		ShardID:     o.ShardID,
		Placement:   o.Placement.Clone(),
		Lease:       o.Lease.Clone(),
		Manager:     o.Manager,
	}
}

// ClaimDocument garante uma lease para o holder solicitado e salva o placement
// do documento apenas depois que o ownership foi confirmado. Holder vazio
// assume o nó local do coordenador.
func (c *StorageOwnershipCoordinator) ClaimDocument(ctx context.Context, req ClaimDocumentRequest) (*DocumentOwnership, error) {
	if c == nil {
		return nil, ErrNilPlacementStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.DocumentKey.Validate(); err != nil {
		return nil, err
	}

	holder := req.Holder
	if holder == "" {
		holder = c.localNode
	}
	if err := holder.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLeaseRequest, err)
	}

	ttl := req.TTL
	if ttl <= 0 {
		ttl = c.ttl
	}

	shardID, err := c.resolver.ResolveShard(req.DocumentKey)
	if err != nil {
		return nil, err
	}
	placementVersion, err := c.resolvePlacementVersion(ctx, req.DocumentKey, shardID, req.PlacementVersion)
	if err != nil {
		return nil, err
	}

	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   c.leases,
		ShardID: shardID,
		Holder:  holder,
		TTL:     ttl,
		Token:   req.Token,
		Metrics: c.metrics,
	})
	if err != nil {
		return nil, err
	}
	lease, acquiredLease, err := manager.acquireAction(ctx, leaseManagerActionAcquire)
	if err != nil {
		return nil, err
	}
	placement, err := c.placements.SavePlacement(ctx, storage.PlacementRecord{
		Key:     req.DocumentKey,
		ShardID: StorageShardID(shardID),
		Version: placementVersion,
	})
	if err != nil {
		if acquiredLease {
			_ = manager.Release(ctx)
		}
		return nil, err
	}

	return &DocumentOwnership{
		DocumentKey: req.DocumentKey,
		ShardID:     shardID,
		Placement:   placement.Clone(),
		Lease:       lease.Clone(),
		Manager:     manager,
	}, nil
}

// PromoteDocument materializa ownership local apenas quando nao ha owner
// remoto ativo para o documento. Leases locais ativas sao adotadas; leases
// ausentes ou expiradas podem ser tomadas com novo epoch monotônico.
func (c *StorageOwnershipCoordinator) PromoteDocument(ctx context.Context, req ClaimDocumentRequest) (*DocumentOwnership, error) {
	if c == nil {
		return nil, ErrNilPlacementStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.DocumentKey.Validate(); err != nil {
		return nil, err
	}

	resolution, err := c.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: req.DocumentKey})
	switch {
	case err == nil:
		if resolution == nil {
			return c.ClaimDocument(ctx, req)
		}
		if !resolution.Local {
			return nil, fmt.Errorf("%w: documento %s/%s pertence a %q", ErrLeaseHeld, req.DocumentKey.Namespace, req.DocumentKey.DocumentID, resolution.Placement.NodeID)
		}
		return c.ClaimDocument(ctx, req)
	case errors.Is(err, ErrOwnerNotFound),
		errors.Is(err, ErrPlacementNotFound),
		errors.Is(err, ErrLeaseExpired):
		return c.ClaimDocument(ctx, req)
	default:
		return nil, err
	}
}

// HandoffDocument transfere o ownership de um documento para o próximo holder
// usando uma troca atômica de lease no storage.
//
// A operação exige que `Current` ainda represente a lease ativa do shard. O
// novo owner recebe `epoch atual + 1`, preservando a monotonicidade usada como
// fence por `snapshot + update log` e pelo provider local.
func (c *StorageOwnershipCoordinator) HandoffDocument(ctx context.Context, req HandoffDocumentRequest) (*DocumentOwnership, error) {
	if c == nil {
		return nil, ErrNilPlacementStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.DocumentKey.Validate(); err != nil {
		return nil, err
	}
	if err := req.Current.Validate(); err != nil {
		return nil, err
	}

	holder := req.NextHolder
	if holder == "" {
		holder = c.localNode
	}
	if err := holder.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLeaseRequest, err)
	}
	if holder == req.Current.Holder {
		return nil, fmt.Errorf("%w: next holder %q ja detem a lease", ErrInvalidLeaseRequest, holder)
	}

	ttl := req.TTL
	if ttl <= 0 {
		ttl = c.ttl
	}

	shardID, err := c.resolver.ResolveShard(req.DocumentKey)
	if err != nil {
		return nil, err
	}
	if req.Current.ShardID != shardID {
		return nil, fmt.Errorf("%w: current shard %s difere do documento %s", ErrInvalidLease, req.Current.ShardID, shardID)
	}

	placement, err := c.placements.LoadPlacement(ctx, req.DocumentKey)
	if err != nil {
		if errors.Is(err, storage.ErrPlacementNotFound) {
			return nil, ErrPlacementNotFound
		}
		return nil, err
	}
	if placement == nil {
		return nil, ErrPlacementNotFound
	}
	if err := placement.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPlacement, err)
	}
	if placement.ShardID != StorageShardID(shardID) {
		return nil, fmt.Errorf("%w: shard %q != %q", ErrInvalidPlacement, placement.ShardID, StorageShardID(shardID))
	}

	lease, err := c.leases.HandoffLease(ctx, req.Current, LeaseRequest{
		ShardID: shardID,
		Holder:  holder,
		TTL:     ttl,
		Token:   req.Token,
	})
	if err != nil {
		return nil, err
	}
	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   c.leases,
		ShardID: shardID,
		Holder:  holder,
		TTL:     ttl,
		Token:   lease.Token,
		Metrics: c.metrics,
	})
	if err != nil {
		return nil, err
	}
	manager.setCurrent(lease)

	return &DocumentOwnership{
		DocumentKey: req.DocumentKey,
		ShardID:     shardID,
		Placement:   placement.Clone(),
		Lease:       lease.Clone(),
		Manager:     manager,
	}, nil
}

// RunDocumentOwnership materializa ownership do documento e mantém a lease
// ativa até ctx ser cancelado ou a lease ser perdida.
func (c *StorageOwnershipCoordinator) RunDocumentOwnership(ctx context.Context, cfg DocumentOwnershipRunConfig) (err error) {
	if c == nil {
		return ErrNilPlacementStore
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.ReleaseTimeout < 0 {
		return fmt.Errorf("%w: releaseTimeout invalido", ErrInvalidLeaseRequest)
	}

	ownership, err := c.ClaimDocument(ctx, cfg.Claim)
	if err != nil {
		return err
	}
	if cfg.OnClaimed != nil {
		cfg.OnClaimed(ownership.Clone())
	}

	if cfg.ReleaseOnStop {
		defer func() {
			if releaseErr := releaseDocumentOwnership(ownership, cfg.ReleaseTimeout); releaseErr != nil {
				err = errors.Join(err, fmt.Errorf("release document ownership: %w", releaseErr))
			}
		}()
	}

	return ownership.Manager.Run(ctx, cfg.Lease)
}

func releaseDocumentOwnership(ownership *DocumentOwnership, timeout time.Duration) error {
	if ownership == nil || ownership.Manager == nil {
		return ErrNilLeaseStore
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := ownership.Manager.Release(ctx); err != nil {
		if errors.Is(err, ErrOwnerNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (c *StorageOwnershipCoordinator) resolvePlacementVersion(ctx context.Context, key storage.DocumentKey, shardID ShardID, requested uint64) (uint64, error) {
	if requested != 0 {
		return requested, nil
	}
	existing, err := c.placements.LoadPlacement(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrPlacementNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if existing == nil {
		return 0, nil
	}
	if err := existing.Validate(); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidPlacement, err)
	}
	expectedStorageShardID := StorageShardID(shardID)
	if existing.ShardID != expectedStorageShardID {
		return 0, fmt.Errorf("%w: shard %q != %q", ErrInvalidPlacement, existing.ShardID, expectedStorageShardID)
	}
	return existing.Version, nil
}

// LookupOwner resolve o owner atual delegando ao lookup storage-backed.
func (c *StorageOwnershipCoordinator) LookupOwner(ctx context.Context, req OwnerLookupRequest) (*OwnerResolution, error) {
	if c == nil || c.lookup == nil {
		return nil, ErrOwnerNotFound
	}
	return c.lookup.LookupOwner(ctx, req)
}

// ResolveAuthorityFence resolve a autoridade local atual como fence de storage.
func (c *StorageOwnershipCoordinator) ResolveAuthorityFence(ctx context.Context, key storage.DocumentKey) (*storage.AuthorityFence, error) {
	return ResolveStorageAuthorityFence(ctx, c, key)
}

// LeaseManagerForDocument constrói um manager para renovar ou liberar a lease
// do shard correspondente ao documento.
func (c *StorageOwnershipCoordinator) LeaseManagerForDocument(key storage.DocumentKey, holder NodeID, ttl time.Duration, token string) (*LeaseManager, error) {
	if c == nil {
		return nil, ErrNilLeaseStore
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if holder == "" {
		holder = c.localNode
	}
	if ttl <= 0 {
		ttl = c.ttl
	}
	shardID, err := c.resolver.ResolveShard(key)
	if err != nil {
		return nil, err
	}
	return NewLeaseManager(LeaseManagerConfig{
		Store:   c.leases,
		ShardID: shardID,
		Holder:  holder,
		TTL:     ttl,
		Token:   token,
		Metrics: c.metrics,
	})
}
