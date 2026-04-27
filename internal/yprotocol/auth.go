package yprotocol

import (
	"fmt"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/internal/varint"
)

// AuthMessageType identifica subtipos do protocolo interno de auth.
type AuthMessageType uint32

const (
	// AuthMessageTypePermissionDenied sinaliza que o acesso foi negado.
	AuthMessageTypePermissionDenied AuthMessageType = 0
)

// AuthMessage representa a mensagem interna de auth suportada pelo codec.
type AuthMessage struct {
	Type   AuthMessageType
	Reason string
}

// AppendAuthMessage serializa uma mensagem interna de auth.
func AppendAuthMessage(dst []byte, typ AuthMessageType, reason string) ([]byte, error) {
	if !typ.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrUnknownAuthMessageType, typ)
	}

	dst = varint.Append(dst, uint32(typ))
	switch typ {
	case AuthMessageTypePermissionDenied:
		return appendVarBytes(dst, []byte(reason)), nil
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownAuthMessageType, typ)
	}
}

// EncodeAuthMessage serializa uma mensagem interna de auth.
func EncodeAuthMessage(typ AuthMessageType, reason string) ([]byte, error) {
	return AppendAuthMessage(nil, typ, reason)
}

// EncodeAuthPermissionDenied serializa a mensagem de permissão negada.
func EncodeAuthPermissionDenied(reason string) []byte {
	dst := varint.Append(nil, uint32(AuthMessageTypePermissionDenied))
	return appendVarBytes(dst, []byte(reason))
}

// EncodeProtocolAuthMessage serializa uma mensagem auth no envelope externo.
func EncodeProtocolAuthMessage(typ AuthMessageType, reason string) ([]byte, error) {
	inner, err := EncodeAuthMessage(typ, reason)
	if err != nil {
		return nil, err
	}
	return append(AppendProtocolType(nil, ProtocolTypeAuth), inner...), nil
}

// EncodeProtocolAuthPermissionDenied serializa uma mensagem auth permission denied.
func EncodeProtocolAuthPermissionDenied(reason string) []byte {
	return append(AppendProtocolType(nil, ProtocolTypeAuth), EncodeAuthPermissionDenied(reason)...)
}

// ReadAuthMessage lê uma mensagem interna de auth.
func ReadAuthMessage(r *ybinary.Reader) (*AuthMessage, error) {
	start := r.Offset()
	value, _, err := varint.Read(r)
	if err != nil {
		return nil, wrapError("ReadAuthMessage.type", start, err)
	}

	typ := AuthMessageType(value)
	if !typ.Valid() {
		return nil, wrapError("ReadAuthMessage.type", start, fmt.Errorf("%w: %d", ErrUnknownAuthMessageType, value))
	}

	switch typ {
	case AuthMessageTypePermissionDenied:
		reasonBytes, err := readVarBytes(r, "ReadAuthMessage.reason")
		if err != nil {
			return nil, err
		}
		return &AuthMessage{Type: typ, Reason: string(reasonBytes)}, nil
	default:
		return nil, wrapError("ReadAuthMessage.type", start, fmt.Errorf("%w: %d", ErrUnknownAuthMessageType, value))
	}
}

// ReadProtocolAuthMessage lê uma mensagem auth enquadrada no envelope externo.
func ReadProtocolAuthMessage(r *ybinary.Reader) (*AuthMessage, error) {
	start := r.Offset()
	typ, err := ReadProtocolType(r)
	if err != nil {
		return nil, err
	}
	if typ != ProtocolTypeAuth {
		return nil, wrapError("ReadProtocolAuthMessage.protocol", start, fmt.Errorf("%w: %s", ErrUnexpectedProtocolType, typ))
	}
	return ReadAuthMessage(r)
}

// DecodeAuthMessage decodifica uma mensagem auth isolada e rejeita bytes extras.
func DecodeAuthMessage(src []byte) (*AuthMessage, error) {
	reader := ybinary.NewReader(src)
	message, err := ReadAuthMessage(reader)
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, wrapError("DecodeAuthMessage.trailing", reader.Offset(), ErrTrailingBytes)
	}
	return message, nil
}

// DecodeProtocolAuthMessage decodifica uma mensagem auth com envelope externo.
func DecodeProtocolAuthMessage(src []byte) (*AuthMessage, error) {
	reader := ybinary.NewReader(src)
	message, err := ReadProtocolAuthMessage(reader)
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, wrapError("DecodeProtocolAuthMessage.trailing", reader.Offset(), ErrTrailingBytes)
	}
	return message, nil
}

// Valid informa se o subtipo auth é reconhecido.
func (t AuthMessageType) Valid() bool {
	switch t {
	case AuthMessageTypePermissionDenied:
		return true
	default:
		return false
	}
}

// String retorna a representação textual conhecida do subtipo de auth.
func (t AuthMessageType) String() string {
	switch t {
	case AuthMessageTypePermissionDenied:
		return "permission-denied"
	default:
		return fmt.Sprintf("unknown(%d)", uint32(t))
	}
}
