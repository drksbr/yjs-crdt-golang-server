package yprotocol

import (
	"context"
	"io"

	internal "yjs-go-bridge/internal/yprotocol"
	"yjs-go-bridge/pkg/yawareness"
	"yjs-go-bridge/pkg/yjsbridge"
)

// ProtocolType identifica o envelope externo usado pelos providers websocket.
type ProtocolType = internal.ProtocolType

// SyncMessageType identifica o subtipo da mensagem de sync.
type SyncMessageType = internal.SyncMessageType

// AuthMessageType identifica o subtipo da mensagem de auth.
type AuthMessageType = internal.AuthMessageType

// SyncMessage representa uma mensagem interna do protocolo de sync.
type SyncMessage = internal.SyncMessage

// AuthMessage representa uma mensagem interna de auth.
type AuthMessage = internal.AuthMessage

// QueryAwarenessMessage representa a consulta vazia do snapshot de awareness.
type QueryAwarenessMessage = internal.QueryAwarenessMessage

// ParseError adiciona contexto mínimo para falhas de parsing do protocolo.
type ParseError = internal.ParseError

// AwarenessMessage representa o payload awareness decodificado do envelope.
type AwarenessMessage = yawareness.Update

// AwarenessClient representa uma entrada do awareness protocol.
type AwarenessClient = yawareness.ClientState

const (
	// ProtocolTypeSync carrega mensagens do protocolo de sync.
	ProtocolTypeSync = internal.ProtocolTypeSync
	// ProtocolTypeAwareness carrega mensagens do protocolo de awareness.
	ProtocolTypeAwareness = internal.ProtocolTypeAwareness
	// ProtocolTypeAuth carrega mensagens auxiliares de autorização.
	ProtocolTypeAuth = internal.ProtocolTypeAuth
	// ProtocolTypeQueryAwareness consulta o snapshot atual de awareness.
	ProtocolTypeQueryAwareness = internal.ProtocolTypeQueryAwareness

	// SyncMessageTypeStep1 contém um state vector.
	SyncMessageTypeStep1 = internal.SyncMessageTypeStep1
	// SyncMessageTypeStep2 contém um update com o estado faltante.
	SyncMessageTypeStep2 = internal.SyncMessageTypeStep2
	// SyncMessageTypeUpdate contém um update incremental.
	SyncMessageTypeUpdate = internal.SyncMessageTypeUpdate

	// AuthMessageTypePermissionDenied sinaliza que o acesso foi negado.
	AuthMessageTypePermissionDenied = internal.AuthMessageTypePermissionDenied
)

var (
	// ErrUnknownProtocolType sinaliza um tipo de protocolo externo não reconhecido.
	ErrUnknownProtocolType = internal.ErrUnknownProtocolType
	// ErrUnexpectedProtocolType sinaliza um protocolo conhecido, mas diferente do esperado.
	ErrUnexpectedProtocolType = internal.ErrUnexpectedProtocolType
	// ErrUnknownSyncMessageType sinaliza um subtipo de sync não reconhecido.
	ErrUnknownSyncMessageType = internal.ErrUnknownSyncMessageType
	// ErrUnknownAuthMessageType sinaliza um subtipo de auth não reconhecido.
	ErrUnknownAuthMessageType = internal.ErrUnknownAuthMessageType
	// ErrInvalidAwarenessJSON sinaliza um estado JSON inválido no payload awareness.
	ErrInvalidAwarenessJSON = internal.ErrInvalidAwarenessJSON
	// ErrTrailingBytes sinaliza bytes extras após o término de uma mensagem isolada.
	ErrTrailingBytes = internal.ErrTrailingBytes
	// ErrProtocolStreamByteLimitExceeded sinaliza estouro do buffer incremental.
	ErrProtocolStreamByteLimitExceeded = internal.ErrProtocolStreamByteLimitExceeded
)

