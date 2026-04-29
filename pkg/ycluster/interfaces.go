package ycluster

import (
	"context"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

// ShardResolver mapeia uma `storage.DocumentKey` para um shard logico estavel.
type ShardResolver interface {
	ResolveShard(key storage.DocumentKey) (ShardID, error)
	ShardCount() uint32
}

// PlacementStore define o contrato de leitura/escrita do owner atual de um
// shard. Implementacoes concretas podem persistir esse estado em qualquer
// backend de coordenacao.
type PlacementStore interface {
	// SavePlacement grava o placement atual do shard.
	SavePlacement(ctx context.Context, placement Placement) error

	// LoadPlacement recupera o placement atual do shard ou `ErrPlacementNotFound`.
	LoadPlacement(ctx context.Context, shardID ShardID) (*Placement, error)
}

// LeaseStore define o contrato minimo para coordenacao de leases de shard.
type LeaseStore interface {
	// AcquireLease tenta adquirir a lease do shard para o holder informado.
	AcquireLease(ctx context.Context, req LeaseRequest) (*Lease, error)

	// RenewLease tenta estender uma lease ja existente.
	RenewLease(ctx context.Context, req LeaseRequest) (*Lease, error)

	// ReleaseLease devolve explicitamente uma lease previamente adquirida.
	ReleaseLease(ctx context.Context, lease Lease) error
}

// LeaseHandoffStore adiciona troca atomica de lease entre owners.
type LeaseHandoffStore interface {
	LeaseStore

	// HandoffLease transfere uma lease ativa para outro holder, exigindo que a
	// lease atual e o token ainda sejam a autoridade persistida do shard.
	HandoffLease(ctx context.Context, current Lease, req LeaseRequest) (*Lease, error)
}

// LocalNode expoe a identidade estavel do no local.
type LocalNode interface {
	LocalNodeID() NodeID
}

// OwnerLookup resolve qual no e owner atual de um documento.
type OwnerLookup interface {
	LookupOwner(ctx context.Context, req OwnerLookupRequest) (*OwnerResolution, error)
}

// Runtime compoe o contrato minimo esperado de um runtime distribuido: saber
// quem e o no local, resolver shards e consultar ownership corrente.
type Runtime interface {
	LocalNode
	ShardResolver
	OwnerLookup
}
