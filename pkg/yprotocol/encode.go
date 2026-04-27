package yprotocol

import (
	"errors"
	"fmt"
)

var (
	// ErrNilProtocolMessage sinaliza tentativa de encode/handle de mensagem nula.
	ErrNilProtocolMessage = errors.New("yprotocol: protocol message nao pode ser nil")
	// ErrInvalidProtocolMessage sinaliza shape inconsistente entre envelope e payload.
	ErrInvalidProtocolMessage = errors.New("yprotocol: protocol message invalida")
	// ErrNilSession sinaliza uso de Session nula.
	ErrNilSession = errors.New("yprotocol: session nao pode ser nil")
)

// EncodeProtocolEnvelope serializa uma mensagem tipada do envelope y-protocols.
func EncodeProtocolEnvelope(message *ProtocolMessage) ([]byte, error) {
	if err := validateProtocolMessage(message); err != nil {
		return nil, err
	}

	switch message.Protocol {
	case ProtocolTypeSync:
		return EncodeProtocolSyncMessage(message.Sync.Type, message.Sync.Payload)
	case ProtocolTypeAwareness:
		return EncodeProtocolAwarenessUpdate(message.Awareness)
	case ProtocolTypeAuth:
		return EncodeProtocolAuthMessage(message.Auth.Type, message.Auth.Reason)
	case ProtocolTypeQueryAwareness:
		return EncodeProtocolQueryAwareness(), nil
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownProtocolType, message.Protocol)
	}
}

// EncodeProtocolEnvelopes serializa um stream concatenado de mensagens tipadas.
func EncodeProtocolEnvelopes(messages ...*ProtocolMessage) ([]byte, error) {
	dst := make([]byte, 0)
	for idx, message := range messages {
		encoded, err := EncodeProtocolEnvelope(message)
		if err != nil {
			return nil, fmt.Errorf("encode protocol envelope %d: %w", idx, err)
		}
		dst = append(dst, encoded...)
	}
	return dst, nil
}

func validateProtocolMessage(message *ProtocolMessage) error {
	if message == nil {
		return ErrNilProtocolMessage
	}

	populated := 0
	if message.Sync != nil {
		populated++
	}
	if message.Awareness != nil {
		populated++
	}
	if message.Auth != nil {
		populated++
	}
	if message.QueryAwareness != nil {
		populated++
	}

	switch message.Protocol {
	case ProtocolTypeSync:
		if message.Sync == nil || populated != 1 {
			return fmt.Errorf("%w: protocolo sync exige exatamente um payload sync", ErrInvalidProtocolMessage)
		}
	case ProtocolTypeAwareness:
		if message.Awareness == nil || populated != 1 {
			return fmt.Errorf("%w: protocolo awareness exige exatamente um payload awareness", ErrInvalidProtocolMessage)
		}
	case ProtocolTypeAuth:
		if message.Auth == nil || populated != 1 {
			return fmt.Errorf("%w: protocolo auth exige exatamente um payload auth", ErrInvalidProtocolMessage)
		}
	case ProtocolTypeQueryAwareness:
		if populated > 1 || message.Sync != nil || message.Awareness != nil || message.Auth != nil {
			return fmt.Errorf("%w: query-awareness nao aceita payload extra", ErrInvalidProtocolMessage)
		}
	default:
		return fmt.Errorf("%w: %d", ErrUnknownProtocolType, message.Protocol)
	}

	return nil
}
