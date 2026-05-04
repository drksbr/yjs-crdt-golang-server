package yupdate

import (
	"bytes"
	"context"
)

// PersistedSnapshot representa o snapshot binário persistido em V2 canônico.
//
// UpdateV1 é mantido como materialização de compatibilidade para callers e stores
// que ainda não migraram para o contrato V2.
type PersistedSnapshot struct {
	UpdateV2 []byte
	UpdateV1 []byte
	Snapshot *Snapshot
}

// NewPersistedSnapshot cria um snapshot persistido vazio na forma canônica V2.
func NewPersistedSnapshot() *PersistedSnapshot {
	return &PersistedSnapshot{
		UpdateV2: encodeEmptyUpdateV2(),
		UpdateV1: encodeEmptyUpdateV1(),
		Snapshot: NewSnapshot(),
	}
}

// Clone cria uma cópia profunda do snapshot persistido.
func (s *PersistedSnapshot) Clone() *PersistedSnapshot {
	if s == nil {
		return NewPersistedSnapshot()
	}

	updateV2 := append([]byte(nil), s.UpdateV2...)
	updateV1 := append([]byte(nil), s.UpdateV1...)
	snapshot := NewSnapshot()
	if s.Snapshot != nil {
		snapshot = s.Snapshot.Clone()
	}

	return &PersistedSnapshot{
		UpdateV2: updateV2,
		UpdateV1: updateV1,
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
	return (len(s.UpdateV2) == 0 || bytes.Equal(s.UpdateV2, encodeEmptyUpdateV2())) &&
		(len(s.UpdateV1) == 0 || bytes.Equal(s.UpdateV1, encodeEmptyUpdateV1()))
}

// PersistedSnapshotFromUpdate cria um snapshot persistido a partir de um único
// payload suportado, normalizando-o para V2 canônico e mantendo V1 compatível.
func PersistedSnapshotFromUpdate(update []byte) (*PersistedSnapshot, error) {
	decoded, err := DecodeUpdate(update)
	if err != nil {
		return nil, err
	}
	return persistedSnapshotFromDecodedUpdate(decoded)
}

// PersistedSnapshotFromUpdatesContext cria um snapshot persistido a partir de
// múltiplos payloads, consolidando-os para um update V2 canônico.
func PersistedSnapshotFromUpdatesContext(ctx context.Context, updates ...[]byte) (*PersistedSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	updateV2, err := ConvertUpdatesToV2Context(ctx, updates...)
	if err != nil {
		return nil, err
	}
	decoded, err := DecodeV2(updateV2)
	if err != nil {
		return nil, err
	}
	return persistedSnapshotFromDecodedUpdate(decoded)
}

// PersistedSnapshotFromUpdates cria um snapshot persistido a partir de múltiplos
// payloads usando contexto em background.
func PersistedSnapshotFromUpdates(updates ...[]byte) (*PersistedSnapshot, error) {
	return PersistedSnapshotFromUpdatesContext(context.Background(), updates...)
}

func persistedSnapshotFromDecodedUpdate(decoded *DecodedUpdate) (*PersistedSnapshot, error) {
	updateV2, err := EncodeUpdateV2(decoded)
	if err != nil {
		return nil, err
	}
	updateV1, err := EncodeUpdate(decoded)
	if err != nil {
		return nil, err
	}
	return &PersistedSnapshot{
		UpdateV2: updateV2,
		UpdateV1: updateV1,
		Snapshot: snapshotFromDecodedUpdate(decoded),
	}, nil
}
