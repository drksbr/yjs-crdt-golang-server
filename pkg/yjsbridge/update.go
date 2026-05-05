package yjsbridge

import (
	"context"

	"github.com/drksbr/yjs-crdt-golang-server/internal/yidset"
	"github.com/drksbr/yjs-crdt-golang-server/internal/yupdate"
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

// ValidateUpdate valida estruturalmente um único update.
//
// Diferente de `FormatFromUpdate`, esta função decodifica o payload detectado e
// confirma que ele é consumível pelo reader V1/V2 correspondente.
func ValidateUpdate(update []byte) error {
	return yupdate.ValidateUpdate(update)
}

// ValidateUpdates valida estruturalmente múltiplos updates e preserva o índice
// do payload inválido no erro retornado.
func ValidateUpdates(updates ...[]byte) error {
	return yupdate.ValidateUpdates(updates...)
}

// ValidateUpdatesContext valida estruturalmente múltiplos updates respeitando
// cancelamento.
func ValidateUpdatesContext(ctx context.Context, updates ...[]byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.ValidateUpdatesContext(ctx, updates...)
}

// ConvertUpdateToV1 normaliza um update suportado para a forma canônica V1.
func ConvertUpdateToV1(update []byte) ([]byte, error) {
	return yupdate.ConvertUpdateToV1(update)
}

// ConvertUpdateToV1YjsWire converte para V1 no wire compatível com clientes
// Yjs (ContentEmbed/ContentFormat em JSON-string).
func ConvertUpdateToV1YjsWire(update []byte) ([]byte, error) {
	return yupdate.ConvertUpdateToV1YjsWire(update)
}

// ConvertUpdateToV2 normaliza um update suportado para a forma canônica V2.
func ConvertUpdateToV2(update []byte) ([]byte, error) {
	return yupdate.ConvertUpdateToV2(update)
}

// ConvertUpdateToV2YjsWire converte para V2 no wire compatível com clientes
// Yjs (ContentEmbed/ContentFormat em lib0-any).
func ConvertUpdateToV2YjsWire(update []byte) ([]byte, error) {
	return yupdate.ConvertUpdateToV2YjsWire(update)
}

// ConvertUpdatesToV1 consolida múltiplos payloads em um único update V1 canônico.
func ConvertUpdatesToV1(updates ...[]byte) ([]byte, error) {
	return yupdate.ConvertUpdatesToV1(updates...)
}

// ConvertUpdatesToV2 consolida múltiplos payloads em um único update V2 canônico.
func ConvertUpdatesToV2(updates ...[]byte) ([]byte, error) {
	return yupdate.ConvertUpdatesToV2(updates...)
}

// ConvertUpdatesToV1Context consolida múltiplos payloads respeitando cancelamento.
func ConvertUpdatesToV1Context(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.ConvertUpdatesToV1Context(ctx, updates...)
}

// ConvertUpdatesToV2Context consolida múltiplos payloads em V2 respeitando cancelamento.
func ConvertUpdatesToV2Context(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.ConvertUpdatesToV2Context(ctx, updates...)
}

// MergeUpdates consolida múltiplos updates em um único payload canônico V1.
func MergeUpdates(updates ...[]byte) ([]byte, error) {
	return yupdate.MergeUpdates(updates...)
}

// MergeUpdatesV2 consolida múltiplos updates em um único payload canônico V2.
func MergeUpdatesV2(updates ...[]byte) ([]byte, error) {
	return yupdate.MergeUpdatesV2(updates...)
}

// MergeUpdatesContext consolida múltiplos updates em V1 respeitando cancelamento.
func MergeUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.MergeUpdatesContext(ctx, updates...)
}

// MergeUpdatesV2Context consolida múltiplos updates em V2 respeitando cancelamento.
func MergeUpdatesV2Context(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.MergeUpdatesV2Context(ctx, updates...)
}

// DiffUpdate retorna a parte do update ainda não coberta pelo state vector em V1.
func DiffUpdate(update, stateVector []byte) ([]byte, error) {
	return yupdate.DiffUpdate(update, stateVector)
}

// DiffUpdateV2 retorna a parte do update ainda não coberta pelo state vector em V2.
func DiffUpdateV2(update, stateVector []byte) ([]byte, error) {
	return yupdate.DiffUpdateV2(update, stateVector)
}

// DiffUpdateContext retorna o diff em V1 respeitando cancelamento.
func DiffUpdateContext(ctx context.Context, update, stateVector []byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.DiffUpdateContext(ctx, update, stateVector)
}

// DiffUpdateV2Context retorna o diff em V2 respeitando cancelamento.
func DiffUpdateV2Context(ctx context.Context, update, stateVector []byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.DiffUpdateV2Context(ctx, update, stateVector)
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

// IntersectUpdateWithContentIDs filtra um update e retorna payload V1.
func IntersectUpdateWithContentIDs(update []byte, contentIDs *ContentIDs) ([]byte, error) {
	return yupdate.IntersectUpdateWithContentIDs(update, contentIDs.toInternal())
}

// IntersectUpdateWithContentIDsV2 filtra um update e retorna payload V2.
func IntersectUpdateWithContentIDsV2(update []byte, contentIDs *ContentIDs) ([]byte, error) {
	return yupdate.IntersectUpdateWithContentIDsV2(update, contentIDs.toInternal())
}

// IntersectUpdateWithContentIDsContext filtra um update em V1 respeitando cancelamento.
func IntersectUpdateWithContentIDsContext(ctx context.Context, update []byte, contentIDs *ContentIDs) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.IntersectUpdateWithContentIDsContext(ctx, update, contentIDs.toInternal())
}

// IntersectUpdateWithContentIDsV2Context filtra um update em V2 respeitando cancelamento.
func IntersectUpdateWithContentIDsV2Context(ctx context.Context, update []byte, contentIDs *ContentIDs) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return yupdate.IntersectUpdateWithContentIDsV2Context(ctx, update, contentIDs.toInternal())
}
