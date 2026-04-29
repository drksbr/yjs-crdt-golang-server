package yupdate

import (
	"context"

	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
)

// Snapshot representa o corte mínimo útil de snapshot neste estágio:
// state vector consolidado e delete set associado.
type Snapshot struct {
	StateVector map[uint32]uint32
	DeleteSet   *ytypes.DeleteSet
}

// NewSnapshot cria um snapshot vazio pronto para uso.
func NewSnapshot() *Snapshot {
	return &Snapshot{
		StateVector: map[uint32]uint32{},
		DeleteSet:   ytypes.NewDeleteSet(),
	}
}

// Clone cria uma cópia profunda do snapshot.
func (s *Snapshot) Clone() *Snapshot {
	if s == nil {
		return NewSnapshot()
	}

	stateVector := make(map[uint32]uint32, len(s.StateVector))
	for client, clock := range s.StateVector {
		stateVector[client] = clock
	}

	return &Snapshot{
		StateVector: stateVector,
		DeleteSet:   s.DeleteSet.Clone(),
	}
}

// IsEmpty informa se state vector e delete set estão vazios.
func (s *Snapshot) IsEmpty() bool {
	if s == nil {
		return true
	}
	return len(s.StateVector) == 0 && (s.DeleteSet == nil || s.DeleteSet.IsEmpty())
}

// SnapshotFromUpdate extrai um snapshot do payload conforme o formato detectado.
func SnapshotFromUpdate(update []byte) (*Snapshot, error) {
	format, err := FormatFromUpdate(update)
	if err != nil {
		return nil, err
	}
	switch format {
	case UpdateFormatV1:
		return SnapshotFromUpdateV1(update)
	case UpdateFormatV2:
		converted, err := ConvertUpdateToV1(update)
		if err != nil {
			return nil, err
		}
		return SnapshotFromUpdateV1(converted)
	default:
		return nil, ErrUnknownUpdateFormat
	}
}

// SnapshotFromUpdateV1 extrai um snapshot em memória a partir de um update V1.
func SnapshotFromUpdateV1(update []byte) (*Snapshot, error) {
	return extractSnapshotFromUpdateV1(context.Background(), 0, update)
}

// SnapshotFromUpdatesContext agrega snapshots extraídos de múltiplos updates,
// respeitando cancelamento e tratando payloads vazios como no-op.
func SnapshotFromUpdatesContext(ctx context.Context, updates ...[]byte) (*Snapshot, error) {
	format, err := detectAggregateUpdateFormatSkippingEmptyContext(ctx, updates...)
	if err != nil {
		return nil, err
	}
	switch format {
	case UpdateFormatUnknown:
		return NewSnapshot(), nil
	case UpdateFormatV2:
		converted, err := ConvertUpdatesToV1Context(ctx, updates...)
		if err != nil {
			return nil, err
		}
		return SnapshotFromUpdateV1(converted)
	}

	filtered := make([][]byte, 0, len(updates))
	for _, update := range updates {
		if len(update) == 0 {
			continue
		}
		filtered = append(filtered, update)
	}

	merged, err := MergeUpdatesV1Context(ctx, filtered...)
	if err != nil {
		return nil, err
	}
	return SnapshotFromUpdateV1(merged)
}

// SnapshotFromUpdates agrega snapshots extraídos de múltiplos updates.
func SnapshotFromUpdates(updates ...[]byte) (*Snapshot, error) {
	return SnapshotFromUpdatesContext(context.Background(), updates...)
}

func extractSnapshotFromUpdateV1(ctx context.Context, _ int, update []byte) (*Snapshot, error) {
	return extractSnapshotFromUpdateV1Context(ctx, update)
}

func extractSnapshotFromUpdateV1Context(ctx context.Context, update []byte) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(update) == 0 {
		return NewSnapshot(), nil
	}

	decoded, err := DecodeV1(update)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	stateVector, err := extractStateVectorFromUpdateV1(ctx, 0, update)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		StateVector: stateVector,
		DeleteSet:   decoded.DeleteSet.Clone(),
	}, nil
}
