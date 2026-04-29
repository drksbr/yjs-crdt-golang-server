package yprotocol

import (
	"fmt"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
	"github.com/drksbr/yjs-crdt-golang-server/internal/yupdate"
)

// SyncMessageType identifica a mensagem interna do protocolo de sync.
type SyncMessageType uint32

const (
	// SyncMessageTypeStep1 contém um state vector.
	SyncMessageTypeStep1 SyncMessageType = 0
	// SyncMessageTypeStep2 contém um update com o estado faltante.
	SyncMessageTypeStep2 SyncMessageType = 1
	// SyncMessageTypeUpdate contém um update incremental.
	SyncMessageTypeUpdate SyncMessageType = 2
)

// SyncMessage representa uma mensagem de sync já enquadrada em tipo e payload.
// O payload referencia o buffer de entrada quando a mensagem é decodificada.
type SyncMessage struct {
	Type    SyncMessageType
	Payload []byte
}

// AppendSyncMessage escreve uma mensagem interna do protocolo de sync.
func AppendSyncMessage(dst []byte, typ SyncMessageType, payload []byte) ([]byte, error) {
	if !typ.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrUnknownSyncMessageType, typ)
	}
	dst = varint.Append(dst, uint32(typ))
	return appendVarBytes(dst, payload), nil
}

// EncodeSyncMessage retorna uma mensagem interna do protocolo de sync.
func EncodeSyncMessage(typ SyncMessageType, payload []byte) ([]byte, error) {
	return AppendSyncMessage(nil, typ, payload)
}

// EncodeSyncStep1 serializa um SyncStep1 com um state vector já codificado.
func EncodeSyncStep1(stateVector []byte) []byte {
	return encodeKnownSyncMessage(SyncMessageTypeStep1, stateVector)
}

// EncodeSyncStep1FromUpdateV1 gera um SyncStep1 a partir de um update V1 já consolidado.
func EncodeSyncStep1FromUpdateV1(update []byte) ([]byte, error) {
	stateVector, err := yupdate.EncodeStateVectorFromUpdateV1(update)
	if err != nil {
		return nil, err
	}
	return EncodeSyncMessage(SyncMessageTypeStep1, stateVector)
}

// EncodeSyncStep1FromUpdatesV1 gera um SyncStep1 a partir de múltiplos updates
// V1, agregando o state vector observado.
func EncodeSyncStep1FromUpdatesV1(updates ...[]byte) ([]byte, error) {
	stateVector, err := yupdate.EncodeStateVectorFromUpdates(updates...)
	if err != nil {
		return nil, err
	}
	return EncodeSyncMessage(SyncMessageTypeStep1, stateVector)
}

// EncodeSyncStep2 serializa um SyncStep2 com um update binário.
func EncodeSyncStep2(update []byte) []byte {
	return encodeKnownSyncMessage(SyncMessageTypeStep2, update)
}

// EncodeSyncStep2FromUpdatesV1 consolida múltiplos updates V1 e serializa o
// resultado como SyncStep2.
func EncodeSyncStep2FromUpdatesV1(updates ...[]byte) ([]byte, error) {
	merged, err := yupdate.MergeUpdatesV1(updates...)
	if err != nil {
		return nil, err
	}
	return EncodeSyncMessage(SyncMessageTypeStep2, merged)
}

// EncodeSyncUpdate serializa uma mensagem incremental de update.
func EncodeSyncUpdate(update []byte) []byte {
	return encodeKnownSyncMessage(SyncMessageTypeUpdate, update)
}

// EncodeProtocolSyncMessage serializa o envelope externo do protocolo combinado.
func EncodeProtocolSyncMessage(typ SyncMessageType, payload []byte) ([]byte, error) {
	inner, err := EncodeSyncMessage(typ, payload)
	if err != nil {
		return nil, err
	}
	return append(AppendProtocolType(nil, ProtocolTypeSync), inner...), nil
}

// EncodeProtocolSyncStep1 serializa uma mensagem combinada de protocolo + SyncStep1.
func EncodeProtocolSyncStep1(stateVector []byte) []byte {
	return append(AppendProtocolType(nil, ProtocolTypeSync), EncodeSyncStep1(stateVector)...)
}

// EncodeProtocolSyncStep1FromUpdateV1 gera uma mensagem combinada de protocolo + SyncStep1.
func EncodeProtocolSyncStep1FromUpdateV1(update []byte) ([]byte, error) {
	stateVector, err := yupdate.EncodeStateVectorFromUpdateV1(update)
	if err != nil {
		return nil, err
	}
	return EncodeProtocolSyncMessage(SyncMessageTypeStep1, stateVector)
}

