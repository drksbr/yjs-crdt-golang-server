package yjsbridge

import (
	"context"

	"yjs-go-bridge/internal/yidset"
	"yjs-go-bridge/internal/yupdate"
)

// UpdateFormat identifica o formato binário detectado para um update Yjs.
type UpdateFormat = yupdate.UpdateFormat

// ParseError adiciona contexto de operação e offset a erros de parsing.
type ParseError = yupdate.ParseError

const (
	// UpdateFormatUnknown indica que o formato não pôde ser identificado.
	UpdateFormatUnknown = yupdate.UpdateFormatUnknown
	// UpdateFormatV1 identifica o wire format clássico do Yjs.
	UpdateFormatV1 = yupdate.UpdateFormatV1
	// UpdateFormatV2 identifica o wire format compacto do Yjs.
	UpdateFormatV2 = yupdate.UpdateFormatV2
)

var (
	// ErrAmbiguousUpdateFormat sinaliza payload inconclusivo entre V1 e V2.
	ErrAmbiguousUpdateFormat = yupdate.ErrAmbiguousUpdateFormat
	// ErrMismatchedUpdateFormats sinaliza mistura de formatos incompatíveis.
	ErrMismatchedUpdateFormats = yupdate.ErrMismatchedUpdateFormats
	// ErrInvalidSliceOffset sinaliza janela de slice inválida em merge/diff/intersect.
	ErrInvalidSliceOffset = yupdate.ErrInvalidSliceOffset
	// ErrUnsupportedContentSlice sinaliza content ainda não fatiável.
	ErrUnsupportedContentSlice = yupdate.ErrUnsupportedContentSlice
	// ErrUnsupportedContentEncode sinaliza content sem bytes suficientes para reencode.
	ErrUnsupportedContentEncode = yupdate.ErrUnsupportedContentEncode
	// ErrTrailingBytes sinaliza bytes residuais após parse do payload.
	ErrTrailingBytes = yupdate.ErrTrailingBytes
	// ErrInvalidContentIDsPayload sinaliza payload inválido de content ids.
	ErrInvalidContentIDsPayload = yupdate.ErrInvalidContentIDsPayload
	// ErrContentIDsTrailingBytes sinaliza bytes residuais após decode de content ids.
	ErrContentIDsTrailingBytes = yupdate.ErrContentIDsTrailingBytes
	// ErrInvalidRangeLength sinaliza tentativa de adicionar range vazio.
	ErrInvalidRangeLength = yidset.ErrInvalidLength
	// ErrRangeOverflow sinaliza range que extrapola o espaço uint32.
	ErrRangeOverflow = yidset.ErrRangeOverflow
)

// SnapshotFromUpdate extrai um snapshot em memória a partir de um único update.
func SnapshotFromUpdate(update []byte) (*Snapshot, error) {
	return yupdate.SnapshotFromUpdate(update)
}

// SnapshotFromUpdates agrega snapshots a partir de múltiplos updates.
func SnapshotFromUpdates(updates ...[]byte) (*Snapshot, error) {
	return yupdate.SnapshotFromUpdates(updates...)
}

// SnapshotFromUpdatesContext agrega snapshots respeitando cancelamento.
func SnapshotFromUpdatesContext(ctx context.Context, updates ...[]byte) (*Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.SnapshotFromUpdatesContext(ctx, updates...)
}

// FormatFromUpdate identifica o formato de um único update.
func FormatFromUpdate(update []byte) (UpdateFormat, error) {
	return yupdate.FormatFromUpdate(update)
}

// FormatFromUpdates identifica o formato comum de múltiplos updates.
func FormatFromUpdates(updates ...[]byte) (UpdateFormat, error) {
	return yupdate.FormatFromUpdates(updates...)
}

// FormatFromUpdatesContext identifica o formato comum respeitando cancelamento.
func FormatFromUpdatesContext(ctx context.Context, updates ...[]byte) (UpdateFormat, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.FormatFromUpdatesContext(ctx, updates...)
}

// ConvertUpdateToV1 normaliza um update suportado para a forma canônica V1.
func ConvertUpdateToV1(update []byte) ([]byte, error) {
	return yupdate.ConvertUpdateToV1(update)
}

