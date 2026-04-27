package storage

import "errors"

var (
	// ErrSnapshotNotFound sinaliza ausência de snapshot persistido para a chave informada.
	ErrSnapshotNotFound = errors.New("storage: snapshot nao encontrado")
	// ErrPlacementNotFound sinaliza ausência de placement persistido para a chave informada.
	ErrPlacementNotFound = errors.New("storage: placement nao encontrado")
	// ErrLeaseNotFound sinaliza ausência de lease persistida para o shard informado.
	ErrLeaseNotFound = errors.New("storage: lease nao encontrada")
	// ErrInvalidDocumentKey sinaliza chave de documento inválida.
	ErrInvalidDocumentKey = errors.New("storage: chave de documento invalida")
	// ErrInvalidShardID sinaliza shard inválido.
	ErrInvalidShardID = errors.New("storage: shardID invalido")
	// ErrInvalidNodeID sinaliza nodeID inválido.
	ErrInvalidNodeID = errors.New("storage: nodeID invalido")
	// ErrInvalidOwnerInfo sinaliza owner inválido.
	ErrInvalidOwnerInfo = errors.New("storage: owner info invalida")
	// ErrInvalidUpdatePayload sinaliza update V1 inválido para log incremental.
	ErrInvalidUpdatePayload = errors.New("storage: update payload invalido")
	// ErrInvalidLeaseToken sinaliza token de lease inválido.
	ErrInvalidLeaseToken = errors.New("storage: lease token invalido")
	// ErrInvalidLeaseExpiry sinaliza validade temporal inválida para a lease.
	ErrInvalidLeaseExpiry = errors.New("storage: lease expiry invalido")
	// ErrNilPersistedSnapshot sinaliza tentativa de persistir snapshot nulo.
	ErrNilPersistedSnapshot = errors.New("storage: persisted snapshot nao pode ser nil")
)
