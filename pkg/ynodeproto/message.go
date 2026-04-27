package ynodeproto

import (
	"encoding/binary"
	"fmt"
	"strings"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/pkg/storage"
)

const fixedUint64Size = 8

type routedPayload struct {
	DocumentKey  storage.DocumentKey
	ConnectionID string
	Epoch        uint64
	Body         []byte
}

// Message representa um payload tipado do protocolo inter-node.
//
// As implementações públicas desse contrato são as structs exportadas deste
// pacote. O método não exportado fecha o conjunto de implementações aceitas
// pelo codec.
type Message interface {
	Type() MessageType
	FrameFlags() Flags
	Validate() error
	appendPayload(dst []byte) ([]byte, error)
}

// ParseError adiciona contexto e offset às falhas de parsing de payloads tipados.
type ParseError struct {
	Op     string
	Offset int
	Err    error
}

// Error retorna a mensagem formatada do erro com contexto e offset.
func (e *ParseError) Error() string {
	return fmt.Sprintf("ynodeproto: %s falhou no offset %d: %v", e.Op, e.Offset, e.Err)
}

// Unwrap expõe o erro interno para `errors.Is` e `errors.As`.
func (e *ParseError) Unwrap() error {
	return e.Err
}

// Handshake inicia a negociação de identidade e roteamento de uma conexão de
// documento entre nós.
type Handshake struct {
	Flags        Flags
	NodeID       string
	DocumentKey  storage.DocumentKey
	ConnectionID string
	Epoch        uint64
}

// Type retorna o message type associado ao payload.
func (m *Handshake) Type() MessageType {
	return MessageTypeHandshake
}

// FrameFlags retorna os bits auxiliares do header preservados por esta mensagem.
func (m *Handshake) FrameFlags() Flags {
	if m == nil {
		return FlagNone
	}
	return m.Flags
}

// Validate confirma se o payload contém os campos mínimos exigidos pelo wire.
func (m *Handshake) Validate() error {
	if m == nil {
		return ErrNilMessage
	}
	return validateNodeRoute(m.NodeID, m.DocumentKey, m.ConnectionID, m.Epoch)
}

func (m *Handshake) appendPayload(dst []byte) ([]byte, error) {
	var err error

	dst, err = appendString(dst, m.NodeID)
	if err != nil {
		return nil, err
	}
	dst, err = appendDocumentKey(dst, m.DocumentKey)
	if err != nil {
		return nil, err
	}
	dst, err = appendString(dst, m.ConnectionID)
	if err != nil {
		return nil, err
	}
	return appendUint64(dst, m.Epoch), nil
}

// HandshakeAck confirma o handshake e espelha o contexto roteável aceito.
type HandshakeAck struct {
	Flags        Flags
	NodeID       string
	DocumentKey  storage.DocumentKey
	ConnectionID string
	Epoch        uint64
}

// Type retorna o message type associado ao payload.
func (m *HandshakeAck) Type() MessageType {
	return MessageTypeHandshakeAck
}

// FrameFlags retorna os bits auxiliares do header preservados por esta mensagem.
func (m *HandshakeAck) FrameFlags() Flags {
	if m == nil {
		return FlagNone
	}
	return m.Flags
}

// Validate confirma se o payload contém os campos mínimos exigidos pelo wire.
func (m *HandshakeAck) Validate() error {
	if m == nil {
		return ErrNilMessage
	}
	return validateNodeRoute(m.NodeID, m.DocumentKey, m.ConnectionID, m.Epoch)
}

func (m *HandshakeAck) appendPayload(dst []byte) ([]byte, error) {
	var err error

	dst, err = appendString(dst, m.NodeID)
	if err != nil {
		return nil, err
	}
	dst, err = appendDocumentKey(dst, m.DocumentKey)
	if err != nil {
		return nil, err
	}
	dst, err = appendString(dst, m.ConnectionID)
	if err != nil {
		return nil, err
	}
	return appendUint64(dst, m.Epoch), nil
}

