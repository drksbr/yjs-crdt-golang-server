package ynodeproto

import (
	"bytes"
	"errors"
	"testing"
)

func TestMessageTypeValidAndString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		typ    MessageType
		want   string
		wantOK bool
	}{
		{name: "handshake", typ: MessageTypeHandshake, want: "handshake", wantOK: true},
		{name: "handshake_ack", typ: MessageTypeHandshakeAck, want: "handshake-ack", wantOK: true},
		{name: "document_sync_request", typ: MessageTypeDocumentSyncRequest, want: "document-sync-request", wantOK: true},
		{name: "document_sync_response", typ: MessageTypeDocumentSyncResponse, want: "document-sync-response", wantOK: true},
		{name: "document_update", typ: MessageTypeDocumentUpdate, want: "document-update", wantOK: true},
		{name: "awareness_update", typ: MessageTypeAwarenessUpdate, want: "awareness-update", wantOK: true},
		{name: "query_awareness_request", typ: MessageTypeQueryAwarenessRequest, want: "query-awareness-request", wantOK: true},
		{name: "query_awareness_response", typ: MessageTypeQueryAwarenessResponse, want: "query-awareness-response", wantOK: true},
		{name: "disconnect", typ: MessageTypeDisconnect, want: "disconnect", wantOK: true},
		{name: "close", typ: MessageTypeClose, want: "close", wantOK: true},
		{name: "ping", typ: MessageTypePing, want: "ping", wantOK: true},
		{name: "pong", typ: MessageTypePong, want: "pong", wantOK: true},
		{name: "unknown", typ: MessageType(255), want: "unknown(255)", wantOK: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.typ.Valid(); got != tt.wantOK {
				t.Fatalf("Valid() = %v, want %v", got, tt.wantOK)
			}
			if got := tt.typ.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodeDecodeFrameRoundTrip(t *testing.T) {
	t.Parallel()

	originalPayload := []byte{0xaa, 0xbb, 0xcc}
	frame, err := NewFrame(MessageTypeDocumentUpdate, Flags(0x1234), originalPayload)
	if err != nil {
		t.Fatalf("NewFrame() error = %v", err)
	}

	originalPayload[0] = 0x00
	if frame.Payload[0] != 0xaa {
		t.Fatalf("NewFrame() nao copiou payload de entrada")
	}

	encoded, err := EncodeFrame(frame)
	if err != nil {
		t.Fatalf("EncodeFrame() error = %v", err)
	}

	want := []byte{
		CurrentVersion,
		byte(MessageTypeDocumentUpdate),
		0x12, 0x34,
		0x00, 0x00, 0x00, 0x03,
		0xaa, 0xbb, 0xcc,
	}
	if !bytes.Equal(encoded, want) {
		t.Fatalf("EncodeFrame() = %v, want %v", encoded, want)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("DecodeFrame() error = %v", err)
	}

	if decoded.Header.Version != CurrentVersion {
		t.Fatalf("decoded.Header.Version = %d, want %d", decoded.Header.Version, CurrentVersion)
	}
	if decoded.Header.Type != MessageTypeDocumentUpdate {
		t.Fatalf("decoded.Header.Type = %v, want %v", decoded.Header.Type, MessageTypeDocumentUpdate)
	}
	if decoded.Header.Flags != Flags(0x1234) {
		t.Fatalf("decoded.Header.Flags = %#x, want %#x", decoded.Header.Flags, Flags(0x1234))
	}
	if decoded.Header.PayloadLength != uint32(len(originalPayload)) {
		t.Fatalf("decoded.Header.PayloadLength = %d, want %d", decoded.Header.PayloadLength, len(originalPayload))
	}
	if !bytes.Equal(decoded.Payload, want[HeaderSize:]) {
		t.Fatalf("decoded.Payload = %v, want %v", decoded.Payload, want[HeaderSize:])
	}

	encoded[HeaderSize] = 0x00
	if decoded.Payload[0] != 0xaa {
		t.Fatalf("DecodeFrame() nao desacoplou payload do buffer original")
	}
}

func TestDecodeFramePrefixReadsOnlyFirstFrame(t *testing.T) {
	t.Parallel()

	first, err := NewFrame(MessageTypePing, FlagNone, []byte{0x01})
	if err != nil {
		t.Fatalf("NewFrame(first) error = %v", err)
	}
	second, err := NewFrame(MessageTypePong, FlagNone, []byte{0x02, 0x03})
	if err != nil {
		t.Fatalf("NewFrame(second) error = %v", err)
	}

	encodedFirst, err := EncodeFrame(first)
	if err != nil {
		t.Fatalf("EncodeFrame(first) error = %v", err)
	}
	encodedSecond, err := EncodeFrame(second)
	if err != nil {
		t.Fatalf("EncodeFrame(second) error = %v", err)
	}

	stream := append(append([]byte(nil), encodedFirst...), encodedSecond...)
	decoded, consumed, err := DecodeFramePrefix(stream)
	if err != nil {
		t.Fatalf("DecodeFramePrefix() error = %v", err)
	}

	if consumed != len(encodedFirst) {
		t.Fatalf("DecodeFramePrefix() consumed = %d, want %d", consumed, len(encodedFirst))
	}
	if decoded.Header.Type != MessageTypePing {
		t.Fatalf("decoded.Header.Type = %v, want %v", decoded.Header.Type, MessageTypePing)
	}
	if !bytes.Equal(stream[consumed:], encodedSecond) {
		t.Fatalf("restante do stream = %v, want %v", stream[consumed:], encodedSecond)
	}
}

func TestHeaderAndFrameErrors(t *testing.T) {
	t.Parallel()

	validFrame, err := NewFrame(MessageTypePing, FlagNone, []byte{0xaa})
	if err != nil {
		t.Fatalf("NewFrame(validFrame) error = %v", err)
	}
	validEncoded, err := EncodeFrame(validFrame)
	if err != nil {
		t.Fatalf("EncodeFrame(validFrame) error = %v", err)
	}

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{
			name:    "new_header_negative_payload_length",
			run:     func() error { _, err := NewHeader(MessageTypePing, FlagNone, -1); return err },
			wantErr: ErrInvalidPayloadLength,
		},
		{
			name:    "encode_nil_frame",
			run:     func() error { _, err := EncodeFrame(nil); return err },
			wantErr: ErrNilFrame,
		},
		{
			name: "encode_payload_length_mismatch",
			run: func() error {
				_, err := EncodeFrame(&Frame{
					Header: Header{
						Version:       CurrentVersion,
						Type:          MessageTypePing,
						Flags:         FlagNone,
						PayloadLength: 2,
					},
					Payload: []byte{0xaa},
				})
				return err
			},
			wantErr: ErrPayloadLengthMismatch,
		},
		{
			name: "decode_incomplete_header",
			run: func() error {
				_, err := DecodeHeader([]byte{CurrentVersion, byte(MessageTypePing)})
				return err
			},
			wantErr: ErrIncompleteHeader,
		},
		{
			name: "decode_unsupported_version",
			run: func() error {
				buf := append([]byte(nil), validEncoded...)
				buf[0] = CurrentVersion + 1
				_, err := DecodeFrame(buf)
				return err
			},
			wantErr: ErrUnsupportedVersion,
		},
		{
			name: "decode_unknown_message_type",
			run: func() error {
				buf := append([]byte(nil), validEncoded...)
				buf[1] = 0xff
				_, err := DecodeFrame(buf)
				return err
			},
			wantErr: ErrUnknownMessageType,
		},
		{
			name: "decode_incomplete_payload",
			run: func() error {
				buf := append([]byte(nil), validEncoded[:len(validEncoded)-1]...)
				_, err := DecodeFrame(buf)
				return err
			},
			wantErr: ErrIncompletePayload,
		},
		{
			name: "decode_trailing_bytes",
			run: func() error {
				buf := append(append([]byte(nil), validEncoded...), 0xff)
				_, err := DecodeFrame(buf)
				return err
			},
			wantErr: ErrTrailingBytes,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
