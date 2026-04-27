package yhttp

import "errors"

var (
	// ErrNilProvider sinaliza ausência de provider na configuração do servidor.
	ErrNilProvider = errors.New("yhttp: provider obrigatorio")
	// ErrNilResolveRequest sinaliza ausência do resolver de request.
	ErrNilResolveRequest = errors.New("yhttp: resolve request obrigatorio")
	// ErrNilLocalServer sinaliza ausência do servidor local em wiring owner-aware.
	ErrNilLocalServer = errors.New("yhttp: local server obrigatorio")
	// ErrNilOwnerLookup sinaliza ausência do lookup de owner em wiring distribuído.
	ErrNilOwnerLookup = errors.New("yhttp: owner lookup obrigatorio")
)
