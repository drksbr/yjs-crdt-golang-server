package yawareness

import internal "yjs-go-bridge/internal/yawareness"

// ClientState representa uma entrada do awareness protocol para um cliente.
type ClientState = internal.ClientState

// Update representa o payload completo do awareness protocol.
type Update = internal.Update

// ClientMeta mantém o clock observado e o instante da última atualização.
type ClientMeta = internal.ClientMeta

// StateManager mantém o estado mais recente de awareness por cliente.
type StateManager = internal.StateManager

// ParseError adiciona contexto e offset às falhas de parsing do protocolo.
type ParseError = internal.ParseError

var (
	// ErrInvalidJSON sinaliza estado awareness com JSON inválido.
	ErrInvalidJSON = internal.ErrInvalidJSON
	// ErrTrailingBytes sinaliza bytes extras após uma mensagem awareness isolada.
	ErrTrailingBytes = internal.ErrTrailingBytes
	// ErrLocalClientIDNotConfigured sinaliza ausência de clientID local.
	ErrLocalClientIDNotConfigured = internal.ErrLocalClientIDNotConfigured
)

// OutdatedTimeout replica o timeout de inatividade usado pelo awareness do Yjs.
const OutdatedTimeout = internal.OutdatedTimeout

// NewStateManager cria um manager já associado a um clientID local.
func NewStateManager(localClientID uint32) *StateManager {
	return internal.NewStateManager(localClientID)
}

// AppendUpdate serializa um payload awareness sem o envelope externo do protocolo.
//
// Este helper mantém compatibilidade binária com o pacote interno e aceita `nil`
// como atualização vazia.
func AppendUpdate(dst []byte, update *Update) ([]byte, error) {
	return internal.AppendUpdate(dst, update)
}

// EncodeUpdate serializa um payload awareness sem o envelope externo.
//
// O contrato atual é de encode/decode estrito em V1: estados com JSON inválido
// retornam `ErrInvalidJSON`, e o estado `null` é normalizado para o tombstone.
func EncodeUpdate(update *Update) ([]byte, error) {
	return internal.EncodeUpdate(update)
}

// DecodeUpdate decodifica um payload awareness isolado e rejeita bytes extras.
//
// O contrato atual exige mensagem exata: qualquer byte residual após o payload
// válido retorna `ErrTrailingBytes`.
func DecodeUpdate(src []byte) (*Update, error) {
	return internal.DecodeUpdate(src)
}

// EncodeProtocolUpdate serializa uma mensagem awareness com envelope y-protocols.
//
// Este é o envelope usado no fluxo de `ProtocolTypeAwareness`.
func EncodeProtocolUpdate(update *Update) ([]byte, error) {
	return internal.EncodeProtocolUpdate(update)
}

// DecodeProtocolUpdate decodifica uma mensagem awareness com envelope y-protocols.
//
// Para observabilidade, erros de protocolo retornam `ParseError` com operação e
// offset quando disponíveis.
func DecodeProtocolUpdate(src []byte) (*Update, error) {
	return internal.DecodeProtocolUpdate(src)
}