// DocumentSyncRequest solicita catch-up de um documento e carrega o state
// vector bruto do solicitante.
type DocumentSyncRequest struct {
	Flags        Flags
	DocumentKey  storage.DocumentKey
	ConnectionID string
	Epoch        uint64
	StateVector  []byte
}

// Type retorna o message type associado ao payload.
func (m *DocumentSyncRequest) Type() MessageType {
	return MessageTypeDocumentSyncRequest
}

// FrameFlags retorna os bits auxiliares do header preservados por esta mensagem.
func (m *DocumentSyncRequest) FrameFlags() Flags {
	if m == nil {
		return FlagNone
	}
	return m.Flags
}

// Validate confirma se o payload contém os campos mínimos exigidos pelo wire.
func (m *DocumentSyncRequest) Validate() error {
	if m == nil {
		return ErrNilMessage
	}
	return validateRoutedBody(m.DocumentKey, m.ConnectionID, m.Epoch, m.StateVector)
}

func (m *DocumentSyncRequest) appendPayload(dst []byte) ([]byte, error) {
	return appendRoutedPayload(dst, m.DocumentKey, m.ConnectionID, m.Epoch, m.StateVector)
}

// DocumentSyncResponse entrega o update V1 necessário para sincronizar um
// documento.
type DocumentSyncResponse struct {
	Flags        Flags
	DocumentKey  storage.DocumentKey
	ConnectionID string
	Epoch        uint64
	UpdateV1     []byte
}

// Type retorna o message type associado ao payload.
func (m *DocumentSyncResponse) Type() MessageType {
	return MessageTypeDocumentSyncResponse
}

// FrameFlags retorna os bits auxiliares do header preservados por esta mensagem.
func (m *DocumentSyncResponse) FrameFlags() Flags {
	if m == nil {
		return FlagNone
	}
	return m.Flags
}

// Validate confirma se o payload contém os campos mínimos exigidos pelo wire.
func (m *DocumentSyncResponse) Validate() error {
	if m == nil {
		return ErrNilMessage
	}
	return validateRoutedBody(m.DocumentKey, m.ConnectionID, m.Epoch, m.UpdateV1)
}

func (m *DocumentSyncResponse) appendPayload(dst []byte) ([]byte, error) {
	return appendRoutedPayload(dst, m.DocumentKey, m.ConnectionID, m.Epoch, m.UpdateV1)
}

// DocumentUpdate carrega um update V1 incremental de documento.
type DocumentUpdate struct {
	Flags        Flags
	DocumentKey  storage.DocumentKey
	ConnectionID string
	Epoch        uint64
	UpdateV1     []byte
}

// Type retorna o message type associado ao payload.
func (m *DocumentUpdate) Type() MessageType {
	return MessageTypeDocumentUpdate
}

// FrameFlags retorna os bits auxiliares do header preservados por esta mensagem.
func (m *DocumentUpdate) FrameFlags() Flags {
	if m == nil {
		return FlagNone
	}
	return m.Flags
}

// Validate confirma se o payload contém os campos mínimos exigidos pelo wire.
func (m *DocumentUpdate) Validate() error {
	if m == nil {
		return ErrNilMessage
	}
	return validateRoutedBody(m.DocumentKey, m.ConnectionID, m.Epoch, m.UpdateV1)
}

func (m *DocumentUpdate) appendPayload(dst []byte) ([]byte, error) {
	return appendRoutedPayload(dst, m.DocumentKey, m.ConnectionID, m.Epoch, m.UpdateV1)
}

// AwarenessUpdate carrega um delta bruto de awareness associado a uma conexão
// já roteada.
type AwarenessUpdate struct {
	Flags        Flags
	DocumentKey  storage.DocumentKey
	ConnectionID string
	Epoch        uint64
	Payload      []byte
}

// Type retorna o message type associado ao payload.
func (m *AwarenessUpdate) Type() MessageType {
	return MessageTypeAwarenessUpdate
}

// FrameFlags retorna os bits auxiliares do header preservados por esta mensagem.
func (m *AwarenessUpdate) FrameFlags() Flags {
	if m == nil {
		return FlagNone
	}
	return m.Flags
}

