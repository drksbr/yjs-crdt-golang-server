package ycluster

import "errors"

var (
	// ErrInvalidNodeID sinaliza node id vazio ou invalido.
	ErrInvalidNodeID = errors.New("ycluster: node id invalido")
	// ErrNilLocalNode sinaliza ausencia da identidade local do cluster.
	ErrNilLocalNode = errors.New("ycluster: local node obrigatorio")
	// ErrNilShardResolver sinaliza ausencia de shard resolver no lookup.
	ErrNilShardResolver = errors.New("ycluster: shard resolver obrigatorio")
	// ErrNilPlacementStore sinaliza ausencia de placement store no lookup.
	ErrNilPlacementStore = errors.New("ycluster: placement store obrigatorio")
	// ErrNilLeaseStore sinaliza ausencia de lease store em wiring storage-backed.
	ErrNilLeaseStore = errors.New("ycluster: lease store obrigatorio")
	// ErrInvalidShardCount sinaliza espaco de shards vazio.
	ErrInvalidShardCount = errors.New("ycluster: shard count invalido")
	// ErrInvalidOwnerLookupRequest sinaliza request invalido para resolucao de owner.
	ErrInvalidOwnerLookupRequest = errors.New("ycluster: owner lookup request invalido")
	// ErrInvalidPlacement sinaliza placement inconsistente ou incompleto.
	ErrInvalidPlacement = errors.New("ycluster: placement invalido")
	// ErrPlacementNotFound sinaliza ausencia de placement para o shard consultado.
	ErrPlacementNotFound = errors.New("ycluster: placement nao encontrado")
	// ErrInvalidLease sinaliza lease inconsistente ou incompleta.
	ErrInvalidLease = errors.New("ycluster: lease invalida")
	// ErrInvalidLeaseRequest sinaliza request invalido para acquire/renew de lease.
	ErrInvalidLeaseRequest = errors.New("ycluster: lease request invalido")
	// ErrLeaseExpired sinaliza tentativa de operar sobre lease ja expirada.
	ErrLeaseExpired = errors.New("ycluster: lease expirada")
	// ErrOwnerNotFound sinaliza ausencia de owner resolvido para a chave consultada.
	ErrOwnerNotFound = errors.New("ycluster: owner nao encontrado")
)