// EncodeProtocolSyncStep1FromUpdatesV1 gera uma mensagem combinada de protocolo
// + SyncStep1 a partir de múltiplos updates V1.
func EncodeProtocolSyncStep1FromUpdatesV1(updates ...[]byte) ([]byte, error) {
	stateVector, err := yupdate.EncodeStateVectorFromUpdates(updates...)
	if err != nil {
		return nil, err
	}
	return EncodeProtocolSyncMessage(SyncMessageTypeStep1, stateVector)
}

// EncodeProtocolSyncStep2 serializa uma mensagem combinada de protocolo + SyncStep2.
func EncodeProtocolSyncStep2(update []byte) []byte {
	return append(AppendProtocolType(nil, ProtocolTypeSync), EncodeSyncStep2(update)...)
}

// EncodeProtocolSyncStep2FromUpdatesV1 consolida múltiplos updates V1 e
// serializa o resultado no envelope combinado do protocolo.
func EncodeProtocolSyncStep2FromUpdatesV1(updates ...[]byte) ([]byte, error) {
	merged, err := yupdate.MergeUpdatesV1(updates...)
	if err != nil {
		return nil, err
	}
	return EncodeProtocolSyncMessage(SyncMessageTypeStep2, merged)
}

// EncodeProtocolSyncUpdate serializa uma mensagem combinada de protocolo + Update.
func EncodeProtocolSyncUpdate(update []byte) []byte {
	return append(AppendProtocolType(nil, ProtocolTypeSync), EncodeSyncUpdate(update)...)
}

// ReadSyncMessage lê uma mensagem interna do protocolo de sync a partir de um stream.
func ReadSyncMessage(r *ybinary.Reader) (*SyncMessage, error) {
	start := r.Offset()
	value, _, err := varint.Read(r)
	if err != nil {
		return nil, wrapError("ReadSyncMessage.type", start, err)
	}

	typ := SyncMessageType(value)
	if !typ.Valid() {
		return nil, wrapError("ReadSyncMessage.type", start, fmt.Errorf("%w: %d", ErrUnknownSyncMessageType, value))
	}

	payload, err := readVarBytes(r, "ReadSyncMessage.payload")
	if err != nil {
		return nil, err
	}
	return &SyncMessage{Type: typ, Payload: payload}, nil
}

// ReadProtocolSyncMessage lê uma mensagem enquadrada no envelope externo combinado.
func ReadProtocolSyncMessage(r *ybinary.Reader) (*SyncMessage, error) {
	start := r.Offset()
	typ, err := ReadProtocolType(r)
	if err != nil {
		return nil, err
	}
	if typ != ProtocolTypeSync {
		return nil, wrapError("ReadProtocolSyncMessage.protocol", start, fmt.Errorf("%w: %s", ErrUnexpectedProtocolType, typ))
	}
	return ReadSyncMessage(r)
}

// DecodeSyncMessage decodifica uma mensagem interna isolada e rejeita bytes extras.
func DecodeSyncMessage(src []byte) (*SyncMessage, error) {
	reader := ybinary.NewReader(src)
	message, err := ReadSyncMessage(reader)
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, wrapError("DecodeSyncMessage.trailing", reader.Offset(), ErrTrailingBytes)
	}
	return message, nil
}

// DecodeProtocolSyncMessage decodifica uma mensagem completa no envelope externo.
func DecodeProtocolSyncMessage(src []byte) (*SyncMessage, error) {
	reader := ybinary.NewReader(src)
	message, err := ReadProtocolSyncMessage(reader)
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, wrapError("DecodeProtocolSyncMessage.trailing", reader.Offset(), ErrTrailingBytes)
	}
	return message, nil
}

// Valid informa se o subtipo de sync é reconhecido.
func (t SyncMessageType) Valid() bool {
	switch t {
	case SyncMessageTypeStep1, SyncMessageTypeStep2, SyncMessageTypeUpdate:
		return true
	default:
		return false
	}
}

// String retorna a representação textual conhecida do subtipo de sync.
func (t SyncMessageType) String() string {
	switch t {
	case SyncMessageTypeStep1:
		return "sync-step-1"
	case SyncMessageTypeStep2:
		return "sync-step-2"
	case SyncMessageTypeUpdate:
		return "sync-update"
	default:
		return fmt.Sprintf("unknown(%d)", uint32(t))
	}
}

func appendVarBytes(dst, payload []byte) []byte {
	dst = varint.Append(dst, uint32(len(payload)))
	return append(dst, payload...)
}

func encodeKnownSyncMessage(typ SyncMessageType, payload []byte) []byte {
	dst := varint.Append(nil, uint32(typ))
	return appendVarBytes(dst, payload)
}

func readVarBytes(r *ybinary.Reader, op string) ([]byte, error) {
	start := r.Offset()
	length, _, err := varint.Read(r)
	if err != nil {
		return nil, wrapError(op+".len", start, err)
	}

	payload, err := r.ReadN(int(length))
	if err != nil {
		return nil, wrapError(op, r.Offset(), err)
	}
	return payload, nil
}
