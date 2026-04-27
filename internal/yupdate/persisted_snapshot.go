package yupdate

import (
	"bytes"
	"context"
)

// PersistedSnapshot representa o corte atual de snapshot binário persistido:
// um update V1 canônico acompanhado do snapshot derivado em memória.
type PersistedSnapshot struct {
	UpdateV1 []byte
	Snapshot *Snapshot
}

// NewPersistedSnapshot cria um snapshot persistido vazio na forma canônica V1.
func NewPersistedSnapshot() *PersistedSnapshot {
	return &PersistedSnapshot{
		UpdateV1: encodeEmptyUpdateV1(),
		Snapshot: NewSnapshot(),
	}
}

// Clone cria uma cópia profunda do snapshot persistido.
func (s *PersistedSnapshot) Clone() *PersistedSnapshot {
	if s == nil {
		return NewPersistedSnapshot()
	}

	update := append([]byte(nil), s.UpdateV1...)
	snapshot := NewSnapshot()
	if s.Snapshot != nil {
		snapshot = s.Snapshot.Clone()
	}

	return &PersistedSnapshot{
		UpdateV1: update,
		Snapshot: snapshot,
	}
}

// IsEmpty informa se o snapshot persistido representa o documento vazio.
func (s *PersistedSnapshot) IsEmpty() bool {
	if s == nil {
		return true
	}
	if s.Snapshot != nil {
		return s.Snapshot.IsEmpty()
	}
	return len(s.UpdateV1) == 0 || bytes.Equal(s.UpdateV1, encodeEmptyUpdateV1())
}

// PersistedSnapshotFromUpdate cria um snapshot persistido a partir de um único
// payload suportado, normalizando-o para V1 canônico.
func PersistedSnapshotFromUpdate(update []byte) (*PersistedSnapshot, error) {
	updateV1, err := ConvertUpdateToV1(update)
	if err != nil {
		return nil, err
	}

	snapshot, err := SnapshotFromUpdateV1(updateV1)
	if err != nil {
		return nil, err
	}

	return &PersistedSnapshot{
		UpdateV1: updateV1,
		Snapshot: snapshot,
	}, nil
}

// PersistedSnapshotFromUpdatesContext cria um snapshot persistido a partir de
// múltiplos payloads, consolidando-os para um update V1 canônico.
func PersistedSnapshotFromUpdatesContext(ctx context.Context, updates ...[]byte) (*PersistedSnapshot, error) {
	updateV1, err := ConvertUpdatesToV1Context(ctx, updates...)
	if err != nil {
		return nil, err
	}

	snapshot, err := SnapshotFromUpdateV1(updateV1)
	if err != nil {
		return nil, err
	}

	return &PersistedSnapshot{
		UpdateV1: updateV1,
		Snapshot: snapshot,
	}, nil
}

// PersistedSnapshotFromUpdates cria um snapshot persistido a partir de múltiplos
// payloads usando contexto em background.
func PersistedSnapshotFromUpdates(updates ...[]byte) (*PersistedSnapshot, error) {
	return PersistedSnapshotFromUpdatesContext(context.Background(), updates...)
}
