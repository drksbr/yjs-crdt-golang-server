package yprotocol

import (
	"errors"
	"fmt"
)

var (
	// ErrUnknownProtocolType sinaliza um tipo de protocolo externo não reconhecido.
	ErrUnknownProtocolType = errors.New("yprotocol: tipo de protocolo desconhecido")
	// ErrUnexpectedProtocolType sinaliza um protocolo conhecido, mas diferente do esperado.
	ErrUnexpectedProtocolType = errors.New("yprotocol: tipo de protocolo inesperado")
	// ErrUnknownSyncMessageType sinaliza um subtipo de mensagem sync não reconhecido.
	ErrUnknownSyncMessageType = errors.New("yprotocol: tipo de mensagem sync desconhecido")
	// ErrInvalidAwarenessJSON sinaliza um estado JSON inválido no payload awareness.
	ErrInvalidAwarenessJSON = errors.New("yprotocol: estado awareness json invalido")
	// ErrTrailingBytes sinaliza bytes extras após o término de uma mensagem isolada.
	ErrTrailingBytes = errors.New("yprotocol: bytes residuais ao final da mensagem")
)

// ParseError adiciona contexto mínimo para falhas de parsing do protocolo.
type ParseError struct {
	Op     string
	Offset int
	Err    error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("yprotocol: %s falhou no offset %d: %v", e.Op, e.Offset, e.Err)
}

// Unwrap expõe o erro interno para `errors.Is` e `errors.As`.
func (e *ParseError) Unwrap() error {
	return e.Err
}

func wrapError(op string, offset int, err error) error {
	if err == nil {
		return nil
	}
	return &ParseError{Op: op, Offset: offset, Err: err}
}
