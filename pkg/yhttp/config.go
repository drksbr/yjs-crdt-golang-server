package yhttp

import (
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

const (
	defaultReadLimitBytes = 16 << 20
	defaultWriteTimeout   = 5 * time.Second
	defaultPersistTimeout = 5 * time.Second
)

// Request descreve como uma requisição HTTP deve ser associada ao provider.
//
// `ClientID` precisa corresponder ao client id usado pelo peer Yjs nos payloads
// de awareness; o provider usa esse valor para agregar estado efêmero e gerar
// tombstones no fechamento da conexão.
type Request struct {
	DocumentKey    storage.DocumentKey
	ConnectionID   string
	ClientID       uint32
	PersistOnClose bool
}

// ResolveRequestFunc mapeia a requisição HTTP para o documento e metadados da
// conexão local.
type ResolveRequestFunc func(r *http.Request) (Request, error)

// ErrorHandler recebe erros assíncronos da camada de transporte que já não
// podem mais ser refletidos na resposta HTTP original.
type ErrorHandler func(r *http.Request, req Request, err error)

// AuthorityLossHandler assume um websocket ja aceito quando a sessao local
// perde autoridade sobre o documento.
//
// O handler recebe o epoch autoritativo que estava ativo na conexao local e
// passa a ser responsavel por manter ou encerrar o socket do cliente.
type AuthorityLossHandler func(
	r *http.Request,
	req Request,
	socket *websocket.Conn,
	previousEpoch uint64,
) error

// ServerConfig define a configuração do handler HTTP/WebSocket.
type ServerConfig struct {
	Provider                      *yprotocol.Provider
	OwnershipRuntime              *ycluster.DocumentOwnershipRuntime
	ResolveRequest                ResolveRequestFunc
	AcceptOptions                 *websocket.AcceptOptions
	ReadLimitBytes                int64
	WriteTimeout                  time.Duration
	PersistTimeout                time.Duration
	AuthorityRevalidationInterval time.Duration
	Metrics                       Metrics
	OnError                       ErrorHandler
}
