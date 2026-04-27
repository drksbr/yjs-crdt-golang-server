package yhttp

import "errors"

var (
	// ErrNilProvider sinaliza ausência de provider na configuração do servidor.
	ErrNilProvider = errors.New("yhttp: provider obrigatorio")
	// ErrNilResolveRequest sinaliza ausência do resolver de request.
	ErrNilResolveRequest = errors.New("yhttp: resolve request obrigatorio")
	// ErrNilLocalServer sinaliza ausência do servidor local em wiring distribuído.
	ErrNilLocalServer = errors.New("yhttp: local server obrigatorio")
	// ErrNilOwnerLookup sinaliza ausência do lookup de owner em wiring distribuído.
	ErrNilOwnerLookup = errors.New("yhttp: owner lookup obrigatorio")
	// ErrNilRemoteOwnerDialer sinaliza ausência do dialer no forwarding remoto.
	ErrNilRemoteOwnerDialer = errors.New("yhttp: remote owner dialer obrigatorio")
	// ErrNilRemoteOwnerEndpoint sinaliza ausência do endpoint owner-side.
	ErrNilRemoteOwnerEndpoint = errors.New("yhttp: remote owner endpoint obrigatorio")
	// ErrNilNodeMessageStream sinaliza ausência do stream bidirecional inter-node.
	ErrNilNodeMessageStream = errors.New("yhttp: node message stream obrigatorio")
	// ErrNilRemoteOwnerURLResolver sinaliza ausência do resolver de URL do dialer websocket.
	ErrNilRemoteOwnerURLResolver = errors.New("yhttp: remote owner url resolver obrigatorio")
)
