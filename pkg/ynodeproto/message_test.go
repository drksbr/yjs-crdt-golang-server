package ynodeproto

import (
	"bytes"
	"errors"
	"testing"

	ybinary "yjs-go-bridge/internal/binary"
	"yjs-go-bridge/pkg/storage"
)

func TestTypedMessageRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message Message
		assert  func(t *testing.T, decoded Message)
	}{
		{
			name: "handshake",
			message: &Handshake{
				Flags:        Flags(0x11),
				NodeID:       "node-a",
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"},
				ConnectionID: "conn-1",
				ClientID:     101,
				Epoch:        7,
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*Handshake)
				if !ok {
					t.Fatalf("decoded = %T, want *Handshake", decoded)
				}
				if got.NodeID != "node-a" {
					t.Fatalf("got.NodeID = %q, want %q", got.NodeID, "node-a")
				}
				if got.DocumentKey != (storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"}) {
					t.Fatalf("got.DocumentKey = %#v", got.DocumentKey)
				}
				if got.ConnectionID != "conn-1" || got.ClientID != 101 || got.Epoch != 7 {
					t.Fatalf("got = %#v, want conn-1/client-101/7", got)
				}
			},
		},
		{
			name: "handshake_ack",
			message: &HandshakeAck{
				Flags:        Flags(0x12),
				NodeID:       "node-b",
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-2"},
				ConnectionID: "conn-2",
				ClientID:     202,
				Epoch:        8,
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*HandshakeAck)
				if !ok {
					t.Fatalf("decoded = %T, want *HandshakeAck", decoded)
				}
				if got.NodeID != "node-b" {
					t.Fatalf("got.NodeID = %q, want %q", got.NodeID, "node-b")
				}
				if got.DocumentKey != (storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-2"}) {
					t.Fatalf("got.DocumentKey = %#v", got.DocumentKey)
				}
				if got.ConnectionID != "conn-2" || got.ClientID != 202 || got.Epoch != 8 {
					t.Fatalf("got = %#v, want conn-2/client-202/8", got)
				}
			},
		},
		{
			name: "document_sync_request",
			message: &DocumentSyncRequest{
				Flags:        Flags(0x21),
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-3"},
				ConnectionID: "conn-3",
				Epoch:        9,
				StateVector:  []byte{0x01, 0x02},
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*DocumentSyncRequest)
				if !ok {
					t.Fatalf("decoded = %T, want *DocumentSyncRequest", decoded)
				}
				if got.DocumentKey.DocumentID != "doc-3" || got.ConnectionID != "conn-3" || got.Epoch != 9 {
					t.Fatalf("got = %#v, want document/doc-3 conn-3 epoch 9", got)
				}
				if !bytes.Equal(got.StateVector, []byte{0x01, 0x02}) {
					t.Fatalf("got.StateVector = %v, want %v", got.StateVector, []byte{0x01, 0x02})
				}
			},
		},
		{
			name: "document_sync_response",
			message: &DocumentSyncResponse{
				Flags:        Flags(0x22),
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-4"},
				ConnectionID: "conn-4",
				Epoch:        10,
				UpdateV1:     []byte{0x0a, 0x0b},
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*DocumentSyncResponse)
				if !ok {
					t.Fatalf("decoded = %T, want *DocumentSyncResponse", decoded)
				}
				if got.DocumentKey.DocumentID != "doc-4" || got.ConnectionID != "conn-4" || got.Epoch != 10 {
					t.Fatalf("got = %#v, want document/doc-4 conn-4 epoch 10", got)
				}
				if !bytes.Equal(got.UpdateV1, []byte{0x0a, 0x0b}) {
					t.Fatalf("got.UpdateV1 = %v, want %v", got.UpdateV1, []byte{0x0a, 0x0b})
				}
			},
		},
		{
			name: "document_update",
			message: &DocumentUpdate{
				Flags:        Flags(0x23),
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-5"},
				ConnectionID: "conn-5",
				Epoch:        11,
				UpdateV1:     []byte{0xaa},
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*DocumentUpdate)
				if !ok {
					t.Fatalf("decoded = %T, want *DocumentUpdate", decoded)
				}
				if got.DocumentKey.DocumentID != "doc-5" || got.ConnectionID != "conn-5" || got.Epoch != 11 {
					t.Fatalf("got = %#v, want document/doc-5 conn-5 epoch 11", got)
				}
				if !bytes.Equal(got.UpdateV1, []byte{0xaa}) {
					t.Fatalf("got.UpdateV1 = %v, want %v", got.UpdateV1, []byte{0xaa})
				}
			},
		},
		{
			name: "awareness_update",
			message: &AwarenessUpdate{
				Flags:        Flags(0x24),
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-6"},
				ConnectionID: "conn-6",
				Epoch:        12,
				Payload:      []byte(`{"cursor":1}`),
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*AwarenessUpdate)
				if !ok {
					t.Fatalf("decoded = %T, want *AwarenessUpdate", decoded)
				}
				if got.DocumentKey.DocumentID != "doc-6" || got.ConnectionID != "conn-6" || got.Epoch != 12 {
					t.Fatalf("got = %#v, want document/doc-6 conn-6 epoch 12", got)
				}
				if !bytes.Equal(got.Payload, []byte(`{"cursor":1}`)) {
					t.Fatalf("got.Payload = %s, want %s", got.Payload, `{"cursor":1}`)
				}
			},
		},
		{
			name: "query_awareness_request",
			message: &QueryAwarenessRequest{
				Flags:        Flags(0x25),
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-7"},
				ConnectionID: "conn-7",
				Epoch:        13,
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*QueryAwarenessRequest)
				if !ok {
					t.Fatalf("decoded = %T, want *QueryAwarenessRequest", decoded)
				}
				if got.DocumentKey.DocumentID != "doc-7" || got.ConnectionID != "conn-7" || got.Epoch != 13 {
					t.Fatalf("got = %#v, want document/doc-7 conn-7 epoch 13", got)
				}
			},
		},
		{
			name: "query_awareness_response",
			message: &QueryAwarenessResponse{
				Flags:        Flags(0x26),
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-8"},
				ConnectionID: "conn-8",
				Epoch:        14,
				Payload:      []byte{0x81, 0x00},
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*QueryAwarenessResponse)
				if !ok {
					t.Fatalf("decoded = %T, want *QueryAwarenessResponse", decoded)
				}
				if got.DocumentKey.DocumentID != "doc-8" || got.ConnectionID != "conn-8" || got.Epoch != 14 {
					t.Fatalf("got = %#v, want document/doc-8 conn-8 epoch 14", got)
				}
				if !bytes.Equal(got.Payload, []byte{0x81, 0x00}) {
					t.Fatalf("got.Payload = %v, want %v", got.Payload, []byte{0x81, 0x00})
				}
			},
		},
		{
			name: "disconnect",
			message: &Disconnect{
				Flags:        Flags(0x27),
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-9"},
				ConnectionID: "conn-9",
				Epoch:        15,
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*Disconnect)
				if !ok {
					t.Fatalf("decoded = %T, want *Disconnect", decoded)
				}
				if got.DocumentKey.DocumentID != "doc-9" || got.ConnectionID != "conn-9" || got.Epoch != 15 {
					t.Fatalf("got = %#v, want document/doc-9 conn-9 epoch 15", got)
				}
			},
		},
		{
			name: "close",
			message: &Close{
				Flags:        Flags(0x28),
				DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-10"},
				ConnectionID: "conn-10",
				Epoch:        16,
			},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*Close)
				if !ok {
					t.Fatalf("decoded = %T, want *Close", decoded)
				}
				if got.DocumentKey.DocumentID != "doc-10" || got.ConnectionID != "conn-10" || got.Epoch != 16 {
					t.Fatalf("got = %#v, want document/doc-10 conn-10 epoch 16", got)
				}
			},
		},
		{
			name:    "ping",
			message: &Ping{Flags: Flags(0x31), Nonce: 17},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*Ping)
				if !ok {
					t.Fatalf("decoded = %T, want *Ping", decoded)
				}
				if got.Nonce != 17 {
					t.Fatalf("got.Nonce = %d, want %d", got.Nonce, 17)
				}
			},
		},
		{
			name:    "pong",
			message: &Pong{Flags: Flags(0x32), Nonce: 18},
			assert: func(t *testing.T, decoded Message) {
				t.Helper()

				got, ok := decoded.(*Pong)
				if !ok {
					t.Fatalf("decoded = %T, want *Pong", decoded)
				}
				if got.Nonce != 18 {
					t.Fatalf("got.Nonce = %d, want %d", got.Nonce, 18)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := EncodeMessagePayload(tt.message)
			if err != nil {
				t.Fatalf("EncodeMessagePayload() unexpected error: %v", err)
			}

			decoded, err := DecodeMessagePayload(tt.message.Type(), FlagNone, payload)
			if err != nil {
				t.Fatalf("DecodeMessagePayload() unexpected error: %v", err)
			}
			tt.assert(t, decoded)

			frame, err := NewMessageFrame(tt.message)
			if err != nil {
				t.Fatalf("NewMessageFrame() unexpected error: %v", err)
			}
			decodedFrameMessage, err := DecodeFrameMessage(frame)
			if err != nil {
				t.Fatalf("DecodeFrameMessage() unexpected error: %v", err)
			}
			tt.assert(t, decodedFrameMessage)
			assertDecodedFlags(t, decodedFrameMessage, tt.message.FrameFlags())
		})
	}
}

func TestTypedMessageValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message Message
		wantErr error
	}{
		{name: "nil_message", message: nil, wantErr: ErrNilMessage},
		{name: "invalid_handshake", message: &Handshake{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, ConnectionID: "conn", Epoch: 1}, wantErr: ErrInvalidNodeID},
		{name: "invalid_handshake_ack", message: &HandshakeAck{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, ConnectionID: "conn", Epoch: 1}, wantErr: ErrInvalidNodeID},
		{name: "invalid_sync_request_key", message: &DocumentSyncRequest{ConnectionID: "conn", Epoch: 1, StateVector: []byte{0x01}}, wantErr: storage.ErrInvalidDocumentKey},
		{name: "invalid_sync_request_connection", message: &DocumentSyncRequest{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, Epoch: 1, StateVector: []byte{0x01}}, wantErr: ErrInvalidConnectionID},
		{name: "invalid_sync_request_epoch", message: &DocumentSyncRequest{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, ConnectionID: "conn", StateVector: []byte{0x01}}, wantErr: ErrInvalidEpoch},
		{name: "invalid_sync_response_payload", message: &DocumentSyncResponse{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, ConnectionID: "conn", Epoch: 1}, wantErr: ErrMissingPayload},
		{name: "invalid_document_update_payload", message: &DocumentUpdate{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, ConnectionID: "conn", Epoch: 1}, wantErr: ErrMissingPayload},
		{name: "invalid_awareness_payload", message: &AwarenessUpdate{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, ConnectionID: "conn", Epoch: 1}, wantErr: ErrMissingPayload},
		{name: "invalid_query_awareness_request", message: &QueryAwarenessRequest{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, Epoch: 1}, wantErr: ErrInvalidConnectionID},
		{name: "invalid_query_awareness_response_payload", message: &QueryAwarenessResponse{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, ConnectionID: "conn", Epoch: 1}, wantErr: ErrMissingPayload},
		{name: "invalid_disconnect_connection", message: &Disconnect{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, Epoch: 1}, wantErr: ErrInvalidConnectionID},
		{name: "invalid_close_epoch", message: &Close{DocumentKey: storage.DocumentKey{DocumentID: "doc-1"}, ConnectionID: "conn"}, wantErr: ErrInvalidEpoch},
		{name: "invalid_ping_nonce", message: &Ping{}, wantErr: ErrInvalidNonce},
		{name: "invalid_pong_nonce", message: &Pong{}, wantErr: ErrInvalidNonce},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := EncodeMessagePayload(tt.message)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("EncodeMessagePayload() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeTypedMessageErrors(t *testing.T) {
	t.Parallel()

	encodedHandshake, err := EncodeMessagePayload(&Handshake{
		NodeID:       "node-a",
		DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"},
		ConnectionID: "conn-1",
		ClientID:     77,
		Epoch:        7,
	})
	if err != nil {
		t.Fatalf("EncodeMessagePayload(handshake) unexpected error: %v", err)
	}

	encodedPing, err := EncodeMessagePayload(&Ping{Nonce: 99})
	if err != nil {
		t.Fatalf("EncodeMessagePayload(ping) unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		run     func() error
		wantErr error
	}{
		{
			name: "unknown_message_type",
			run: func() error {
				_, err := DecodeMessagePayload(MessageType(255), FlagNone, []byte{0x01})
				return err
			},
			wantErr: ErrUnknownMessageType,
		},
		{
			name: "trailing_payload_bytes",
			run: func() error {
				_, err := DecodeMessagePayload(MessageTypeHandshake, FlagNone, append(append([]byte(nil), encodedHandshake...), 0xff))
				return err
			},
			wantErr: ErrTrailingPayloadBytes,
		},
		{
			name: "incomplete_nonce_payload",
			run: func() error {
				_, err := DecodeMessagePayload(MessageTypePing, FlagNone, encodedPing[:7])
				return err
			},
			wantErr: ybinary.ErrUnexpectedEOF,
		},
		{
			name: "invalid_handshake_payload",
			run: func() error {
				_, err := DecodeMessagePayload(MessageTypeHandshake, FlagNone, []byte{0x00})
				return err
			},
			wantErr: ErrInvalidNodeID,
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

func TestDecodeFrameMessageCopiesPayload(t *testing.T) {
	t.Parallel()

	message := &DocumentUpdate{
		Flags:        Flags(0x44),
		DocumentKey:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-7"},
		ConnectionID: "conn-7",
		Epoch:        15,
		UpdateV1:     []byte{0x10, 0x11, 0x12},
	}
	encoded, err := EncodeMessageFrame(message)
	if err != nil {
		t.Fatalf("EncodeMessageFrame() unexpected error: %v", err)
	}

	frame, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("DecodeFrame() unexpected error: %v", err)
	}

	decoded, err := DecodeFrameMessage(frame)
	if err != nil {
		t.Fatalf("DecodeFrameMessage() unexpected error: %v", err)
	}
	got, ok := decoded.(*DocumentUpdate)
	if !ok {
		t.Fatalf("decoded = %T, want *DocumentUpdate", decoded)
	}

	frame.Payload[len(frame.Payload)-1] = 0xff
	if !bytes.Equal(got.UpdateV1, []byte{0x10, 0x11, 0x12}) {
		t.Fatalf("got.UpdateV1 = %v, want original payload", got.UpdateV1)
	}
}

func TestDecodeHandshakePayloadWithoutClientID(t *testing.T) {
	t.Parallel()

	var payload []byte
	var err error

	payload, err = appendString(payload, "node-a")
	if err != nil {
		t.Fatalf("appendString(nodeID) unexpected error: %v", err)
	}
	payload, err = appendString(payload, "team-a")
	if err != nil {
		t.Fatalf("appendString(namespace) unexpected error: %v", err)
	}
	payload, err = appendString(payload, "doc-legacy")
	if err != nil {
		t.Fatalf("appendString(documentID) unexpected error: %v", err)
	}
	payload, err = appendString(payload, "conn-legacy")
	if err != nil {
		t.Fatalf("appendString(connectionID) unexpected error: %v", err)
	}
	payload = appendUint64(payload, 9)

	decoded, err := DecodeMessagePayload(MessageTypeHandshake, Flags(0x55), payload)
	if err != nil {
		t.Fatalf("DecodeMessagePayload() unexpected error: %v", err)
	}

	got, ok := decoded.(*Handshake)
	if !ok {
		t.Fatalf("decoded = %T, want *Handshake", decoded)
	}
	if got.ClientID != 0 {
		t.Fatalf("got.ClientID = %d, want 0 for legacy payload", got.ClientID)
	}
	if got.ConnectionID != "conn-legacy" || got.Epoch != 9 {
		t.Fatalf("got = %#v, want conn-legacy epoch 9", got)
	}
	if got.Flags != Flags(0x55) {
		t.Fatalf("got.Flags = %#x, want %#x", got.Flags, Flags(0x55))
	}
}

func assertDecodedFlags(t *testing.T, message Message, want Flags) {
	t.Helper()

	var got Flags
	switch m := message.(type) {
	case *Handshake:
		got = m.Flags
	case *HandshakeAck:
		got = m.Flags
	case *DocumentSyncRequest:
		got = m.Flags
	case *DocumentSyncResponse:
		got = m.Flags
	case *DocumentUpdate:
		got = m.Flags
	case *AwarenessUpdate:
		got = m.Flags
	case *QueryAwarenessRequest:
		got = m.Flags
	case *QueryAwarenessResponse:
		got = m.Flags
	case *Disconnect:
		got = m.Flags
	case *Close:
		got = m.Flags
	case *Ping:
		got = m.Flags
	case *Pong:
		got = m.Flags
	default:
		t.Fatalf("unsupported message type %T", message)
	}

	if got != want {
		t.Fatalf("flags = %#x, want %#x", got, want)
	}
}
