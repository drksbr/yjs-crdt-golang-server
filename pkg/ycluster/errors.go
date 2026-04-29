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
	// ErrNilOwnershipCoordinator sinaliza ausencia do coordenador de ownership.
	ErrNilOwnershipCoordinator = errors.New("ycluster: ownership coordinator obrigatorio")
	// ErrOwnershipRuntimeClosed sinaliza uso de runtime de ownership encerrado.
	ErrOwnershipRuntimeClosed = errors.New("ycluster: ownership runtime encerrado")
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
	// ErrLeaseHeld sinaliza tentativa de acquire sobre lease ainda ativa de outro owner.
	ErrLeaseHeld = errors.New("ycluster: lease ativa para outro owner")
	// ErrLeaseTokenMismatch sinaliza renew/release com token diferente do owner atual.
	ErrLeaseTokenMismatch = errors.New("ycluster: lease token nao corresponde ao owner atual")
	// ErrLeaseExpired sinaliza tentativa de operar sobre lease ja expirada.
	ErrLeaseExpired = errors.New("ycluster: lease expirada")
	// ErrLeaseHandoffUnsupported sinaliza storage sem troca atomica de lease.
	ErrLeaseHandoffUnsupported = errors.New("ycluster: handoff atomico de lease nao suportado")
	// ErrOwnerNotFound sinaliza ausencia de owner resolvido para a chave consultada.
	ErrOwnerNotFound = errors.New("ycluster: owner nao encontrado")
)
