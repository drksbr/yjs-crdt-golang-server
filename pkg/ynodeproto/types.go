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
	case MessageTypePing:
		return "ping"
	case MessageTypePong:
		return "pong"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(t))
	}
}
