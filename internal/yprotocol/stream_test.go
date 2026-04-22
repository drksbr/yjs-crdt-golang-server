package yprotocol

import (
	"errors"
	"testing"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/internal/varint"
)

func TestReadProtocolMessagesReadsMixedStream(t *testing.T) {
	t.Parallel()

	stream := append(
		EncodeProtocolSyncStep1([]byte{0x00}),
		buildAwarenessProtocolMessage()...,
	)
	stream = append(stream, EncodeProtocolSyncUpdate([]byte{0x01, 0x02})...)

	reader := ybinary.NewReader(stream)
	messages, err := ReadProtocolMessages(reader)
	if err != nil {
		t.Fatalf("ReadProtocolMessages() unexpected error: %v", err)
	}

	if len(messages) != 3 {
		t.Fatalf("ReadProtocolMessages() len = %d, want 3", len(messages))
	}

	if messages[0].Protocol != ProtocolTypeSync || messages[0].Sync == nil || messages[0].Sync.Type != SyncMessageTypeStep1 {
		t.Fatalf("messages[0] = %#v, want protocolo sync step1", messages[0])
	}
	if messages[1].Protocol != ProtocolTypeAwareness || messages[1].Awareness == nil || len(messages[1].Awareness.Clients) != 1 {
		t.Fatalf("messages[1] = %#v, want protocolo awareness com 1 client", messages[1])
	}
	if messages[2].Protocol != ProtocolTypeSync || messages[2].Sync == nil || messages[2].Sync.Type != SyncMessageTypeUpdate {
		t.Fatalf("messages[2] = %#v, want protocolo sync update", messages[2])
	}

	if reader.Remaining() != 0 {
		t.Fatalf("reader.Remaining() = %d, want 0", reader.Remaining())
	}
}

func TestDecodeProtocolMessagesRejectsTruncatedMessageAtEnd(t *testing.T) {
	t.Parallel()

	stream := append(EncodeProtocolSyncStep1([]byte{0x00}), 0x80)

	_, err := DecodeProtocolMessages(stream)
	if !errors.Is(err, varint.ErrUnexpectedEOF) {
		t.Fatalf("DecodeProtocolMessages() error = %v, want varint.ErrUnexpectedEOF", err)
	}
}

func TestReadProtocolMessagesNRespectsLimit(t *testing.T) {
	t.Parallel()

	stream := append(
		EncodeProtocolSyncStep1([]byte{0x00}),
		EncodeProtocolSyncUpdate([]byte{0x01})...,
	)
	stream = append(stream, EncodeProtocolSyncStep2([]byte{0x02, 0x03})...)

	reader := ybinary.NewReader(stream)
	messages, err := ReadProtocolMessagesN(reader, 2)
	if err != nil {
		t.Fatalf("ReadProtocolMessagesN() unexpected error: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("ReadProtocolMessagesN() len = %d, want 2", len(messages))
	}
	if messages[0].Protocol != ProtocolTypeSync || messages[1].Protocol != ProtocolTypeSync {
		t.Fatalf("mensagens inesperadas: %#v", messages)
	}
	if reader.Remaining() <= 0 {
		t.Fatalf("reader.Remaining() = %d, want > 0", reader.Remaining())
	}
}
