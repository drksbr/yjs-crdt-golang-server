package ycluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

// RebalanceDocumentRequest descreve uma operacao explicita de rebalance de um
// documento para um holder alvo.
type RebalanceDocumentRequest struct {
	DocumentKey  storage.DocumentKey
	TargetHolder NodeID
	TTL          time.Duration
	Token        string

	// PromoteIfOwnerMissing permite materializar ownership no alvo quando nao
	// ha owner ativo. Sem esta flag, rebalance exige uma lease ativa para
	// preservar semantica estrita de handoff.
	PromoteIfOwnerMissing bool

	// PlacementVersion e repassado para a promocao quando nao existe owner
	// ativo. Zero preserva o placement existente ou cria um placement inicial.
	PlacementVersion uint64
}

// RebalanceDocumentResult descreve o resultado materializado de uma tentativa
// de rebalance.
type RebalanceDocumentResult struct {
	DocumentKey      storage.DocumentKey
	From             NodeID
	To               NodeID
	Changed          bool
	Previous         *OwnerResolution
	Ownership        *DocumentOwnership
	PromotedFromLost bool
}

// RebalanceDocument move o ownership de um documento para o holder alvo usando
// a troca atomica de lease/epoch quando existe owner ativo.
//
// Quando o alvo ja e o owner, a operacao e no-op e retorna a ownership atual.
// Quando nao ha owner ativo, a operacao so promove o alvo se
// PromoteIfOwnerMissing estiver habilitado.
func (c *StorageOwnershipCoordinator) RebalanceDocument(ctx context.Context, req RebalanceDocumentRequest) (*RebalanceDocumentResult, error) {
	if c == nil {
		return nil, ErrNilOwnershipCoordinator
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.DocumentKey.Validate(); err != nil {
		return nil, err
	}

	target := req.TargetHolder
	if target == "" {
		target = c.localNode
	}
	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLeaseRequest, err)
	}

	ttl := req.TTL
	if ttl <= 0 {
		ttl = c.ttl
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("%w: ttl obrigatorio", ErrInvalidLeaseRequest)
	}

	resolution, err := c.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: req.DocumentKey})
	if err != nil {
		if !canPromoteAfterOwnerLookup(err) || !req.PromoteIfOwnerMissing {
			return nil, normalizeRebalanceOwnerLookupError(err)
		}

		ownership, promoteErr := c.PromoteDocument(ctx, ClaimDocumentRequest{
			DocumentKey:      req.DocumentKey,
			Holder:           target,
			TTL:              ttl,
			Token:            req.Token,
			PlacementVersion: req.PlacementVersion,
		})
		if promoteErr != nil {
			return nil, promoteErr
		}
		return &RebalanceDocumentResult{
			DocumentKey:      req.DocumentKey,
			To:               target,
			Changed:          true,
			Ownership:        ownership.Clone(),
			PromotedFromLost: true,
		}, nil
	}
	if resolution == nil || resolution.Placement.Lease == nil {
		return nil, ErrOwnerNotFound
	}

	currentHolder := resolution.Placement.NodeID
	if currentHolder == target {
		ownership, err := c.ownershipFromResolution(resolution, ttl)
		if err != nil {
			return nil, err
		}
		return &RebalanceDocumentResult{
			DocumentKey: req.DocumentKey,
			From:        currentHolder,
			To:          target,
			Previous:    cloneOwnerResolution(resolution),
			Ownership:   ownership,
		}, nil
	}

	ownership, err := c.HandoffDocument(ctx, HandoffDocumentRequest{
		DocumentKey: req.DocumentKey,
		Current:     *resolution.Placement.Lease,
		NextHolder:  target,
		TTL:         ttl,
		Token:       req.Token,
	})
	if err != nil {
		return nil, err
	}
	return &RebalanceDocumentResult{
		DocumentKey: req.DocumentKey,
		From:        currentHolder,
		To:          target,
		Changed:     true,
		Previous:    cloneOwnerResolution(resolution),
		Ownership:   ownership.Clone(),
	}, nil
}

func canPromoteAfterOwnerLookup(err error) bool {
	return errors.Is(err, ErrOwnerNotFound) ||
		errors.Is(err, ErrPlacementNotFound) ||
		errors.Is(err, ErrLeaseExpired)
}

func normalizeRebalanceOwnerLookupError(err error) error {
	if errors.Is(err, ErrPlacementNotFound) {
		return fmt.Errorf("%w: %w", ErrOwnerNotFound, err)
	}
	return err
}

func (c *StorageOwnershipCoordinator) ownershipFromResolution(resolution *OwnerResolution, ttl time.Duration) (*DocumentOwnership, error) {
	if c == nil {
		return nil, ErrNilOwnershipCoordinator
	}
	if resolution == nil || resolution.Placement.Lease == nil {
		return nil, ErrOwnerNotFound
	}
	placement := storage.PlacementRecord{
		Key:     resolution.DocumentKey,
		ShardID: StorageShardID(resolution.Placement.ShardID),
		Version: resolution.Placement.Version,
	}
	if err := placement.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPlacement, err)
	}
	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   c.leases,
		ShardID: resolution.Placement.ShardID,
		Holder:  resolution.Placement.NodeID,
		TTL:     ttl,
		Token:   resolution.Placement.Lease.Token,
		Metrics: c.metrics,
	})
	if err != nil {
		return nil, err
	}
	manager.setCurrent(resolution.Placement.Lease)

	return &DocumentOwnership{
		DocumentKey: resolution.DocumentKey,
		ShardID:     resolution.Placement.ShardID,
		Placement:   placement.Clone(),
		Lease:       resolution.Placement.Lease.Clone(),
		Manager:     manager,
	}, nil
}

func cloneOwnerResolution(resolution *OwnerResolution) *OwnerResolution {
	if resolution == nil {
		return nil
	}
	cloned := *resolution
	cloned.Placement.Lease = resolution.Placement.Lease.Clone()
	return &cloned
}