// Validate confirma se o payload contém os campos mínimos exigidos pelo wire.
func (m *AwarenessUpdate) Validate() error {
	if m == nil {
		return ErrNilMessage
	}
	return validateRoutedBody(m.DocumentKey, m.ConnectionID, m.Epoch, m.Payload)
}

func (m *AwarenessUpdate) appendPayload(dst []byte) ([]byte, error) {
	return appendRoutedPayload(dst, m.DocumentKey, m.ConnectionID, m.Epoch, m.Payload)
}

// Ping carrega um nonce correlacionável para keepalive/latência.
type Ping struct {
	Flags Flags
	Nonce uint64
}

// Type retorna o message type associado ao payload.
func (m *Ping) Type() MessageType {
	return MessageTypePing
}

// FrameFlags retorna os bits auxiliares do header preservados por esta mensagem.
func (m *Ping) FrameFlags() Flags {
	if m == nil {
		return FlagNone
	}
	return m.Flags
}

// Validate confirma se o payload contém os campos mínimos exigidos pelo wire.
func (m *Ping) Validate() error {
	if m == nil {
		return ErrNilMessage
	}
	if m.Nonce == 0 {
		return ErrInvalidNonce
	}
	return nil
}

func (m *Ping) appendPayload(dst []byte) ([]byte, error) {
	return appendUint64(dst, m.Nonce), nil
}

// Pong responde a um ping previamente emitido e repete o nonce recebido.
type Pong struct {
	Flags Flags
	Nonce uint64
}

// Type retorna o message type associado ao payload.
func (m *Pong) Type() MessageType {
	return MessageTypePong
}

// FrameFlags retorna os bits auxiliares do header preservados por esta mensagem.
func (m *Pong) FrameFlags() Flags {
	if m == nil {
		return FlagNone
	}
	return m.Flags
}

// Validate confirma se o payload contém os campos mínimos exigidos pelo wire.
func (m *Pong) Validate() error {
	if m == nil {
		return ErrNilMessage
	}
	if m.Nonce == 0 {
		return ErrInvalidNonce
	}
	return nil
}

func (m *Pong) appendPayload(dst []byte) ([]byte, error) {
	return appendUint64(dst, m.Nonce), nil
}

// EncodeMessagePayload serializa apenas o payload tipado, sem o frame externo.
func EncodeMessagePayload(message Message) ([]byte, error) {
	if message == nil {
		return nil, ErrNilMessage
	}
	if err := message.Validate(); err != nil {
		return nil, err
	}
	return message.appendPayload(nil)
}

// NewMessageFrame constrói um frame validado a partir de uma mensagem tipada.
func NewMessageFrame(message Message) (*Frame, error) {
	payload, err := EncodeMessagePayload(message)
	if err != nil {
		return nil, err
	}
	return NewFrame(message.Type(), message.FrameFlags(), payload)
}

// EncodeMessageFrame serializa uma mensagem tipada como frame completo.
func EncodeMessageFrame(message Message) ([]byte, error) {
	frame, err := NewMessageFrame(message)
	if err != nil {
		return nil, err
	}
	return EncodeFrame(frame)
}

// DecodeMessagePayload decodifica um payload tipado isolado para a mensagem
// concreta correspondente ao `typ` informado.
func DecodeMessagePayload(typ MessageType, flags Flags, payload []byte) (Message, error) {
	if !typ.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrUnknownMessageType, uint8(typ))
	}

	reader := ybinary.NewReader(payload)
	var (
		message Message
		err     error
	)

	switch typ {
	case MessageTypeHandshake:
		message, err = decodeHandshake(reader, flags)
	case MessageTypeHandshakeAck:
		message, err = decodeHandshakeAck(reader, flags)
	case MessageTypeDocumentSyncRequest:
		message, err = decodeDocumentSyncRequest(reader, flags)
	case MessageTypeDocumentSyncResponse:
		message, err = decodeDocumentSyncResponse(reader, flags)
	case MessageTypeDocumentUpdate:
		message, err = decodeDocumentUpdate(reader, flags)
	case MessageTypeAwarenessUpdate:
		message, err = decodeAwarenessUpdate(reader, flags)
	case MessageTypePing:
		message, err = decodePing(reader, flags)
	case MessageTypePong:
		message, err = decodePong(reader, flags)
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownMessageType, uint8(typ))
	}
	if err != nil {
		return nil, err
	}
	if reader.Remaining() != 0 {
		return nil, wrapParseError("DecodeMessagePayload.trailing", reader.Offset(), ErrTrailingPayloadBytes)
	}
	return message, nil
}

