package yprotocol

import (
	"fmt"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/internal/varint"
)

// ProtocolType identifica a camada externa combinada descrita em y-protocols/PROTOCOL.md.
type ProtocolType uint32

const (
	// ProtocolTypeSync carrega mensagens do protocolo de sync.
	ProtocolTypeSync ProtocolType = 0
	// ProtocolTypeAwareness carrega mensagens do protocolo de awareness.
	ProtocolTypeAwareness ProtocolType = 1
)

// AppendProtocolType escreve o prefixo do protocolo externo.
func AppendProtocolType(dst []byte, typ ProtocolType) []byte {
	return varint.Append(dst, uint32(typ))
}

// ReadProtocolType lê e valida o prefixo do protocolo externo.
func ReadProtocolType(r *ybinary.Reader) (ProtocolType, error) {
	start := r.Offset()
	value, _, err := varint.Read(r)
	if err != nil {
		return 0, wrapError("ReadProtocolType", start, err)
	}

	typ := ProtocolType(value)
	if !typ.Valid() {
		return 0, wrapError("ReadProtocolType", start, fmt.Errorf("%w: %d", ErrUnknownProtocolType, value))
	}
	return typ, nil
}

// Valid informa se o protocolo é reconhecido pelo port atual.
func (t ProtocolType) Valid() bool {
	switch t {
	case ProtocolTypeSync, ProtocolTypeAwareness:
		return true
	default:
		return false
	}
}

func (t ProtocolType) String() string {
	switch t {
	case ProtocolTypeSync:
		return "sync"
	case ProtocolTypeAwareness:
		return "awareness"
	default:
		return fmt.Sprintf("unknown(%d)", uint32(t))
	}
}
