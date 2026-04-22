package yprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/internal/varint"
)

// ProtocolMessage representa uma mensagem com envelope externo do y-protocols.
// Apenas um subtipo (sync ou awareness) deve estar preenchido.
type ProtocolMessage struct {
	Protocol  ProtocolType
	Sync      *SyncMessage
	Awareness *AwarenessMessage
}

// AppendProtocolMessagePayload escreve o envelope externo com protocolo e payload bruto.
func AppendProtocolMessagePayload(dst []byte, protocol ProtocolType, payload []byte) ([]byte, error) {
	if !protocol.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrUnknownProtocolType, protocol)
	}
	dst = AppendProtocolType(dst, protocol)
	return append(dst, payload...), nil
}

// EncodeProtocolMessage serializa uma mensagem no envelope externo do protocolo.
func EncodeProtocolMessage(protocol ProtocolType, payload []byte) ([]byte, error) {
	return AppendProtocolMessagePayload(nil, protocol, payload)
}

// ReadProtocolMessage lê uma mensagem com envelope externo e faz roteamento por tipo.
func ReadProtocolMessage(r *ybinary.Reader) (*ProtocolMessage, error) {
	start := r.Offset()
	protocol, err := ReadProtocolType(r)
	if err != nil {
		return nil, err
	}

	message := &ProtocolMessage{Protocol: protocol}
	switch protocol {
	case ProtocolTypeSync:
		syncMessage, err := ReadSyncMessage(r)
		if err != nil {
			return nil, err
		}
		message.Sync = syncMessage
		return message, nil
	case ProtocolTypeAwareness:
		awarenessMessage, err := readAwarenessMessage(r)
		if err != nil {
			return nil, wrapError("ReadProtocolMessage.awareness", start, err)
		}
		message.Awareness = awarenessMessage
		return message, nil
	default:
		return nil, wrapError("ReadProtocolMessage.protocol", start, fmt.Errorf("%w: %d", ErrUnknownProtocolType, protocol))
	}
}

// DecodeProtocolMessage decodifica uma mensagem completa com envelope e rejeita bytes extras.
func DecodeProtocolMessage(src []byte) (*ProtocolMessage, error) {
	reader := ybinary.NewReader(src)
	message, err := ReadProtocolMessage(reader)
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, wrapError("DecodeProtocolMessage.trailing", reader.Offset(), ErrTrailingBytes)
	}
	return message, nil
}

// AwarenessMessage representa o payload tipado do awareness protocol.
type AwarenessMessage struct {
	Clients []AwarenessClient
}

// AwarenessClient representa uma entrada de client-aware no awareness protocol.
type AwarenessClient struct {
	ClientID uint32
	Clock    uint32
	State    json.RawMessage
}

// IsNull informa se o estado é tombstone/offline (`null`).
func (c AwarenessClient) IsNull() bool {
	return bytes.Equal(c.State, []byte("null"))
}

func readAwarenessMessage(r *ybinary.Reader) (*AwarenessMessage, error) {
	start := r.Offset()
	count, _, err := varint.Read(r)
	if err != nil {
		return nil, wrapError("ReadAwarenessMessage.count", start, err)
	}

	message := &AwarenessMessage{Clients: make([]AwarenessClient, 0, count)}
	for i := uint32(0); i < count; i++ {
		clientStart := r.Offset()
		clientID, _, err := varint.Read(r)
		if err != nil {
			return nil, wrapError("ReadAwarenessMessage.clientID", clientStart, err)
		}

		clockStart := r.Offset()
		clock, _, err := varint.Read(r)
		if err != nil {
			return nil, wrapError("ReadAwarenessMessage.clock", clockStart, err)
		}

		state, err := readAwarenessState(r, "ReadAwarenessMessage.state")
		if err != nil {
			return nil, err
		}

		message.Clients = append(message.Clients, AwarenessClient{
			ClientID: clientID,
			Clock:    clock,
			State:    state,
		})
	}
	return message, nil
}

func readAwarenessState(r *ybinary.Reader, op string) (json.RawMessage, error) {
	start := r.Offset()
	length, _, err := varint.Read(r)
	if err != nil {
		return nil, wrapError(op+".len", start, err)
	}

	raw, err := r.ReadN(int(length))
	if err != nil {
		return nil, wrapError(op, r.Offset(), err)
	}

	state, err := normalizeJSON(raw)
	if err != nil {
		return nil, wrapError(op, start, err)
	}
	return state, nil
}

func normalizeJSON(state json.RawMessage) (json.RawMessage, error) {
	if len(state) == 0 {
		return json.RawMessage("null"), nil
	}

	if !json.Valid(state) {
		return nil, ErrInvalidAwarenessJSON
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, state); err != nil {
		return nil, ErrInvalidAwarenessJSON
	}
	return compact.Bytes(), nil
}
