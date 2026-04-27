package yawareness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/yprotocol"
)

var (
	// ErrInvalidJSON sinaliza estado awareness com JSON inválido.
	ErrInvalidJSON = errors.New("yawareness: estado json invalido")
	// ErrTrailingBytes sinaliza bytes extras após uma mensagem awareness isolada.
	ErrTrailingBytes = errors.New("yawareness: bytes residuais ao final da mensagem")
)

// ParseError adiciona contexto e offset às falhas de parsing do protocolo de awareness.
type ParseError struct {
	Op     string
	Offset int
	Err    error
}

// Error retorna a mensagem formatada do erro com contexto e offset.
func (e *ParseError) Error() string {
	return fmt.Sprintf("yawareness: %s falhou no offset %d: %v", e.Op, e.Offset, e.Err)
}

// Unwrap expõe o erro interno para `errors.Is` e `errors.As`.
func (e *ParseError) Unwrap() error {
	return e.Err
}

// ClientState representa uma entrada do awareness protocol para um cliente.
type ClientState struct {
	ClientID uint32          `json:"clientID"`
	Clock    uint32          `json:"clock"`
	State    json.RawMessage `json:"state"`
}

// IsNull informa se o estado representa o tombstone/offline (`null`).
func (c ClientState) IsNull() bool {
	return bytes.Equal(c.State, []byte("null"))
}

// Update representa o payload completo do awareness protocol.
type Update struct {
	Clients []ClientState `json:"clients"`
}

// AppendUpdate serializa um payload awareness sem o envelope externo do protocolo.
func AppendUpdate(dst []byte, update *Update) ([]byte, error) {
	if update == nil {
		update = &Update{}
	}

	dst = varint.Append(dst, uint32(len(update.Clients)))
	for _, client := range update.Clients {
		state, err := normalizeState(client.State)
		if err != nil {
			return nil, err
		}
		dst = varint.Append(dst, client.ClientID)
		dst = varint.Append(dst, client.Clock)
		dst = appendVarString(dst, state)
	}
	return dst, nil
}

// EncodeUpdate serializa um payload awareness sem o envelope externo.
func EncodeUpdate(update *Update) ([]byte, error) {
	return AppendUpdate(nil, update)
}

// EncodeProtocolUpdate serializa uma mensagem awareness no envelope combinado de y-protocols.
func EncodeProtocolUpdate(update *Update) ([]byte, error) {
	payload, err := EncodeUpdate(update)
	if err != nil {
		return nil, err
	}
	return append(yprotocol.AppendProtocolType(nil, yprotocol.ProtocolTypeAwareness), payload...), nil
}

// ReadUpdate lê um payload awareness de um stream.
func ReadUpdate(r *ybinary.Reader) (*Update, error) {
	start := r.Offset()
	count, _, err := varint.Read(r)
	if err != nil {
		return nil, wrapError("ReadUpdate.count", start, err)
	}

	update := &Update{Clients: make([]ClientState, 0, count)}
	for i := uint32(0); i < count; i++ {
		clientStart := r.Offset()
		clientID, _, err := varint.Read(r)
		if err != nil {
			return nil, wrapError("ReadUpdate.clientID", clientStart, err)
		}

		clockStart := r.Offset()
		clock, _, err := varint.Read(r)
		if err != nil {
			return nil, wrapError("ReadUpdate.clock", clockStart, err)
		}

		state, err := readVarStringJSON(r, "ReadUpdate.state")
		if err != nil {
			return nil, err
		}

		update.Clients = append(update.Clients, ClientState{
			ClientID: clientID,
			Clock:    clock,
			State:    state,
		})
	}
	return update, nil
}

// ReadProtocolUpdate lê uma mensagem awareness já enquadrada no protocolo combinado.
func ReadProtocolUpdate(r *ybinary.Reader) (*Update, error) {
	start := r.Offset()
	typ, err := yprotocol.ReadProtocolType(r)
	if err != nil {
		return nil, err
	}
	if typ != yprotocol.ProtocolTypeAwareness {
		return nil, wrapError("ReadProtocolUpdate.protocol", start, fmt.Errorf("%w: %s", yprotocol.ErrUnexpectedProtocolType, typ))
	}
	return ReadUpdate(r)
}

// DecodeUpdate decodifica um payload awareness isolado e rejeita bytes extras.
func DecodeUpdate(src []byte) (*Update, error) {
	reader := ybinary.NewReader(src)
	update, err := ReadUpdate(reader)
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, wrapError("DecodeUpdate.trailing", reader.Offset(), ErrTrailingBytes)
	}
	return update, nil
}

// DecodeProtocolUpdate decodifica uma mensagem awareness completa com envelope.
func DecodeProtocolUpdate(src []byte) (*Update, error) {
	reader := ybinary.NewReader(src)
	update, err := ReadProtocolUpdate(reader)
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, wrapError("DecodeProtocolUpdate.trailing", reader.Offset(), ErrTrailingBytes)
	}
	return update, nil
}

func appendVarString(dst []byte, value []byte) []byte {
	dst = varint.Append(dst, uint32(len(value)))
	return append(dst, value...)
}

func normalizeState(state json.RawMessage) (json.RawMessage, error) {
	if len(state) == 0 {
		return json.RawMessage("null"), nil
	}
	if !json.Valid(state) {
		return nil, ErrInvalidJSON
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, state); err != nil {
		return nil, ErrInvalidJSON
	}
	return compact.Bytes(), nil
}

func readVarStringJSON(r *ybinary.Reader, op string) (json.RawMessage, error) {
	start := r.Offset()
	length, _, err := varint.Read(r)
	if err != nil {
		return nil, wrapError(op+".len", start, err)
	}

	data, err := r.ReadN(int(length))
	if err != nil {
		return nil, wrapError(op, r.Offset(), err)
	}

	state, err := normalizeState(data)
	if err != nil {
		return nil, wrapError(op, start, err)
	}
	return state, nil
}

func wrapError(op string, offset int, err error) error {
	if err == nil {
		return nil
	}
	return &ParseError{Op: op, Offset: offset, Err: err}
}
