package yupdate

import (
	"errors"
	"fmt"
)

var (
	// ErrUnknownContentRef sinaliza payload de Item não suportado pelo parser.
	ErrUnknownContentRef = errors.New("yupdate: content ref desconhecido")
	// ErrUnknownTypeRef sinaliza YType embutido não reconhecido.
	ErrUnknownTypeRef = errors.New("yupdate: type ref desconhecido")
	// ErrAmbiguousUpdateFormat sinaliza que a heurística não conseguiu distinguir com segurança entre V1 e V2.
	ErrAmbiguousUpdateFormat = errors.New("yupdate: formato de update ambíguo entre V1 e V2")
	// ErrUnknownUpdateFormat sinaliza que o formato binário do update não pôde ser identificado.
	ErrUnknownUpdateFormat = errors.New("yupdate: formato de update desconhecido")
	// ErrUnsupportedUpdateFormatV2 sinaliza caminhos V2 ainda sem paridade completa.
	ErrUnsupportedUpdateFormatV2 = errors.New("yupdate: formato de update v2 ainda nao suportado nesta operacao")
	// ErrMismatchedUpdateFormats sinaliza mistura de formatos incompatíveis entre updates.
	ErrMismatchedUpdateFormats = errors.New("yupdate: formatos de update incompatíveis")
	// ErrInvalidSliceOffset sinaliza tentativa de fatiar uma struct fora do range.
	ErrInvalidSliceOffset = errors.New("yupdate: offset de slice invalido")
	// ErrUnsupportedContentSlice sinaliza conteúdo ainda não fatiável neste estágio.
	ErrUnsupportedContentSlice = errors.New("yupdate: slice de content nao suportado")
	// ErrUnsupportedContentEncode sinaliza conteúdo sem bytes suficientes para reencode.
	ErrUnsupportedContentEncode = errors.New("yupdate: encode de content nao suportado")
	// ErrDeleteSetBeforeStructsEnd sinaliza uso incorreto da API lazy.
	ErrDeleteSetBeforeStructsEnd = errors.New("yupdate: delete set lido antes do fim dos structs")
	// ErrTrailingBytes sinaliza bytes remanescentes após o delete set.
	ErrTrailingBytes = errors.New("yupdate: bytes residuais ao final do update")
	// ErrInvalidContentIDsPayload sinaliza inconsistência sintática/semântica no payload de content ids.
	ErrInvalidContentIDsPayload = errors.New("yupdate: payload de content ids invalido")
	// ErrContentIDsTrailingBytes sinaliza bytes remanescentes após decodificação de content ids.
	ErrContentIDsTrailingBytes = errors.New("yupdate: bytes residuais ao final do content ids")
	// ErrInconsistentPersistedSnapshot sinaliza snapshot persistido em memória inconsistente com o payload V1 armazenado.
	ErrInconsistentPersistedSnapshot = errors.New("yupdate: persisted snapshot inconsistente")
	// ErrDecodedCollectionTooLarge sinaliza uma contagem de elementos grande demais para decodificação segura.
	ErrDecodedCollectionTooLarge = errors.New("yupdate: colecao decodificada excede o limite seguro")
)

// ParseError adiciona contexto de operação e offset aos erros do parser.
type ParseError struct {
	Op     string
	Offset int
	Err    error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("yupdate: %s falhou no offset %d: %v", e.Op, e.Offset, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

func wrapError(op string, offset int, err error) error {
	if err == nil {
		return nil
	}
	return &ParseError{Op: op, Offset: offset, Err: err}
}
