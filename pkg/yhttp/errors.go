package yhttp

import "errors"

var (
	// ErrNilProvider sinaliza ausência de provider na configuração do servidor.
	ErrNilProvider = errors.New("yhttp: provider obrigatorio")
	// ErrNilResolveRequest sinaliza ausência do resolver de request.
	ErrNilResolveRequest = errors.New("yhttp: resolve request obrigatorio")
)