// ConvertUpdatesToV1 consolida múltiplos payloads em um único update V1 canônico.
func ConvertUpdatesToV1(updates ...[]byte) ([]byte, error) {
	return yupdate.ConvertUpdatesToV1(updates...)
}

// ConvertUpdatesToV1Context consolida múltiplos payloads respeitando cancelamento.
func ConvertUpdatesToV1Context(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.ConvertUpdatesToV1Context(ctx, updates...)
}

// MergeUpdates consolida múltiplos updates em um único payload.
func MergeUpdates(updates ...[]byte) ([]byte, error) {
	return yupdate.MergeUpdates(updates...)
}

// MergeUpdatesContext consolida múltiplos updates respeitando cancelamento.
func MergeUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.MergeUpdatesContext(ctx, updates...)
}

// DiffUpdate retorna a parte de update ainda não coberta pelo state vector.
func DiffUpdate(update, stateVector []byte) ([]byte, error) {
	return yupdate.DiffUpdate(update, stateVector)
}

// DiffUpdateContext retorna o diff respeitando cancelamento.
func DiffUpdateContext(ctx context.Context, update, stateVector []byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.DiffUpdateContext(ctx, update, stateVector)
}

// DecodeStateVector decodifica um state vector serializado.
func DecodeStateVector(data []byte) (map[uint32]uint32, error) {
	return yupdate.DecodeStateVector(data)
}

// StateVectorFromUpdate extrai o state vector de um único update.
func StateVectorFromUpdate(update []byte) (map[uint32]uint32, error) {
	return yupdate.StateVectorFromUpdate(update)
}

// StateVectorFromUpdates agrega state vectors de múltiplos updates.
func StateVectorFromUpdates(updates ...[]byte) (map[uint32]uint32, error) {
	return yupdate.StateVectorFromUpdates(updates...)
}

// StateVectorFromUpdatesContext agrega state vectors respeitando cancelamento.
func StateVectorFromUpdatesContext(ctx context.Context, updates ...[]byte) (map[uint32]uint32, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.StateVectorFromUpdatesContext(ctx, updates...)
}

// EncodeStateVectorFromUpdate extrai e serializa o state vector de um update.
func EncodeStateVectorFromUpdate(update []byte) ([]byte, error) {
	return yupdate.EncodeStateVectorFromUpdate(update)
}

// EncodeStateVectorFromUpdates agrega e serializa state vectors.
func EncodeStateVectorFromUpdates(updates ...[]byte) ([]byte, error) {
	return yupdate.EncodeStateVectorFromUpdates(updates...)
}

// EncodeStateVectorFromUpdatesContext agrega e serializa state vectors com cancelamento.
func EncodeStateVectorFromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.EncodeStateVectorFromUpdatesContext(ctx, updates...)
}

// CreateContentIDsFromUpdate extrai content ids de um único update.
func CreateContentIDsFromUpdate(update []byte) (*ContentIDs, error) {
	contentIDs, err := yupdate.CreateContentIDsFromUpdate(update)
	if err != nil {
		return nil, err
	}
	return wrapContentIDs(contentIDs), nil
}

// ContentIDsFromUpdates agrega content ids de múltiplos updates.
func ContentIDsFromUpdates(updates ...[]byte) (*ContentIDs, error) {
	contentIDs, err := yupdate.ContentIDsFromUpdates(updates...)
	if err != nil {
		return nil, err
	}
	return wrapContentIDs(contentIDs), nil
}

// ContentIDsFromUpdatesContext agrega content ids respeitando cancelamento.
func ContentIDsFromUpdatesContext(ctx context.Context, updates ...[]byte) (*ContentIDs, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	contentIDs, err := yupdate.ContentIDsFromUpdatesContext(ctx, updates...)
	if err != nil {
		return nil, err
	}
	return wrapContentIDs(contentIDs), nil
}

// IntersectUpdateWithContentIDs filtra um update usando content ids públicos.
func IntersectUpdateWithContentIDs(update []byte, contentIDs *ContentIDs) ([]byte, error) {
	return yupdate.IntersectUpdateWithContentIDs(update, contentIDs.toInternal())
}

// IntersectUpdateWithContentIDsContext filtra um update respeitando cancelamento.
func IntersectUpdateWithContentIDsContext(ctx context.Context, update []byte, contentIDs *ContentIDs) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.IntersectUpdateWithContentIDsContext(ctx, update, contentIDs.toInternal())
}