// DecodeFrameMessage decodifica um frame já lido para a mensagem tipada contida
// em seu payload.
func DecodeFrameMessage(frame *Frame) (Message, error) {
	if err := frame.Validate(); err != nil {
		return nil, err
	}
	return DecodeMessagePayload(frame.Header.Type, frame.Header.Flags, frame.Payload)
}

// DecodeMessageFrame decodifica exatamente um frame e materializa a mensagem
// tipada contida nele.
func DecodeMessageFrame(src []byte) (Message, error) {
	frame, err := DecodeFrame(src)
	if err != nil {
		return nil, err
	}
	return DecodeFrameMessage(frame)
}

func decodeHandshake(r *ybinary.Reader, flags Flags) (*Handshake, error) {
	nodeID, err := readNodeID(r, "ReadHandshake.nodeID")
	if err != nil {
		return nil, err
	}
	key, err := readDocumentKey(r, "ReadHandshake.documentKey")
	if err != nil {
		return nil, err
	}
	connectionID, err := readString(r, "ReadHandshake.connectionID")
	if err != nil {
		return nil, err
	}
	epoch, err := readUint64(r, "ReadHandshake.epoch")
	if err != nil {
		return nil, err
	}

	message := &Handshake{
		Flags:        flags,
		NodeID:       nodeID,
		DocumentKey:  key,
		ConnectionID: connectionID,
		Epoch:        epoch,
	}
	if err := message.Validate(); err != nil {
		return nil, wrapParseError("ReadHandshake.validate", r.Offset(), err)
	}
	return message, nil
}

func decodeHandshakeAck(r *ybinary.Reader, flags Flags) (*HandshakeAck, error) {
	nodeID, err := readNodeID(r, "ReadHandshakeAck.nodeID")
	if err != nil {
		return nil, err
	}
	key, err := readDocumentKey(r, "ReadHandshakeAck.documentKey")
	if err != nil {
		return nil, err
	}
	connectionID, err := readString(r, "ReadHandshakeAck.connectionID")
	if err != nil {
		return nil, err
	}
	epoch, err := readUint64(r, "ReadHandshakeAck.epoch")
	if err != nil {
		return nil, err
	}

	message := &HandshakeAck{
		Flags:        flags,
		NodeID:       nodeID,
		DocumentKey:  key,
		ConnectionID: connectionID,
		Epoch:        epoch,
	}
	if err := message.Validate(); err != nil {
		return nil, wrapParseError("ReadHandshakeAck.validate", r.Offset(), err)
	}
	return message, nil
}

func decodeDocumentSyncRequest(r *ybinary.Reader, flags Flags) (*DocumentSyncRequest, error) {
	routed, err := readRoutedPayload(
		r,
		"ReadDocumentSyncRequest.documentKey",
		"ReadDocumentSyncRequest.connectionID",
		"ReadDocumentSyncRequest.epoch",
	)
	if err != nil {
		return nil, err
	}
	message := &DocumentSyncRequest{
		Flags:        flags,
		DocumentKey:  routed.DocumentKey,
		ConnectionID: routed.ConnectionID,
		Epoch:        routed.Epoch,
		StateVector:  routed.Body,
	}
	if err := message.Validate(); err != nil {
		return nil, wrapParseError("ReadDocumentSyncRequest.validate", r.Offset(), err)
	}
	return message, nil
}

