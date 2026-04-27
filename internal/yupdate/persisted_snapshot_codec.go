package yupdate

import (
	"bytes"
	"context"
)

// EncodePersistedSnapshotV1 materializa o payload canônico V1 a ser armazenado
// para um snapshot persistido.
func EncodePersistedSnapshotV1(snapshot *PersistedSnapshot) ([]byte, error) {
	if snapshot == nil {
		return encodeEmptyUpdateV1(), nil
	}
	if len(snapshot.UpdateV1) == 0 {
		if snapshot.Snapshot != nil && !snapshot.Snapshot.IsEmpty() {
			return nil, ErrInconsistentPersistedSnapshot
		}
		return encodeEmptyUpdateV1(), nil
	}

	updateV1, err := ConvertUpdateToV1(snapshot.UpdateV1)
	if err != nil {
		return nil, err
	}
	if snapshot.Snapshot != nil && !snapshot.Snapshot.IsEmpty() && bytes.Equal(updateV1, encodeEmptyUpdateV1()) {
		return nil, ErrInconsistentPersistedSnapshot
	}
	return updateV1, nil
}

// DecodePersistedSnapshotV1 restaura um snapshot persistido a partir do payload
// V1 armazenado, tratando payload vazio como documento vazio.
func DecodePersistedSnapshotV1(payload []byte) (*PersistedSnapshot, error) {
	return DecodePersistedSnapshotV1Context(context.Background(), payload)
}

// DecodePersistedSnapshotV1Context restaura um snapshot persistido a partir do
// payload V1 armazenado, respeitando cancelamento do contexto.
func DecodePersistedSnapshotV1Context(ctx context.Context, payload []byte) (*PersistedSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return NewPersistedSnapshot(), nil
	}

	updateV1, err := ConvertUpdateToV1(payload)
	if err != nil {
		return nil, err
	}

	snapshot, err := extractSnapshotFromUpdateV1Context(ctx, updateV1)
	if err != nil {
		return nil, err
	}

	return &PersistedSnapshot{
		UpdateV1: updateV1,
		Snapshot: snapshot,
	}, nil
}
