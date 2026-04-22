package yupdate

import (
	"errors"

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
	if _, err := ReadDeleteSetBlockV1(reader); err != nil {
		if isMalformedUpdatePayloadForV1(err) {
			return UpdateFormatUnknown, err
		}
		return UpdateFormatUnknown, ErrAmbiguousUpdateFormat
	}

	if reader.Remaining() != 0 {
		return UpdateFormatUnknown, ErrTrailingBytes
	}

	return UpdateFormatV1, nil
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
