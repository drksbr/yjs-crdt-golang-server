package ycluster

import (
	"context"
	"fmt"
)

// StaticLocalNode expõe uma identidade local fixa para wiring simples do cluster.
type StaticLocalNode struct {
	ID NodeID
}

// LocalNodeID retorna o identificador estável do nó local.
func (n StaticLocalNode) LocalNodeID() NodeID {
	return n.ID
}

// Validate confirma se a identidade local está configurada.
func (n StaticLocalNode) Validate() error {
	if err := n.ID.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrNilLocalNode, err)
	}
	return nil
}

// PlacementOwnerLookup resolve owner usando shard resolver e placement store.
type PlacementOwnerLookup struct {
	localNodeID    NodeID
	shardResolver  ShardResolver
	placementStore PlacementStore
}

// NewPlacementOwnerLookup constrói um lookup de owner com dependências explícitas.
func NewPlacementOwnerLookup(localNode NodeID, resolver ShardResolver, placements PlacementStore) (*PlacementOwnerLookup, error) {
	if err := localNode.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNilLocalNode, err)
	}
	if resolver == nil {
		return nil, ErrNilShardResolver
	}
	if placements == nil {
		return nil, ErrNilPlacementStore
	}
	return &PlacementOwnerLookup{
		localNodeID:    localNode,
		shardResolver:  resolver,
		placementStore: placements,
	}, nil
}

// LookupOwner resolve o owner atual do documento e informa se ele é local.
func (l *PlacementOwnerLookup) LookupOwner(ctx context.Context, req OwnerLookupRequest) (*OwnerResolution, error) {
	if l == nil {
		return nil, ErrOwnerNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	shardID, err := l.shardResolver.ResolveShard(req.DocumentKey)
	if err != nil {
		return nil, err
	}

	placement, err := l.placementStore.LoadPlacement(ctx, shardID)
	if err != nil {
		return nil, err
	}
	if placement == nil {
		return nil, ErrOwnerNotFound
	}
	if err := placement.Validate(); err != nil {
		return nil, err
	}
	if placement.ShardID != shardID {
		return nil, fmt.Errorf("%w: shard %s != %s", ErrInvalidPlacement, placement.ShardID, shardID)
	}

	return &OwnerResolution{
		DocumentKey: req.DocumentKey,
		Placement:   *placement,
		Local:       placement.NodeID == l.localNodeID,
	}, nil
}
