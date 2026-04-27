package storage

import "errors"

var (
	// ErrSnapshotNotFound sinaliza ausência de snapshot persistido para a chave informada.
	ErrSnapshotNotFound = errors.New("storage: snapshot não encontrado")
	// ErrAuthorityLost sinaliza que o caller nao corresponde mais ao owner atual.
	ErrAuthorityLost = errors.New("storage: autoridade perdida")
	// ErrPlacementNotFound sinaliza ausência de placement persistido para a chave informada.
	ErrPlacementNotFound = errors.New("storage: placement não encontrado")
	// ErrLeaseNotFound sinaliza ausência de lease persistida para o shard informado.
	ErrLeaseNotFound = errors.New("storage: lease não encontrada")
	// ErrLeaseConflict sinaliza conflito de lease com o estado persistido atual do shard.
	ErrLeaseConflict = errors.New("storage: lease em conflito")
	// ErrLeaseStaleEpoch sinaliza epoch obsoleto frente a geracao persistida do shard.
	ErrLeaseStaleEpoch = errors.New("storage: lease com epoch obsoleto")
	// ErrInvalidDocumentKey sinaliza chave de documento inválida.
	ErrInvalidDocumentKey = errors.New("storage: chave de documento inválida")
	// ErrInvalidShardID sinaliza shard inválido.
	ErrInvalidShardID = errors.New("storage: shardID inválido")
	// ErrInvalidNodeID sinaliza nodeID inválido.
	ErrInvalidNodeID = errors.New("storage: nodeID inválido")
	// ErrInvalidOwnerInfo sinaliza owner inválido.
	ErrInvalidOwnerInfo = errors.New("storage: owner info inválida")
	// ErrInvalidUpdatePayload sinaliza update V1 inválido para log incremental.
	ErrInvalidUpdatePayload = errors.New("storage: update payload inválido")
	// ErrInvalidLeaseToken sinaliza token de lease inválido.
	ErrInvalidLeaseToken = errors.New("storage: lease token inválido")
	// ErrInvalidLeaseExpiry sinaliza validade temporal inválida para a lease.
	ErrInvalidLeaseExpiry = errors.New("storage: lease expiry inválido")
	// ErrNilPersistedSnapshot sinaliza tentativa de persistir snapshot nulo.
	ErrNilPersistedSnapshot = errors.New("storage: persisted snapshot não pode ser nil")
	// ErrSnapshotCheckpointMismatch sinaliza conflito entre o checkpoint
	// persistido do snapshot e o offset `after` pedido pelo caller.
	ErrSnapshotCheckpointMismatch = errors.New("storage: checkpoint do snapshot conflita com offset solicitado")
)
