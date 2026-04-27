package yprotocol

import (
	"bytes"
	"errors"
	"testing"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
	"yjs-go-bridge/internal/yupdate"
)

func TestEncodeDecodeSyncMessageRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typ     SyncMessageType
		payload []byte
	}{
		{name: "step1", typ: SyncMessageTypeStep1, payload: []byte{0x01, 0x02}},
		{name: "step2", typ: SyncMessageTypeStep2, payload: []byte{0xaa}},
		{name: "update", typ: SyncMessageTypeUpdate, payload: []byte{0xde, 0xad, 0xbe, 0xef}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := EncodeSyncMessage(tt.typ, tt.payload)
			if err != nil {
				t.Fatalf("EncodeSyncMessage() unexpected error: %v", err)
			}

			decoded, err := DecodeSyncMessage(encoded)
			if err != nil {
				t.Fatalf("DecodeSyncMessage() unexpected error: %v", err)
			}

			if decoded.Type != tt.typ {
				t.Fatalf("decoded.Type = %v, want %v", decoded.Type, tt.typ)
			}
			if !bytes.Equal(decoded.Payload, tt.payload) {
				t.Fatalf("decoded.Payload = %v, want %v", decoded.Payload, tt.payload)
			}
		})
	}
}

func TestReadProtocolSyncMessageStreaming(t *testing.T) {
	t.Parallel()

	stream := append(
		EncodeProtocolSyncStep1([]byte{0x00}),
		EncodeProtocolSyncUpdate([]byte{0x01, 0x02})...,
	)

	reader := ybinary.NewReader(stream)

	first, err := ReadProtocolSyncMessage(reader)
	if err != nil {
		t.Fatalf("ReadProtocolSyncMessage() first unexpected error: %v", err)
	}
	if first.Type != SyncMessageTypeStep1 || !bytes.Equal(first.Payload, []byte{0x00}) {
		t.Fatalf("first = %#v, want SyncStep1 payload [0]", first)
	}

	second, err := ReadProtocolSyncMessage(reader)
	if err != nil {
		t.Fatalf("ReadProtocolSyncMessage() second unexpected error: %v", err)
	}
	if second.Type != SyncMessageTypeUpdate || !bytes.Equal(second.Payload, []byte{0x01, 0x02}) {
		t.Fatalf("second = %#v, want SyncUpdate payload [1 2]", second)
	}

	if reader.Remaining() != 0 {
		t.Fatalf("Remaining() = %d, want 0", reader.Remaining())
	}
}

func TestEncodeProtocolSyncStep1FromUpdateV1(t *testing.T) {
	t.Parallel()

	update := buildGCOnlyUpdate(3, 2)

	expectedStateVector, err := yupdate.EncodeStateVectorFromUpdateV1(update)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdateV1() unexpected error: %v", err)
	}

	message, err := EncodeProtocolSyncStep1FromUpdateV1(update)
	if err != nil {
		t.Fatalf("EncodeProtocolSyncStep1FromUpdateV1() unexpected error: %v", err)
	}

	decoded, err := DecodeProtocolSyncMessage(message)
	if err != nil {
		t.Fatalf("DecodeProtocolSyncMessage() unexpected error: %v", err)
	}

	if decoded.Type != SyncMessageTypeStep1 {
		t.Fatalf("decoded.Type = %v, want %v", decoded.Type, SyncMessageTypeStep1)
	}
	if !bytes.Equal(decoded.Payload, expectedStateVector) {
		t.Fatalf("decoded.Payload = %v, want %v", decoded.Payload, expectedStateVector)
	}
}