func decodeDocumentSyncResponse(r *ybinary.Reader, flags Flags) (*DocumentSyncResponse, error) {
	routed, err := readRoutedPayload(
		r,
		"ReadDocumentSyncResponse.documentKey",
		"ReadDocumentSyncResponse.connectionID",
		"ReadDocumentSyncResponse.epoch",
	)
	if err != nil {
		return nil, err
	}
	message := &DocumentSyncResponse{
		Flags:        flags,
		DocumentKey:  routed.DocumentKey,
		ConnectionID: routed.ConnectionID,
		Epoch:        routed.Epoch,
		UpdateV1:     routed.Body,
	}
	if err := message.Validate(); err != nil {
		return nil, wrapParseError("ReadDocumentSyncResponse.validate", r.Offset(), err)
	}
	return message, nil
}

func decodeDocumentUpdate(r *ybinary.Reader, flags Flags) (*DocumentUpdate, error) {
	routed, err := readRoutedPayload(
		r,
		"ReadDocumentUpdate.documentKey",
		"ReadDocumentUpdate.connectionID",
		"ReadDocumentUpdate.epoch",
	)
	if err != nil {
		return nil, err
	}
	message := &DocumentUpdate{
		Flags:        flags,
		DocumentKey:  routed.DocumentKey,
		ConnectionID: routed.ConnectionID,
		Epoch:        routed.Epoch,
		UpdateV1:     routed.Body,
	}
	if err := message.Validate(); err != nil {
		return nil, wrapParseError("ReadDocumentUpdate.validate", r.Offset(), err)
	}
	return message, nil
}

func decodeAwarenessUpdate(r *ybinary.Reader, flags Flags) (*AwarenessUpdate, error) {
	routed, err := readRoutedPayload(
		r,
		"ReadAwarenessUpdate.documentKey",
		"ReadAwarenessUpdate.connectionID",
		"ReadAwarenessUpdate.epoch",
	)
	if err != nil {
		return nil, err
	}
	message := &AwarenessUpdate{
		Flags:        flags,
		DocumentKey:  routed.DocumentKey,
		ConnectionID: routed.ConnectionID,
		Epoch:        routed.Epoch,
		Payload:      routed.Body,
	}
	if err := message.Validate(); err != nil {
		return nil, wrapParseError("ReadAwarenessUpdate.validate", r.Offset(), err)
	}
	return message, nil
}

func decodePing(r *ybinary.Reader, flags Flags) (*Ping, error) {
	nonce, err := readUint64(r, "ReadPing.nonce")
	if err != nil {
		return nil, err
	}
	message := &Ping{
		Flags: flags,
		Nonce: nonce,
	}
	if err := message.Validate(); err != nil {
		return nil, wrapParseError("ReadPing.validate", r.Offset(), err)
	}
	return message, nil
}

func decodePong(r *ybinary.Reader, flags Flags) (*Pong, error) {
	nonce, err := readUint64(r, "ReadPong.nonce")
	if err != nil {
		return nil, err
	}
	message := &Pong{
		Flags: flags,
		Nonce: nonce,
	}
	if err := message.Validate(); err != nil {
		return nil, wrapParseError("ReadPong.validate", r.Offset(), err)
	}
	return message, nil
}

func validateNodeRoute(nodeID string, key storage.DocumentKey, connectionID string, epoch uint64) error {
	if strings.TrimSpace(nodeID) == "" {
		return ErrInvalidNodeID
	}
	return validateRoute(key, connectionID, epoch)
}

func validateRoute(key storage.DocumentKey, connectionID string, epoch uint64) error {
	if err := key.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(connectionID) == "" {
		return ErrInvalidConnectionID
	}
	if epoch == 0 {
		return ErrInvalidEpoch
	}
	return nil
}

func validateRoutedBody(key storage.DocumentKey, connectionID string, epoch uint64, body []byte) error {
	if err := validateRoute(key, connectionID, epoch); err != nil {
		return err
	}
	if len(body) == 0 {
		return ErrMissingPayload
	}
	return nil
}

