package yprotocol

import (
	"fmt"

	ybinary "yjs-go-bridge/internal/binary"
)

// QueryAwarenessMessage representa a consulta vazia usada pelo provider para
// solicitar o snapshot atual de awareness.
type QueryAwarenessMessage struct{}

// EncodeProtocolQueryAwareness serializa a consulta de awareness no envelope externo.
func EncodeProtocolQueryAwareness() []byte {
	return AppendProtocolType(nil, ProtocolTypeQueryAwareness)
}

// ReadProtocolQueryAwareness valida o envelope de consulta de awareness.
func ReadProtocolQueryAwareness(r *ybinary.Reader) (*QueryAwarenessMessage, error) {
	start := r.Offset()
	typ, err := ReadProtocolType(r)
	if err != nil {
		return nil, err
	}
	if typ != ProtocolTypeQueryAwareness {
		return nil, wrapError("ReadProtocolQueryAwareness.protocol", start, fmt.Errorf("%w: %s", ErrUnexpectedProtocolType, typ))
	}
	return &QueryAwarenessMessage{}, nil
}

// DecodeProtocolQueryAwareness decodifica uma consulta awareness isolada.
func DecodeProtocolQueryAwareness(src []byte) (*QueryAwarenessMessage, error) {
	reader := ybinary.NewReader(src)
	message, err := ReadProtocolQueryAwareness(reader)
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, wrapError("DecodeProtocolQueryAwareness.trailing", reader.Offset(), ErrTrailingBytes)
	}
	return message, nil
}