func TestEncodeProtocolSyncStep1FromUpdatesV1(t *testing.T) {
	t.Parallel()

	left := buildGCOnlyUpdate(3, 2)
	right := buildGCOnlyUpdate(7, 1)

	expectedStateVector, err := yupdate.EncodeStateVectorFromUpdates(left, right)
	if err != nil {
		t.Fatalf("EncodeStateVectorFromUpdates() unexpected error: %v", err)
	}

	message, err := EncodeProtocolSyncStep1FromUpdatesV1(left, right)
	if err != nil {
		t.Fatalf("EncodeProtocolSyncStep1FromUpdatesV1() unexpected error: %v", err)
	}

	decoded, err := DecodeProtocolSyncMessage(message)
	if err != nil {
		t.Fatalf("DecodeProtocolSyncMessage() unexpected error: %v", err)
	}
	if decoded.Type != SyncMessageTypeStep1 {
		t.Fatalf("decoded.Type = %v, want %v", decoded.Type, SyncMessageTypeStep1)
	}
	if !bytes.Equal(decoded.Payload, expectedStateVector) {
		t.Fatalf("decoded.Payload = %v, want %v", decoded.Payload, expectedStateVector)
	}
}

func TestEncodeProtocolSyncStep2FromUpdatesV1(t *testing.T) {
	t.Parallel()

	left := buildGCOnlyUpdate(4, 1)
	right := buildGCOnlyUpdate(5, 2)

	expectedMerged, err := yupdate.MergeUpdatesV1(left, right)
	if err != nil {
		t.Fatalf("MergeUpdatesV1() unexpected error: %v", err)
	}

	message, err := EncodeProtocolSyncStep2FromUpdatesV1(left, right)
	if err != nil {
		t.Fatalf("EncodeProtocolSyncStep2FromUpdatesV1() unexpected error: %v", err)
	}

	decoded, err := DecodeProtocolSyncMessage(message)
	if err != nil {
		t.Fatalf("DecodeProtocolSyncMessage() unexpected error: %v", err)
	}
	if decoded.Type != SyncMessageTypeStep2 {
		t.Fatalf("decoded.Type = %v, want %v", decoded.Type, SyncMessageTypeStep2)
	}
	if !bytes.Equal(decoded.Payload, expectedMerged) {
		t.Fatalf("decoded.Payload = %v, want %v", decoded.Payload, expectedMerged)
	}
}

func TestDecodeSyncMessageRejectsUnknownType(t *testing.T) {
	t.Parallel()

	src := varint.Append(nil, 9)
	src = varint.Append(src, 0)

	_, err := DecodeSyncMessage(src)
	if !errors.Is(err, ErrUnknownSyncMessageType) {
		t.Fatalf("DecodeSyncMessage() error = %v, want ErrUnknownSyncMessageType", err)
	}
}

func TestDecodeProtocolSyncMessageRejectsWrongProtocol(t *testing.T) {
	t.Parallel()

	src := AppendProtocolType(nil, ProtocolTypeAwareness)
	src = append(src, EncodeSyncUpdate([]byte{0x01})...)

	_, err := DecodeProtocolSyncMessage(src)
	if !errors.Is(err, ErrUnexpectedProtocolType) {
		t.Fatalf("DecodeProtocolSyncMessage() error = %v, want ErrUnexpectedProtocolType", err)
	}
}

func TestDecodeSyncMessageRejectsTruncatedPayload(t *testing.T) {
	t.Parallel()

	src := varint.Append(nil, uint32(SyncMessageTypeStep2))
	src = varint.Append(src, 2)
	src = append(src, 0xaa)

	_, err := DecodeSyncMessage(src)
	if !errors.Is(err, ybinary.ErrUnexpectedEOF) {
		t.Fatalf("DecodeSyncMessage() error = %v, want binary.ErrUnexpectedEOF", err)
	}
}

func TestDecodeSyncMessageRejectsTrailingBytes(t *testing.T) {
	t.Parallel()

	src := append(EncodeSyncUpdate([]byte{0x01}), 0xff)

	_, err := DecodeSyncMessage(src)
	if !errors.Is(err, ErrTrailingBytes) {
		t.Fatalf("DecodeSyncMessage() error = %v, want ErrTrailingBytes", err)
	}
}

func buildGCOnlyUpdate(client, length uint32) []byte {
	update := varint.Append(nil, 1)
	update = varint.Append(update, 1)
	update = varint.Append(update, client)
	update = varint.Append(update, 0)
	update = append(update, 0)
	update = varint.Append(update, length)
	return append(update, yupdate.EncodeDeleteSetBlockV1(ytypes.NewDeleteSet())...)
}
