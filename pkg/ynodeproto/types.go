package ynodeproto

import "fmt"

const (
	// Version1 identifica a primeira revisão pública do wire format inter-node.
	Version1 uint8 = 1
	// CurrentVersion identifica a revisão emitida pelo encoder deste pacote.
	CurrentVersion = Version1
	// HeaderSize é o tamanho fixo do header v1 em bytes.
	HeaderSize = 8
)

// Flags representa bits auxiliares do frame preservados pelo codec.
// A semântica de cada bit é definida pelo tipo de mensagem que o consome.
type Flags uint16

const (
	// FlagNone representa ausência de bits auxiliares.
	FlagNone Flags = 0
)

const (
	// FlagPersistOnClose informa que a conexão roteada deve persistir o snapshot
	// ao encerrar o contexto remoto.
	FlagPersistOnClose Flags = 1 << iota
	// FlagCloseRetryable informa que a mensagem Close representa um cutover
	// retryable e o cliente deve tentar reconectar.
	FlagCloseRetryable
	// FlagSupportsUpdateV2 anuncia suporte aos message types de update V2.
	FlagSupportsUpdateV2
)

// MessageType identifica a classe semântica do payload inter-node.
type MessageType uint8

const (
	// MessageTypeHandshake inicia negociação de identidade/capacidades do nó.
	MessageTypeHandshake MessageType = 1
	// MessageTypeHandshakeAck confirma a negociação inicial entre nós.
	MessageTypeHandshakeAck MessageType = 2

	// MessageTypeDocumentSyncRequest solicita estado inicial/catch-up de um documento.
	MessageTypeDocumentSyncRequest MessageType = 16
	// MessageTypeDocumentSyncResponse entrega material de sincronização solicitado.
	MessageTypeDocumentSyncResponse MessageType = 17
	// MessageTypeDocumentUpdate carrega um delta incremental de documento.
	MessageTypeDocumentUpdate MessageType = 18
	// MessageTypeAwarenessUpdate carrega um delta incremental de awareness.
	MessageTypeAwarenessUpdate MessageType = 19
	// MessageTypeQueryAwarenessRequest consulta o snapshot de awareness roteado.
	MessageTypeQueryAwarenessRequest MessageType = 20
	// MessageTypeQueryAwarenessResponse entrega o snapshot de awareness solicitado.
	MessageTypeQueryAwarenessResponse MessageType = 21
	// MessageTypeDisconnect notifica o owner que a borda perdeu a conexao do cliente.
	MessageTypeDisconnect MessageType = 22
	// MessageTypeClose instrui a borda a encerrar explicitamente a conexao encaminhada.
	MessageTypeClose MessageType = 23
	// MessageTypeDocumentSyncResponseV2 entrega material de sincronização em Update V2.
	MessageTypeDocumentSyncResponseV2 MessageType = 24
	// MessageTypeDocumentUpdateV2 carrega um delta incremental em Update V2.
	MessageTypeDocumentUpdateV2 MessageType = 25
	// MessageTypeDocumentSyncRequestV2 solicita catch-up owner-side com resposta em Update V2.
	MessageTypeDocumentSyncRequestV2 MessageType = 26
	// MessageTypeDocumentUpdateV2FromEdge carrega um delta Update V2 da borda para o owner.
	MessageTypeDocumentUpdateV2FromEdge MessageType = 27

	// MessageTypePing carrega keepalive/medição de latência entre nós.
	MessageTypePing MessageType = 240
	// MessageTypePong responde a um ping previamente recebido.
	MessageTypePong MessageType = 241
)

// Valid informa se o tipo de mensagem é reconhecido pelo protocolo atual.
func (t MessageType) Valid() bool {
	switch t {
	case MessageTypeHandshake,
		MessageTypeHandshakeAck,
		MessageTypeDocumentSyncRequest,
		MessageTypeDocumentSyncResponse,
		MessageTypeDocumentUpdate,
		MessageTypeAwarenessUpdate,
		MessageTypeQueryAwarenessRequest,
		MessageTypeQueryAwarenessResponse,
		MessageTypeDisconnect,
		MessageTypeClose,
		MessageTypeDocumentSyncResponseV2,
		MessageTypeDocumentUpdateV2,
		MessageTypeDocumentSyncRequestV2,
		MessageTypeDocumentUpdateV2FromEdge,
		MessageTypePing,
		MessageTypePong:
		return true
	default:
		return false
	}
}

// String retorna a representação textual conhecida do tipo de mensagem.
func (t MessageType) String() string {
	switch t {
	case MessageTypeHandshake:
		return "handshake"
	case MessageTypeHandshakeAck:
		return "handshake-ack"
	case MessageTypeDocumentSyncRequest:
		return "document-sync-request"
	case MessageTypeDocumentSyncResponse:
		return "document-sync-response"
	case MessageTypeDocumentUpdate:
		return "document-update"
	case MessageTypeAwarenessUpdate:
		return "awareness-update"
	case MessageTypeQueryAwarenessRequest:
		return "query-awareness-request"
	case MessageTypeQueryAwarenessResponse:
		return "query-awareness-response"
	case MessageTypeDisconnect:
		return "disconnect"
	case MessageTypeClose:
		return "close"
	case MessageTypeDocumentSyncResponseV2:
		return "document-sync-response-v2"
	case MessageTypeDocumentUpdateV2:
		return "document-update-v2"
	case MessageTypeDocumentSyncRequestV2:
		return "document-sync-request-v2"
	case MessageTypeDocumentUpdateV2FromEdge:
		return "document-update-v2-from-edge"
	case MessageTypePing:
		return "ping"
	case MessageTypePong:
		return "pong"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(t))
	}
}