// ProtocolMessage representa uma mensagem com envelope externo do y-protocols.
type ProtocolMessage struct {
	Protocol       ProtocolType
	Sync           *SyncMessage
	Awareness      *AwarenessMessage
	Auth           *AuthMessage
	QueryAwareness *QueryAwarenessMessage
}

// EncodeProtocolMessage serializa o envelope externo com payload bruto.
func EncodeProtocolMessage(protocol ProtocolType, payload []byte) ([]byte, error) {
	return internal.EncodeProtocolMessage(protocol, payload)
}

// DecodeProtocolMessage decodifica uma mensagem completa com envelope externo.
func DecodeProtocolMessage(src []byte) (*ProtocolMessage, error) {
	message, err := internal.DecodeProtocolMessage(src)
	if err != nil {
		return nil, err
	}
	return wrapProtocolMessage(message), nil
}

// DecodeProtocolMessages decodifica um fluxo completo de mensagens protocoladas.
func DecodeProtocolMessages(src []byte) ([]*ProtocolMessage, error) {
	messages, err := internal.DecodeProtocolMessages(src)
	if err != nil {
		return nil, err
	}
	return wrapProtocolMessages(messages), nil
}

// ReadProtocolMessagesFromStream lê mensagens protocoladas até o fim do fluxo.
func ReadProtocolMessagesFromStream(ctx context.Context, stream io.Reader) ([]*ProtocolMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	messages, err := internal.ReadProtocolMessagesFromStream(ctx, stream)
	if err != nil {
		return nil, err
	}
	return wrapProtocolMessages(messages), nil
}

// ReadProtocolMessagesFromStreamN lê no máximo n mensagens do fluxo.
func ReadProtocolMessagesFromStreamN(ctx context.Context, stream io.Reader, n int) ([]*ProtocolMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	messages, err := internal.ReadProtocolMessagesFromStreamN(ctx, stream, n)
	if err != nil {
		return nil, err
	}
	return wrapProtocolMessages(messages), nil
}

// ReadProtocolMessagesFromStreamNWithLimit lê no máximo n mensagens com limite
// opcional de bytes no buffer incremental.
func ReadProtocolMessagesFromStreamNWithLimit(ctx context.Context, stream io.Reader, n int, limitBytes int) ([]*ProtocolMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	messages, err := internal.ReadProtocolMessagesFromStreamNWithLimit(ctx, stream, n, limitBytes)
	if err != nil {
		return nil, err
	}
	return wrapProtocolMessages(messages), nil
}

// EncodeSyncMessage serializa uma mensagem interna do protocolo de sync.
func EncodeSyncMessage(typ SyncMessageType, payload []byte) ([]byte, error) {
	return internal.EncodeSyncMessage(typ, payload)
}

// EncodeSyncStep1 serializa um SyncStep1 com state vector já codificado.
func EncodeSyncStep1(stateVector []byte) []byte {
	return internal.EncodeSyncStep1(stateVector)
}

// EncodeSyncStep1FromUpdate gera um SyncStep1 a partir de um update suportado.
func EncodeSyncStep1FromUpdate(update []byte) ([]byte, error) {
	stateVector, err := yjsbridge.EncodeStateVectorFromUpdate(update)
	if err != nil {
		return nil, err
	}
	return EncodeSyncMessage(SyncMessageTypeStep1, stateVector)
}

// EncodeSyncStep1FromUpdates agrega updates e serializa um SyncStep1.
func EncodeSyncStep1FromUpdates(updates ...[]byte) ([]byte, error) {
	return EncodeSyncStep1FromUpdatesContext(context.Background(), updates...)
}

// EncodeSyncStep1FromUpdatesContext agrega updates respeitando cancelamento.
func EncodeSyncStep1FromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	stateVector, err := yjsbridge.EncodeStateVectorFromUpdatesContext(ctx, updates...)
	if err != nil {
		return nil, err
	}
	return EncodeSyncMessage(SyncMessageTypeStep1, stateVector)
}

