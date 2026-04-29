package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

// DocumentKey identifica um documento persistível.
// Namespace é opcional; DocumentID é obrigatório.
type DocumentKey struct {
	Namespace  string
	DocumentID string
}

// Validate confirma se a chave pode ser usada como identificador de documento.
func (k DocumentKey) Validate() error {
	if strings.TrimSpace(k.DocumentID) == "" {
		return fmt.Errorf("%w: documentID obrigatorio", ErrInvalidDocumentKey)
	}
	return nil
}

// SnapshotRecord representa um snapshot persistido carregado do store.
type SnapshotRecord struct {
	Key      DocumentKey
	Snapshot *yjsbridge.PersistedSnapshot
	Through  UpdateOffset
	Epoch    uint64
	StoredAt time.Time
}

// Clone retorna uma cópia profunda do registro.
func (r *SnapshotRecord) Clone() *SnapshotRecord {
	if r == nil {
		return nil
	}

	var snapshot *yjsbridge.PersistedSnapshot
	if r.Snapshot != nil {
		snapshot = r.Snapshot.Clone()
	}

	return &SnapshotRecord{
		Key:      r.Key,
		Snapshot: snapshot,
		Through:  r.Through,
		Epoch:    r.Epoch,
		StoredAt: r.StoredAt,
	}
}

// SnapshotStore define o contrato mínimo de persistência de snapshots.
type SnapshotStore interface {
	// SaveSnapshot grava um snapshot persistido para a chave informada e retorna o registro salvo.
	SaveSnapshot(ctx context.Context, key DocumentKey, snapshot *yjsbridge.PersistedSnapshot) (*SnapshotRecord, error)

	// LoadSnapshot recupera o snapshot persistido para a chave informada.
	LoadSnapshot(ctx context.Context, key DocumentKey) (*SnapshotRecord, error)
}

// SnapshotCheckpointStore adiciona persistência opcional do checkpoint
// representado pelo snapshot.
type SnapshotCheckpointStore interface {
	SnapshotStore

	// SaveSnapshotCheckpoint grava o snapshot junto com o offset máximo que ele
	// já incorpora do update log. Implementações podem ignorar esse valor quando
	// não oferecem replay incremental.
	SaveSnapshotCheckpoint(ctx context.Context, key DocumentKey, snapshot *yjsbridge.PersistedSnapshot, through UpdateOffset) (*SnapshotRecord, error)
}

// SnapshotCheckpointEpochStore adiciona persistência opcional do epoch
// observado no checkpoint salvo.
type SnapshotCheckpointEpochStore interface {
	SnapshotCheckpointStore

	// SaveSnapshotCheckpointEpoch grava o snapshot junto com o offset máximo e o
	// epoch autoritativo observado naquele checkpoint.
	SaveSnapshotCheckpointEpoch(ctx context.Context, key DocumentKey, snapshot *yjsbridge.PersistedSnapshot, through UpdateOffset, epoch uint64) (*SnapshotRecord, error)
}
