package yupdate

import (
	"context"
	"errors"
	"fmt"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/yidset"
	"yjs-go-bridge/internal/ytypes"
)

// UpdateFormat identifica o formato binário detectado para um update Yjs.
type UpdateFormat uint8

const (
	// UpdateFormatUnknown indica que o formato não pôde ser identificado.
	UpdateFormatUnknown UpdateFormat = iota
	// UpdateFormatV1 identifica o wire format clássico do Yjs.
	UpdateFormatV1
	// UpdateFormatV2 identifica o wire format compacto do Yjs.
	UpdateFormatV2
)

func (f UpdateFormat) String() string {
	switch f {
	case UpdateFormatV1:
		return "v1"
	case UpdateFormatV2:
		return "v2"
	case UpdateFormatUnknown:
		fallthrough
	default:
		return "unknown"
	}
}

// DetectUpdateFormat aplica a heurística atual de identificação de formato.
func DetectUpdateFormat(update []byte) UpdateFormat {
	format, _ := DetectUpdateFormatWithReason(update)
	return format
}

// DetectUpdateFormatWithReason identifica o formato do update e informa quando a
// heurística não é conclusiva.
func DetectUpdateFormatWithReason(update []byte) (UpdateFormat, error) {
	format, err := detectUpdateFormatWithReason(update)
	if err != nil {
		return format, err
	}
	return format, nil
}

// DetectUpdatesFormatWithReasonContext valida uma lista de updates e confirma se
// todas utilizam o mesmo formato, respeitando cancelamento.
func DetectUpdatesFormatWithReasonContext(ctx context.Context, updates ...[]byte) (UpdateFormat, error) {
	stateVectorFormat, err := aggregatePayloadsInParallel(
		ctx,
		updates,
		0,
		detectSingleUpdateFormat,
		mergeUpdateFormats,
	)
	if err != nil {
		return UpdateFormatUnknown, err
	}
	return stateVectorFormat, nil
}

// DetectUpdatesFormatWithReason valida uma lista de updates e confirma se todas
// utilizam o mesmo formato. O retorno inclui o motivo da falha em índice.
func DetectUpdatesFormatWithReason(updates ...[]byte) (UpdateFormat, error) {
	return DetectUpdatesFormatWithReasonContext(context.Background(), updates...)
}

func detectUpdateFormatWithReason(update []byte) (UpdateFormat, error) {
	if len(update) == 0 {
		return UpdateFormatUnknown, ErrUnknownUpdateFormat
	}

	first, consumed, err := varint.Decode(update)
	if err != nil {
		return UpdateFormatUnknown, err
	}

	if first != 0 {
		return UpdateFormatV1, nil
	}

	// O V1 vazio também começa com zero, então tentamos validar o bloco de delete
	// set restante.
	reader := ybinary.NewReader(update[consumed:])
	v1Err := error(nil)
	if _, err := ReadDeleteSetBlockV1(reader); err == nil {
		if reader.Remaining() == 0 {
			return UpdateFormatV1, nil
		}
		v1Err = ErrTrailingBytes
	} else if !isMalformedUpdatePayloadForV1(err) {
		v1Err = ErrAmbiguousUpdateFormat
	} else {
		v1Err = err
	}

	if err := validateV2HeaderPrefix(update[consumed:]); err == nil {
		return UpdateFormatV2, nil
	}

	if v1Err != nil {
		return UpdateFormatUnknown, v1Err
	}

	return UpdateFormatUnknown, ErrAmbiguousUpdateFormat
}

func detectSingleUpdateFormat(_ context.Context, _ int, update []byte) (UpdateFormat, error) {
	if len(update) == 0 {
		return UpdateFormatUnknown, nil
	}
	return detectUpdateFormatWithReason(update)
}

func mergeUpdateFormats(_ context.Context, formats []UpdateFormat) (UpdateFormat, error) {
	if len(formats) == 0 {
		return UpdateFormatUnknown, ErrUnknownUpdateFormat
	}

	var (
		lastFormat UpdateFormat
		found      bool
	)

	for _, format := range formats {
		if format == UpdateFormatUnknown {
			continue
		}
		if !found {
			lastFormat = format
			found = true
			continue
		}
		if format != lastFormat {
			return UpdateFormatUnknown, fmt.Errorf("%w", ErrMismatchedUpdateFormats)
		}
	}

	if !found {
		return UpdateFormatUnknown, ErrUnknownUpdateFormat
	}
	return lastFormat, nil
}

func isMalformedUpdatePayloadForV1(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, varint.ErrUnexpectedEOF) ||
		errors.Is(err, varint.ErrNonCanonical) ||
		errors.Is(err, varint.ErrOverflow) ||
		errors.Is(err, ytypes.ErrInvalidLength) ||
		errors.Is(err, ytypes.ErrStructOverflow) ||
		errors.Is(err, yidset.ErrInvalidLength) ||
		errors.Is(err, yidset.ErrRangeOverflow)
}

func validateV2HeaderPrefix(src []byte) error {
	reader := ybinary.NewReader(src)
	for i := 0; i < 9; i++ {
		length, _, err := varint.Read(reader)
		if err != nil {
			return err
		}
		if _, err := reader.ReadN(int(length)); err != nil {
			return err
		}
	}
	return nil
}