// EncodeSyncStep2 serializa um SyncStep2 com um update binário.
func EncodeSyncStep2(update []byte) []byte {
	return internal.EncodeSyncStep2(update)
}

// EncodeSyncStep2FromUpdates consolida múltiplos updates e serializa o resultado.
func EncodeSyncStep2FromUpdates(updates ...[]byte) ([]byte, error) {
	return EncodeSyncStep2FromUpdatesContext(context.Background(), updates...)
}

// EncodeSyncStep2FromUpdatesContext consolida múltiplos updates respeitando cancelamento.
func EncodeSyncStep2FromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	merged, err := yjsbridge.MergeUpdatesContext(ctx, updates...)
	if err != nil {
		return nil, err
	}
	return EncodeSyncMessage(SyncMessageTypeStep2, merged)
}

// EncodeSyncUpdate serializa uma mensagem incremental de update.
func EncodeSyncUpdate(update []byte) []byte {
	return internal.EncodeSyncUpdate(update)
}

// DecodeSyncMessage decodifica uma mensagem de sync isolada.
func DecodeSyncMessage(src []byte) (*SyncMessage, error) {
	return internal.DecodeSyncMessage(src)
}

// EncodeProtocolSyncMessage serializa o envelope externo do protocolo de sync.
func EncodeProtocolSyncMessage(typ SyncMessageType, payload []byte) ([]byte, error) {
	return internal.EncodeProtocolSyncMessage(typ, payload)
}

// EncodeProtocolSyncStep1 serializa protocolo + SyncStep1.
func EncodeProtocolSyncStep1(stateVector []byte) []byte {
	return internal.EncodeProtocolSyncStep1(stateVector)
}

// EncodeProtocolSyncStep1FromUpdate gera protocolo + SyncStep1 a partir de um update.
func EncodeProtocolSyncStep1FromUpdate(update []byte) ([]byte, error) {
	stateVector, err := yjsbridge.EncodeStateVectorFromUpdate(update)
	if err != nil {
		return nil, err
	}
	return EncodeProtocolSyncMessage(SyncMessageTypeStep1, stateVector)
}

// EncodeProtocolSyncStep1FromUpdates agrega updates e serializa protocolo + SyncStep1.
func EncodeProtocolSyncStep1FromUpdates(updates ...[]byte) ([]byte, error) {
	return EncodeProtocolSyncStep1FromUpdatesContext(context.Background(), updates...)
}

// EncodeProtocolSyncStep1FromUpdatesContext agrega updates e serializa protocolo + SyncStep1.
func EncodeProtocolSyncStep1FromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	stateVector, err := yjsbridge.EncodeStateVectorFromUpdatesContext(ctx, updates...)
	if err != nil {
		return nil, err
	}
	return EncodeProtocolSyncMessage(SyncMessageTypeStep1, stateVector)
}

// EncodeProtocolSyncStep2 serializa protocolo + SyncStep2.
func EncodeProtocolSyncStep2(update []byte) []byte {
	return internal.EncodeProtocolSyncStep2(update)
}

// EncodeProtocolSyncStep2FromUpdates consolida updates e serializa protocolo + SyncStep2.
func EncodeProtocolSyncStep2FromUpdates(updates ...[]byte) ([]byte, error) {
	return EncodeProtocolSyncStep2FromUpdatesContext(context.Background(), updates...)
}

// EncodeProtocolSyncStep2FromUpdatesContext consolida updates e serializa protocolo + SyncStep2.
func EncodeProtocolSyncStep2FromUpdatesContext(ctx context.Context, updates ...[]byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	merged, err := yjsbridge.MergeUpdatesContext(ctx, updates...)
	if err != nil {
		return nil, err
	}
	return EncodeProtocolSyncMessage(SyncMessageTypeStep2, merged)
}

