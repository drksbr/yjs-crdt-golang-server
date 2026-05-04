package yprotocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	ybinary "github.com/drksbr/yjs-crdt-golang-server/internal/binary"
	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
)

func TestReadProtocolMessageStreamingMixedProtocols(t *testing.T) {
	t.Parallel()

	first := EncodeProtocolSyncStep1([]byte{0x00})
	second := EncodeProtocolQueryAwareness()
	third := EncodeProtocolAuthPermissionDenied("forbidden")
	fourth := buildAwarenessProtocolMessage()

	stream := append(first, second...)
	stream = append(stream, third...)
	stream = append(stream, fourth...)
	reader := ybinary.NewReader(stream)

	syncMessage, err := ReadProtocolMessage(reader)
	if err != nil {
		t.Fatalf("ReadProtocolMessage() first unexpected error: %v", err)
	}
	if syncMessage.Protocol != ProtocolTypeSync || syncMessage.Sync == nil || syncMessage.Sync.Type != SyncMessageTypeStep1 || len(syncMessage.Sync.Payload) != 1 || syncMessage.Sync.Payload[0] != 0x00 {
		t.Fatalf("syncMessage = %#v, want protocolo sync step1 com payload [0]", syncMessage)
	}
	if syncMessage.Awareness != nil {
		t.Fatal("syncMessage.Awareness esperado como nil")
	}

	queryMessage, err := ReadProtocolMessage(reader)
	if err != nil {
		t.Fatalf("ReadProtocolMessage() second unexpected error: %v", err)
	}
	if queryMessage.Protocol != ProtocolTypeQueryAwareness || queryMessage.QueryAwareness == nil {
		t.Fatalf("queryMessage = %#v, want protocolo query-awareness", queryMessage)
	}

	authMessage, err := ReadProtocolMessage(reader)
	if err != nil {
		t.Fatalf("ReadProtocolMessage() third unexpected error: %v", err)
	}
	if authMessage.Protocol != ProtocolTypeAuth || authMessage.Auth == nil || authMessage.Auth.Type != AuthMessageTypePermissionDenied || authMessage.Auth.Reason != "forbidden" {
		t.Fatalf("authMessage = %#v, want protocolo auth permission denied", authMessage)
	}

	awarenessMessage, err := ReadProtocolMessage(reader)
	if err != nil {
		t.Fatalf("ReadProtocolMessage() fourth unexpected error: %v", err)
	}
	if awarenessMessage.Protocol != ProtocolTypeAwareness || awarenessMessage.Awareness == nil || len(awarenessMessage.Awareness.Clients) != 1 {
		t.Fatalf("awarenessMessage = %#v, want protocolo awareness com 1 client", awarenessMessage)
	}
	if awarenessMessage.Awareness.Clients[0].ClientID != 7 || awarenessMessage.Awareness.Clients[0].Clock != 3 {
		t.Fatalf("client = %+v, want clientID=7 clock=3", awarenessMessage.Awareness.Clients[0])
	}
	if string(awarenessMessage.Awareness.Clients[0].State) != `{"name":"ramon"}` {
		t.Fatalf("state = %s, want '{\"name\":\"ramon\"}'", awarenessMessage.Awareness.Clients[0].State)
	}

	if reader.Remaining() != 0 {
		t.Fatalf("Remaining() = %d, want 0", reader.Remaining())
	}
}

func buildAwarenessProtocolMessage() []byte {
	payload := varint.Append(nil, 1)
	payload = varint.Append(payload, 7)
	payload = varint.Append(payload, 3)

	state := json.RawMessage(`{"name":"ramon"}`)
	payload = varint.Append(payload, uint32(len(state)))
	payload = append(payload, state...)

	enveloped, err := AppendProtocolMessagePayload(nil, ProtocolTypeAwareness, payload)
	if err != nil {
		panic(err)
	}
	return enveloped
}

func TestEncodeProtocolMessageWrapsAwarenessPayload(t *testing.T) {
	t.Parallel()

	payload := varint.Append(nil, 1)
	payload = varint.Append(payload, 7)
	payload = varint.Append(payload, 3)
	payload = varint.Append(payload, uint32(len(`{"name":"ramon"}`)))
	payload = append(payload, []byte(`{"name":"ramon"}`)...)

	got, err := EncodeProtocolMessage(ProtocolTypeAwareness, payload)
	if err != nil {
		t.Fatalf("EncodeProtocolMessage() unexpected error: %v", err)
	}

	want := AppendProtocolType(nil, ProtocolTypeAwareness)
	want = varint.Append(want, uint32(len(payload)))
	want = append(want, payload...)
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeProtocolMessage() = %v, want y-websocket varUint8Array framing %v", got, want)
	}
}

func TestDecodeProtocolMessageRejectsUnknownProtocol(t *testing.T) {
	t.Parallel()

	src := AppendProtocolType(nil, ProtocolType(127))
	src = append(src, 0x00)

	_, err := DecodeProtocolMessage(src)
	if !errors.Is(err, ErrUnknownProtocolType) {
		t.Fatalf("DecodeProtocolMessage() error = %v, want ErrUnknownProtocolType", err)
	}
}
