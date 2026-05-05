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
		if len(snapshot.UpdateV2) != 0 {
			updateV1, err := compactUpdateAsV1(snapshot.UpdateV2)
			if err != nil {
				return nil, err
			}
			return updateV1, nil
		}
		if snapshot.Snapshot != nil && !snapshot.Snapshot.IsEmpty() {
			return nil, ErrInconsistentPersistedSnapshot
		}
		return encodeEmptyUpdateV1(), nil
	}

	updateV1, err := compactUpdateAsV1(snapshot.UpdateV1)
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

	format, err := FormatFromUpdate(payload)
	if err != nil {
		return nil, err
	}
	if format == UpdateFormatV2 {
		return nil, ErrUnsupportedUpdateFormatV2
	}

	decoded, err := DecodeV1(payload)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return persistedSnapshotFromDecodedUpdate(decoded)
}

// EncodePersistedSnapshotV2 materializa o payload canônico V2 a ser armazenado
// no contrato canônico atual.
func EncodePersistedSnapshotV2(snapshot *PersistedSnapshot) ([]byte, error) {
	if snapshot == nil {
		return encodeEmptyUpdateV2(), nil
	}
	if len(snapshot.UpdateV2) != 0 {
		updateV2, err := compactUpdateAsV2(snapshot.UpdateV2)
		if err != nil {
			return nil, err
		}
		if snapshot.Snapshot != nil && !snapshot.Snapshot.IsEmpty() && bytes.Equal(updateV2, encodeEmptyUpdateV2()) {
			return nil, ErrInconsistentPersistedSnapshot
		}
		return updateV2, nil
	}
	updateV1, err := EncodePersistedSnapshotV1(snapshot)
	if err != nil {
		return nil, err
	}
	return compactUpdateAsV2(updateV1)
}

func compactUpdateAsV1(update []byte) ([]byte, error) {
	if len(update) == 0 {
		return encodeEmptyUpdateV1(), nil
	}
	decoded, err := DecodeUpdate(update)
	if err != nil {
		return nil, err
	}
	decoded, err = garbageCollectDeletedContent(decoded)
	if err != nil {
		return nil, err
	}
	return EncodeUpdate(decoded)
}

func compactUpdateAsV2(update []byte) ([]byte, error) {
	if len(update) == 0 {
		return encodeEmptyUpdateV2(), nil
	}
	decoded, err := DecodeUpdate(update)
	if err != nil {
		return nil, err
	}
	decoded, err = garbageCollectDeletedContent(decoded)
	if err != nil {
		return nil, err
	}
	return EncodeUpdateV2(decoded)
}

// DecodePersistedSnapshotV2 restaura um snapshot persistido a partir de um
// payload V2 armazenado, tratando payload vazio como documento vazio.
//
// A representação retornada mantém UpdateV2 como forma canônica e materializa
// UpdateV1 apenas para compatibilidade.
func DecodePersistedSnapshotV2(payload []byte) (*PersistedSnapshot, error) {
	return DecodePersistedSnapshotV2Context(context.Background(), payload)
}

// DecodePersistedSnapshotV2Context restaura um snapshot persistido a partir de
// um payload V2 armazenado, respeitando cancelamento do contexto.
func DecodePersistedSnapshotV2Context(ctx context.Context, payload []byte) (*PersistedSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return NewPersistedSnapshot(), nil
	}

	format, err := FormatFromUpdate(payload)
	if err != nil {
		return nil, err
	}
	if format != UpdateFormatV2 {
		return nil, ErrUnsupportedUpdateFormatV2
	}

	decoded, err := DecodeV2(payload)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return persistedSnapshotFromDecodedUpdate(decoded)
}
