package yprotocol

import (
	"errors"
	"testing"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/internal/varint"
)

func TestEncodeDecodeAuthMessageRoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := EncodeAuthMessage(AuthMessageTypePermissionDenied, "forbidden")
	if err != nil {
		t.Fatalf("EncodeAuthMessage() unexpected error: %v", err)
	}

	decoded, err := DecodeAuthMessage(encoded)
	if err != nil {
		t.Fatalf("DecodeAuthMessage() unexpected error: %v", err)
	}

	if decoded.Type != AuthMessageTypePermissionDenied {
		t.Fatalf("decoded.Type = %v, want %v", decoded.Type, AuthMessageTypePermissionDenied)
	}
	if decoded.Reason != "forbidden" {
		t.Fatalf("decoded.Reason = %q, want %q", decoded.Reason, "forbidden")
	}
}

func TestReadProtocolAuthMessageStreaming(t *testing.T) {
	t.Parallel()

	stream := append(
		EncodeProtocolAuthPermissionDenied("missing token"),
		EncodeProtocolQueryAwareness()...,
	)

	reader := ybinary.NewReader(stream)

	authMessage, err := ReadProtocolAuthMessage(reader)
	if err != nil {
		t.Fatalf("ReadProtocolAuthMessage() unexpected error: %v", err)
	}
	if authMessage.Type != AuthMessageTypePermissionDenied || authMessage.Reason != "missing token" {
		t.Fatalf("authMessage = %#v, want permission denied missing token", authMessage)
	}

	queryMessage, err := ReadProtocolQueryAwareness(reader)
	if err != nil {
		t.Fatalf("ReadProtocolQueryAwareness() unexpected error: %v", err)
	}
	if queryMessage == nil {
		t.Fatal("queryMessage = nil, want non-nil")
	}

	if reader.Remaining() != 0 {
		t.Fatalf("Remaining() = %d, want 0", reader.Remaining())
	}
}

func TestDecodeAuthMessageRejectsUnknownType(t *testing.T) {
	t.Parallel()

	src := varint.Append(nil, 9)
	src = varint.Append(src, 0)

	_, err := DecodeAuthMessage(src)
	if !errors.Is(err, ErrUnknownAuthMessageType) {
		t.Fatalf("DecodeAuthMessage() error = %v, want ErrUnknownAuthMessageType", err)
	}
}

func TestDecodeProtocolAuthMessageRejectsWrongProtocol(t *testing.T) {
	t.Parallel()

	src := AppendProtocolType(nil, ProtocolTypeSync)
	src = append(src, EncodeAuthPermissionDenied("wrong")...)

	_, err := DecodeProtocolAuthMessage(src)
	if !errors.Is(err, ErrUnexpectedProtocolType) {
		t.Fatalf("DecodeProtocolAuthMessage() error = %v, want ErrUnexpectedProtocolType", err)
	}
}

func TestDecodeAuthMessageRejectsTruncatedReason(t *testing.T) {
	t.Parallel()

	src := varint.Append(nil, uint32(AuthMessageTypePermissionDenied))
	src = varint.Append(src, 4)
	src = append(src, 'b', 'a', 'd')

	_, err := DecodeAuthMessage(src)
	if !errors.Is(err, ybinary.ErrUnexpectedEOF) {
		t.Fatalf("DecodeAuthMessage() error = %v, want binary.ErrUnexpectedEOF", err)
	}
}

func TestDecodeProtocolQueryAwarenessRejectsTrailingBytes(t *testing.T) {
	t.Parallel()

	src := append(EncodeProtocolQueryAwareness(), 0xff)

	_, err := DecodeProtocolQueryAwareness(src)
	if !errors.Is(err, ErrTrailingBytes) {
		t.Fatalf("DecodeProtocolQueryAwareness() error = %v, want ErrTrailingBytes", err)
	}
}