// EncodeProtocolSyncUpdate serializa protocolo + Update incremental.
func EncodeProtocolSyncUpdate(update []byte) []byte {
	return internal.EncodeProtocolSyncUpdate(update)
}

// DecodeProtocolSyncMessage decodifica uma mensagem completa de sync.
func DecodeProtocolSyncMessage(src []byte) (*SyncMessage, error) {
	return internal.DecodeProtocolSyncMessage(src)
}

// EncodeAuthMessage serializa uma mensagem interna de auth.
func EncodeAuthMessage(typ AuthMessageType, reason string) ([]byte, error) {
	return internal.EncodeAuthMessage(typ, reason)
}

// EncodeAuthPermissionDenied serializa a mensagem de permissão negada.
func EncodeAuthPermissionDenied(reason string) []byte {
	return internal.EncodeAuthPermissionDenied(reason)
}

// EncodeProtocolAuthMessage serializa uma mensagem auth no envelope externo.
func EncodeProtocolAuthMessage(typ AuthMessageType, reason string) ([]byte, error) {
	return internal.EncodeProtocolAuthMessage(typ, reason)
}

// EncodeProtocolAuthPermissionDenied serializa permission denied com envelope.
func EncodeProtocolAuthPermissionDenied(reason string) []byte {
	return internal.EncodeProtocolAuthPermissionDenied(reason)
}

// DecodeAuthMessage decodifica uma mensagem auth isolada.
func DecodeAuthMessage(src []byte) (*AuthMessage, error) {
	return internal.DecodeAuthMessage(src)
}

// DecodeProtocolAuthMessage decodifica uma mensagem auth com envelope externo.
func DecodeProtocolAuthMessage(src []byte) (*AuthMessage, error) {
	return internal.DecodeProtocolAuthMessage(src)
}

// EncodeProtocolQueryAwareness serializa a consulta de awareness no envelope externo.
func EncodeProtocolQueryAwareness() []byte {
	return internal.EncodeProtocolQueryAwareness()
}

// EncodeProtocolAwarenessUpdate serializa um update awareness com envelope externo.
func EncodeProtocolAwarenessUpdate(update *yawareness.Update) ([]byte, error) {
	return yawareness.EncodeProtocolUpdate(update)
}

// DecodeProtocolQueryAwareness decodifica uma consulta awareness isolada.
func DecodeProtocolQueryAwareness(src []byte) (*QueryAwarenessMessage, error) {
	return internal.DecodeProtocolQueryAwareness(src)
}

// DecodeProtocolAwarenessUpdate decodifica um update awareness com envelope externo.
func DecodeProtocolAwarenessUpdate(src []byte) (*yawareness.Update, error) {
	return yawareness.DecodeProtocolUpdate(src)
}

func wrapProtocolMessages(messages []*internal.ProtocolMessage) []*ProtocolMessage {
	if len(messages) == 0 {
		return []*ProtocolMessage{}
	}
	out := make([]*ProtocolMessage, 0, len(messages))
	for _, current := range messages {
		out = append(out, wrapProtocolMessage(current))
	}
	return out
}

func wrapProtocolMessage(message *internal.ProtocolMessage) *ProtocolMessage {
	if message == nil {
		return nil
	}
	wrapped := &ProtocolMessage{
		Protocol:       message.Protocol,
		Sync:           message.Sync,
		Auth:           message.Auth,
		QueryAwareness: message.QueryAwareness,
	}
	if message.Awareness != nil {
		clients := make([]yawareness.ClientState, 0, len(message.Awareness.Clients))
		for _, client := range message.Awareness.Clients {
			clients = append(clients, yawareness.ClientState{
				ClientID: client.ClientID,
				Clock:    client.Clock,
				State:    append([]byte(nil), client.State...),
			})
		}
		wrapped.Awareness = &yawareness.Update{Clients: clients}
	}
	return wrapped
}
