package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ShardID identifica um shard lógico dentro da topologia distribuída.
type ShardID string

// Validate confirma se o identificador de shard pode ser usado.
func (id ShardID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("%w: shardID obrigatorio", ErrInvalidShardID)
	}
	return nil
}

// NodeID identifica um nó elegível para receber placement e lease.
type NodeID string

// Validate confirma se o identificador de nó pode ser usado.
func (id NodeID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("%w: nodeID obrigatorio", ErrInvalidNodeID)
	}
	return nil
}

// UpdateOffset representa a posição monotônica de um update em um log por documento.
type UpdateOffset uint64

// UpdateLogRecord representa um update V1 persistido em log append-only.
type UpdateLogRecord struct {
	Key      DocumentKey
	Offset   UpdateOffset
	UpdateV1 []byte
	StoredAt time.Time
}

// Validate confirma se o registro de log pode ser persistido ou retornado.
func (r UpdateLogRecord) Validate() error {
	if err := r.Key.Validate(); err != nil {
		return err
	}
	if len(r.UpdateV1) == 0 {
		return fmt.Errorf("%w: updateV1 obrigatorio", ErrInvalidUpdatePayload)
	}
	return nil
}

// Clone retorna uma cópia profunda do registro.
func (r *UpdateLogRecord) Clone() *UpdateLogRecord {
	if r == nil {
		return nil
	}

	clone := &UpdateLogRecord{
		Key:      r.Key,
		Offset:   r.Offset,
		StoredAt: r.StoredAt,
	}
	if len(r.UpdateV1) > 0 {
		clone.UpdateV1 = append([]byte(nil), r.UpdateV1...)
	}
	return clone
}

// PlacementRecord representa a alocação lógica de um documento para um shard.
type PlacementRecord struct {
	Key       DocumentKey
	ShardID   ShardID
	Version   uint64
	UpdatedAt time.Time
}

// Validate confirma se o placement pode ser persistido ou retornado.
func (r PlacementRecord) Validate() error {
	if err := r.Key.Validate(); err != nil {
		return err
	}
	if err := r.ShardID.Validate(); err != nil {
		return err
	}
	return nil
}

// Clone retorna uma cópia do placement.
func (r *PlacementRecord) Clone() *PlacementRecord {
	if r == nil {
		return nil
	}

	return &PlacementRecord{
		Key:       r.Key,
		ShardID:   r.ShardID,
		Version:   r.Version,
		UpdatedAt: r.UpdatedAt,
	}
}

// OwnerInfo descreve a identidade estável de quem detém uma lease.
//
// Epoch é opcional e permite distinguir reincarnações do mesmo nó quando a
// camada de coordenação expõe uma geração monotônica.
type OwnerInfo struct {
	NodeID NodeID
	Epoch  uint64
}

// Validate confirma se o owner pode ser usado em placement ou lease.
func (o OwnerInfo) Validate() error {
	if err := o.NodeID.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOwnerInfo, err)
	}
	return nil
}

// LeaseRecord representa uma lease efêmera de ownership sobre um shard.
//
// Token é opaco para o pacote e deve ser reutilizado por quem salvar ou liberar
// a lease para permitir semânticas de fencing e renovação na implementação.
type LeaseRecord struct {
	ShardID    ShardID
	Owner      OwnerInfo
	Token      string
	AcquiredAt time.Time
	ExpiresAt  time.Time
}

// Validate confirma se a lease pode ser persistida ou retornada.
func (r LeaseRecord) Validate() error {
	if err := r.ShardID.Validate(); err != nil {
		return err
	}
	if err := r.Owner.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.Token) == "" {
		return fmt.Errorf("%w: token obrigatorio", ErrInvalidLeaseToken)
	}
	if r.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: expiresAt obrigatorio", ErrInvalidLeaseExpiry)
	}
	if !r.AcquiredAt.IsZero() && !r.ExpiresAt.After(r.AcquiredAt) {
		return fmt.Errorf("%w: expiresAt deve ser apos acquiredAt", ErrInvalidLeaseExpiry)
	}
	return nil
}

// Clone retorna uma cópia da lease.
func (r *LeaseRecord) Clone() *LeaseRecord {
	if r == nil {
		return nil
	}

	return &LeaseRecord{
		ShardID:    r.ShardID,
		Owner:      r.Owner,
		Token:      r.Token,
		AcquiredAt: r.AcquiredAt,
		ExpiresAt:  r.ExpiresAt,
	}
}

// UpdateLogStore define o contrato de log incremental por documento.
type UpdateLogStore interface {
	// AppendUpdate adiciona um update V1 ao fim do log do documento e retorna o
	// registro persistido com offset monotônico atribuído pela implementação.
	AppendUpdate(ctx context.Context, key DocumentKey, update []byte) (*UpdateLogRecord, error)

	// ListUpdates retorna updates com offset estritamente maior que after.
	//
	// Quando limit <= 0, a implementação pode retornar todos os registros
	// disponíveis para o documento.
	ListUpdates(ctx context.Context, key DocumentKey, after UpdateOffset, limit int) ([]*UpdateLogRecord, error)

	// TrimUpdates remove, de forma inclusiva, registros com offset <= through.
	TrimUpdates(ctx context.Context, key DocumentKey, through UpdateOffset) error
}

// PlacementStore define o contrato de resolução persistida documento -> shard.
type PlacementStore interface {
	// SavePlacement grava ou substitui o placement lógico de um documento.
	//
	// Implementações podem normalizar campos como Version e UpdatedAt antes de
	// retornar o registro persistido.
	SavePlacement(ctx context.Context, placement PlacementRecord) (*PlacementRecord, error)

	// LoadPlacement recupera o placement persistido do documento.
	//
	// Retorna ErrPlacementNotFound quando não houver alocação registrada.
	LoadPlacement(ctx context.Context, key DocumentKey) (*PlacementRecord, error)
}

// LeaseStore define o contrato de ownership temporário por shard.
type LeaseStore interface {
	// SaveLease grava ou renova a lease informada para o shard.
	//
	// Implementações podem normalizar Token, AcquiredAt e ExpiresAt antes de
	// retornar o registro persistido.
	SaveLease(ctx context.Context, lease LeaseRecord) (*LeaseRecord, error)

	// LoadLease recupera a lease atual do shard.
	//
	// Retorna ErrLeaseNotFound quando não houver ownership persistido.
	LoadLease(ctx context.Context, shardID ShardID) (*LeaseRecord, error)

	// ReleaseLease remove explicitamente a lease do shard identificada pelo token.
	ReleaseLease(ctx context.Context, shardID ShardID, token string) error
}

// DistributedStore agrega os contratos opcionais usados por um runtime
// distribuído completo, mantendo SnapshotStore como base compatível.
type DistributedStore interface {
	SnapshotStore
	UpdateLogStore
	PlacementStore
	LeaseStore
}
