package yupdate

import "errors"

// FormatFromUpdate identifica o formato do update e retorna erro quando a
// heurística não consegue classificar de forma definitiva.
func FormatFromUpdate(update []byte) (UpdateFormat, error) {
	format, err := DetectUpdateFormatWithReason(update)
	if err != nil {
		if errors.Is(err, ErrAmbiguousUpdateFormat) {
			return UpdateFormatUnknown, ErrUnknownUpdateFormat
		}
		return UpdateFormatUnknown, err
	}
	if format == UpdateFormatUnknown {
		return UpdateFormatUnknown, ErrUnknownUpdateFormat
	}
	return format, nil
}

// DecodeUpdate trata o update conforme o formato detectado pelo payload.
func DecodeUpdate(update []byte) (*DecodedUpdate, error) {
	format, err := FormatFromUpdate(update)
	if err != nil {
		return nil, err
	}
	switch format {
	case UpdateFormatV1:
		return DecodeV1(update)
	case UpdateFormatV2:
		return nil, ErrUnsupportedUpdateFormatV2
	default:
		return nil, ErrUnknownUpdateFormat
	}
}

// EncodeUpdate serializa o update em formato V1.
// A estrutura de atualização internamente ainda não diferencia V1/V2.
func EncodeUpdate(update *DecodedUpdate) ([]byte, error) {
	return EncodeV1(update)
}

// DiffUpdate trata o diff conforme o formato detectado pelo payload.
func DiffUpdate(update, stateVector []byte) ([]byte, error) {
	format, err := FormatFromUpdate(update)
	if err != nil {
		return nil, err
	}
	switch format {
	case UpdateFormatV1:
		return DiffUpdateV1(update, stateVector)
	case UpdateFormatV2:
		return nil, ErrUnsupportedUpdateFormatV2
	default:
		return nil, ErrUnknownUpdateFormat
	}
}

// MergeUpdates consolida múltiplos updates e valida consistência de formato.
// Quando há mistura de formatos, retorna erro explícito.
func MergeUpdates(updates ...[]byte) ([]byte, error) {
	if len(updates) == 0 {
		return MergeUpdatesV1()
	}

	format, err := FormatFromUpdate(updates[0])
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(updates); i++ {
		current, err := FormatFromUpdate(updates[i])
		if err != nil {
			return nil, err
		}
		if current != format {
			return nil, ErrMismatchedUpdateFormats
		}
	}

	switch format {
	case UpdateFormatV1:
		return MergeUpdatesV1(updates...)
	case UpdateFormatV2:
		return nil, ErrUnsupportedUpdateFormatV2
	default:
		return nil, ErrUnknownUpdateFormat
	}
}
