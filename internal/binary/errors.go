package binary

import (
	"errors"
	"fmt"
)

var (
	// ErrUnexpectedEOF sinaliza que o parser tentou consumir mais bytes do que o buffer contém.
	ErrUnexpectedEOF = errors.New("binary: dados insuficientes")
	// ErrInvalidReadSize sinaliza chamada inválida da API, como leitura com tamanho negativo.
	ErrInvalidReadSize = errors.New("binary: tamanho de leitura invalido")
)

// ParseError carrega contexto mínimo para depuração de falhas de parsing binário.
// O erro interno define a classe da falha, enquanto os campos numéricos descrevem
// o estado exato do leitor no momento do erro.
type ParseError struct {
	Op        string
	Offset    int
	Need      int
	Remaining int
	Size      int
	Err       error
}

func (e *ParseError) Error() string {
	switch {
	case errors.Is(e.Err, ErrUnexpectedEOF):
		return fmt.Sprintf(
			"binary: %s falhou no offset %d: dados insuficientes (precisa=%d restante=%d)",
			e.Op,
			e.Offset,
			e.Need,
			e.Remaining,
		)
	case errors.Is(e.Err, ErrInvalidReadSize):
		return fmt.Sprintf(
			"binary: %s falhou no offset %d: tamanho de leitura invalido (%d)",
			e.Op,
			e.Offset,
			e.Size,
		)
	default:
		return fmt.Sprintf("binary: %s falhou no offset %d: %v", e.Op, e.Offset, e.Err)
	}
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

func newUnexpectedEOF(op string, offset, need, remaining int) error {
	return &ParseError{
		Op:        op,
		Offset:    offset,
		Need:      need,
		Remaining: remaining,
		Err:       ErrUnexpectedEOF,
	}
}

func newInvalidReadSize(op string, offset, size int) error {
	return &ParseError{
		Op:     op,
		Offset: offset,
		Size:   size,
		Err:    ErrInvalidReadSize,
	}
}