func appendRoutedPayload(dst []byte, key storage.DocumentKey, connectionID string, epoch uint64, body []byte) ([]byte, error) {
	var err error

	dst, err = appendDocumentKey(dst, key)
	if err != nil {
		return nil, err
	}
	dst, err = appendString(dst, connectionID)
	if err != nil {
		return nil, err
	}
	dst = appendUint64(dst, epoch)
	return append(dst, body...), nil
}

func appendDocumentKey(dst []byte, key storage.DocumentKey) ([]byte, error) {
	var err error

	dst, err = appendString(dst, key.Namespace)
	if err != nil {
		return nil, err
	}
	return appendString(dst, key.DocumentID)
}

func appendString(dst []byte, value string) ([]byte, error) {
	if uint64(len(value)) > maxHeaderPayloadLength {
		return nil, fmt.Errorf("%w: %d", ErrPayloadTooLarge, len(value))
	}
	dst = varint.Append(dst, uint32(len(value)))
	return append(dst, value...), nil
}

func appendUint64(dst []byte, value uint64) []byte {
	start := len(dst)
	dst = append(dst, make([]byte, fixedUint64Size)...)
	binary.BigEndian.PutUint64(dst[start:start+fixedUint64Size], value)
	return dst
}

func readNodeID(r *ybinary.Reader, op string) (string, error) {
	start := r.Offset()
	nodeID, err := readString(r, op)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(nodeID) == "" {
		return "", wrapParseError(op, start, ErrInvalidNodeID)
	}
	return nodeID, nil
}

func readDocumentKey(r *ybinary.Reader, op string) (storage.DocumentKey, error) {
	start := r.Offset()
	namespace, err := readString(r, op+".namespace")
	if err != nil {
		return storage.DocumentKey{}, err
	}
	documentID, err := readString(r, op+".documentID")
	if err != nil {
		return storage.DocumentKey{}, err
	}

	key := storage.DocumentKey{
		Namespace:  namespace,
		DocumentID: documentID,
	}
	if err := key.Validate(); err != nil {
		return storage.DocumentKey{}, wrapParseError(op, start, err)
	}
	return key, nil
}

func readString(r *ybinary.Reader, op string) (string, error) {
	start := r.Offset()
	length, _, err := varint.Read(r)
	if err != nil {
		return "", wrapParseError(op+".len", start, err)
	}

	data, err := r.ReadN(int(length))
	if err != nil {
		return "", wrapParseError(op, r.Offset(), err)
	}
	return string(data), nil
}

func readUint64(r *ybinary.Reader, op string) (uint64, error) {
	start := r.Offset()
	raw, err := r.ReadN(fixedUint64Size)
	if err != nil {
		return 0, wrapParseError(op, start, err)
	}
	return binary.BigEndian.Uint64(raw), nil
}

func readRoutedPayload(r *ybinary.Reader, keyOp string, connectionOp string, epochOp string) (*routedPayload, error) {
	key, err := readDocumentKey(r, keyOp)
	if err != nil {
		return nil, err
	}
	connectionID, err := readString(r, connectionOp)
	if err != nil {
		return nil, err
	}
	epoch, err := readUint64(r, epochOp)
	if err != nil {
		return nil, err
	}
	body, err := readRemainingBytes(r)
	if err != nil {
		return nil, err
	}
	return &routedPayload{
		DocumentKey:  key,
		ConnectionID: connectionID,
		Epoch:        epoch,
		Body:         body,
	}, nil
}

func readRemainingBytes(r *ybinary.Reader) ([]byte, error) {
	if r.Remaining() == 0 {
		return nil, nil
	}
	raw, err := r.ReadN(r.Remaining())
	if err != nil {
		return nil, wrapParseError("readRemainingBytes", r.Offset(), err)
	}
	return append([]byte(nil), raw...), nil
}

func wrapParseError(op string, offset int, err error) error {
	if err == nil {
		return nil
	}
	return &ParseError{Op: op, Offset: offset, Err: err}
}
