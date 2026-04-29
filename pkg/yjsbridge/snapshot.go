package yjsbridge

import (
	"context"

	"github.com/drksbr/yjs-crdt-golang-server/internal/yupdate"
)

// Snapshot é o contrato público de snapshot em memória para o fluxo de update.
//
// A estrutura contém o state vector consolidado e o delete set atual do documento.
type Snapshot = yupdate.Snapshot

// PersistedSnapshot é o contrato público de snapshot persistível com `UpdateV1` e
// estado em memória derivado em `Snapshot`.
type PersistedSnapshot = yupdate.PersistedSnapshot

var (
	// ErrUnsupportedUpdateFormatV2 sinaliza que V2 ainda não é suportado.
	ErrUnsupportedUpdateFormatV2 = yupdate.ErrUnsupportedUpdateFormatV2
	// ErrUnknownUpdateFormat sinaliza payload de update com formato desconhecido.
	ErrUnknownUpdateFormat = yupdate.ErrUnknownUpdateFormat
	// ErrInconsistentPersistedSnapshot sinaliza inconsistência entre `UpdateV1` e
	// o estado em memória de `PersistedSnapshot`.
	ErrInconsistentPersistedSnapshot = yupdate.ErrInconsistentPersistedSnapshot
)

// NewSnapshot cria um novo snapshot em memória vazio.
func NewSnapshot() *Snapshot {
	return yupdate.NewSnapshot()
}

// NewPersistedSnapshot cria um novo snapshot persistido vazio em V1 canônico.
func NewPersistedSnapshot() *PersistedSnapshot {
	return yupdate.NewPersistedSnapshot()
}

// PersistedSnapshotFromUpdate materializa um snapshot persistido a partir de um
// único update.
//
// Atualizações V2 retornam `ErrUnsupportedUpdateFormatV2`.
func PersistedSnapshotFromUpdate(update []byte) (*PersistedSnapshot, error) {
	return yupdate.PersistedSnapshotFromUpdate(update)
}

// PersistedSnapshotFromUpdates materializa um snapshot persistido a partir de
// múltiplos updates.
//
// Payloads `nil` e vazios são tratados como no-op durante a agregação.
func PersistedSnapshotFromUpdates(updates ...[]byte) (*PersistedSnapshot, error) {
	return yupdate.PersistedSnapshotFromUpdates(updates...)
}

// PersistedSnapshotFromUpdatesContext é a variante context-aware da agregação.
//
// `ctx == nil` é tratado como `context.Background()`.
//
// Erros de contexto (`context.Canceled` e `context.DeadlineExceeded`) são
// propagados diretamente quando o contexto é encerrado.
func PersistedSnapshotFromUpdatesContext(ctx context.Context, updates ...[]byte) (*PersistedSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.PersistedSnapshotFromUpdatesContext(ctx, updates...)
}

// EncodePersistedSnapshotV1 materializa e retorna o payload canônico V1 do
// snapshot persistido.
//
// Retorna `ErrInconsistentPersistedSnapshot` se `snapshot` estiver
// estruturalmente inconsistente.
func EncodePersistedSnapshotV1(snapshot *PersistedSnapshot) ([]byte, error) {
	return yupdate.EncodePersistedSnapshotV1(snapshot)
}

// DecodePersistedSnapshotV1 restaura um snapshot persistido a partir de um
// payload V1.
//
// `payload == nil` e payloads vazios são tratados como snapshot vazio.
func DecodePersistedSnapshotV1(payload []byte) (*PersistedSnapshot, error) {
	return yupdate.DecodePersistedSnapshotV1(payload)
}

// DecodePersistedSnapshotV1Context é a variante context-aware da restauração V1.
//
// `ctx == nil` é tratado como `context.Background()`.
//
// Retorna erro de contexto (`context.Canceled` e `context.DeadlineExceeded`) caso
// o contexto seja encerrado antes da conclusão.
func DecodePersistedSnapshotV1Context(ctx context.Context, payload []byte) (*PersistedSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.DecodePersistedSnapshotV1Context(ctx, payload)
}
