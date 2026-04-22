package yidset

import "errors"

var (
	// ErrInvalidLength sinaliza tentativa de registrar um range vazio.
	ErrInvalidLength = errors.New("yidset: comprimento invalido")
	// ErrRangeOverflow sinaliza que clock+length sairia da faixa uint32 compatível.
	ErrRangeOverflow = errors.New("yidset: range excede uint32")
)
