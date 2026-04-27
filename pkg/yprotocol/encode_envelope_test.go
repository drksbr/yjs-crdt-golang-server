package yprotocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"yjs-go-bridge/pkg/yawareness"
)

func TestPublicEncodeProtocolEnvelope_RoundTrip(t *testing.T) {
	t.Parallel()

	update := buildGCOnlyUpdate(9, 3)
	awareness := &yawareness.Update{
		Clients: []yawareness.ClientState{
			{ClientID: 7, Clock: 4, State: json.RawMessage(`{"name":"ramon","online":true}`)},
		},
	}

	authWant, err := EncodeProtocolAuthMessage(AuthMessageTypePermissionDenied, "forbidden")
	if err != nil {
		t.Fatalf("EncodeProtocolAuthMessage() unexpected error: %v", err)
	}
	awarenessWant, err := EncodeProtocolAwarenessUpdate(awareness)
	if err != nil {
		t.Fatalf("EncodeProtocolAwarenessUpdate() unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		message *ProtocolMessage
		want    []byte
	}{
		{
			name: "sync update",
			message: &ProtocolMessage{
				Protocol: ProtocolTypeSync,
				Sync: &SyncMessage{
					Type:    SyncMessageTypeUpdate,
					Payload: update,
				},
			},
			want: EncodeProtocolSyncUpdate(update),
		},
		{
			name: "auth permission denied",
			message: &ProtocolMessage{
				Protocol: ProtocolTypeAuth,
				Auth: &AuthMessage{
					Type:   AuthMessageTypePermissionDenied,
					Reason: "forbidden",
				},
			},
			want: authWant,
		},
		{
			name: "awareness update",
			message: &ProtocolMessage{
				Protocol:  ProtocolTypeAwareness,
				Awareness: awareness,
			},
			want: awarenessWant,
		},
		{
			name: "query awareness",
			message: &ProtocolMessage{
				Protocol:       ProtocolTypeQueryAwareness,
				QueryAwareness: &QueryAwarenessMessage{},
			},
			want: EncodeProtocolQueryAwareness(),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := EncodeProtocolEnvelope(tt.message)
			if err != nil {
				t.Fatalf("EncodeProtocolEnvelope() unexpected error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("EncodeProtocolEnvelope() = %v, want %v", got, tt.want)
			}

			decoded, err := DecodeProtocolMessage(got)
			if err != nil {
				t.Fatalf("DecodeProtocolMessage() unexpected error: %v", err)
			}
			assertPublicProtocolMessageEqual(t, decoded, tt.message)
		})
	}
}

func TestPublicEncodeProtocolEnvelopes_StreamRoundTrip(t *testing.T) {
	t.Parallel()

	update := buildGCOnlyUpdate(11, 2)
	awareness := &yawareness.Update{
		Clients: []yawareness.ClientState{
			{ClientID: 3, Clock: 5, State: json.RawMessage(`{"role":"editor"}`)},
		},
	}

	messages := []*ProtocolMessage{
		{
			Protocol: ProtocolTypeSync,
			Sync: &SyncMessage{
				Type:    SyncMessageTypeStep2,
				Payload: update,
			},
		},
		{
			Protocol:  ProtocolTypeAwareness,
			Awareness: awareness,
		},
		{
			Protocol: ProtocolTypeAuth,
			Auth: &AuthMessage{
				Type:   AuthMessageTypePermissionDenied,
				Reason: "denied",
			},
		},
		{
			Protocol:       ProtocolTypeQueryAwareness,
			QueryAwareness: &QueryAwarenessMessage{},
		},
	}

	stream, err := EncodeProtocolEnvelopes(messages...)
	if err != nil {
		t.Fatalf("EncodeProtocolEnvelopes() unexpected error: %v", err)
	}

	var expected []byte
	for _, message := range messages {
		encoded, err := EncodeProtocolEnvelope(message)
		if err != nil {
			t.Fatalf("EncodeProtocolEnvelope() unexpected error while building expected stream: %v", err)
		}
		expected = append(expected, encoded...)
	}
	if !bytes.Equal(stream, expected) {
		t.Fatalf("EncodeProtocolEnvelopes() = %v, want %v", stream, expected)
	}

	decoded, err := DecodeProtocolMessages(stream)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(decoded) != len(messages) {
		t.Fatalf("len(decoded) = %d, want %d", len(decoded), len(messages))
	}
	for i := range decoded {
		assertPublicProtocolMessageEqual(t, decoded[i], messages[i])
	}

	fromStream, err := ReadProtocolMessagesFromStream(nil, bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("ReadProtocolMessagesFromStream() unexpected error: %v", err)
	}
	if len(fromStream) != len(messages) {
		t.Fatalf("len(fromStream) = %d, want %d", len(fromStream), len(messages))
	}
	for i := range fromStream {
		assertPublicProtocolMessageEqual(t, fromStream[i], messages[i])
	}
}

func TestPublicEncodeProtocolEnvelope_Errors(t *testing.T) {
	t.Parallel()

	validSync := &ProtocolMessage{
		Protocol: ProtocolTypeSync,
		Sync: &SyncMessage{
			Type:    SyncMessageTypeStep1,
			Payload: []byte{0x00},
		},
	}

	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{
			name: "nil single message",
			run: func() error {
				_, err := EncodeProtocolEnvelope(nil)
				return err
			},
			want: ErrNilProtocolMessage,
		},
		{
			name: "nil inside stream",
			run: func() error {
				_, err := EncodeProtocolEnvelopes(validSync, nil)
				return err
			},
			want: ErrNilProtocolMessage,
		},
		{
			name: "sync without sync payload",
			run: func() error {
				_, err := EncodeProtocolEnvelope(&ProtocolMessage{Protocol: ProtocolTypeSync})
				return err
			},
			want: ErrInvalidProtocolMessage,
		},
		{
			name: "auth with mismatched payload field",
			run: func() error {
				_, err := EncodeProtocolEnvelope(&ProtocolMessage{
					Protocol: ProtocolTypeAuth,
					Sync: &SyncMessage{
						Type:    SyncMessageTypeUpdate,
						Payload: buildGCOnlyUpdate(1, 1),
					},
				})
				return err
			},
			want: ErrInvalidProtocolMessage,
		},
		{
			name: "multiple payloads set",
			run: func() error {
				_, err := EncodeProtocolEnvelope(&ProtocolMessage{
					Protocol: ProtocolTypeSync,
					Sync: &SyncMessage{
						Type:    SyncMessageTypeUpdate,
						Payload: buildGCOnlyUpdate(2, 1),
					},
					Auth: &AuthMessage{
						Type:   AuthMessageTypePermissionDenied,
						Reason: "forbidden",
					},
				})
				return err
			},
			want: ErrInvalidProtocolMessage,
		},
		{
			name: "query-awareness with extra auth payload",
			run: func() error {
				_, err := EncodeProtocolEnvelope(&ProtocolMessage{
					Protocol: ProtocolTypeQueryAwareness,
					Auth: &AuthMessage{
						Type:   AuthMessageTypePermissionDenied,
						Reason: "forbidden",
					},
				})
				return err
			},
			want: ErrInvalidProtocolMessage,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run()
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func assertPublicProtocolMessageEqual(t *testing.T, got, want *ProtocolMessage) {
	t.Helper()

	if got == nil {
		t.Fatal("got = nil, want non-nil")
	}
	if want == nil {
		t.Fatal("want = nil, test bug")
	}
	if got.Protocol != want.Protocol {
		t.Fatalf("got.Protocol = %v, want %v", got.Protocol, want.Protocol)
	}

	switch want.Protocol {
	case ProtocolTypeSync:
		if got.Sync == nil || want.Sync == nil {
			t.Fatalf("sync payload mismatch: got=%#v want=%#v", got.Sync, want.Sync)
		}
		if got.Awareness != nil || got.Auth != nil || got.QueryAwareness != nil {
			t.Fatalf("sync decode returned extra payloads: %#v", got)
		}
		if got.Sync.Type != want.Sync.Type {
			t.Fatalf("got.Sync.Type = %v, want %v", got.Sync.Type, want.Sync.Type)
		}
		if !bytes.Equal(got.Sync.Payload, want.Sync.Payload) {
			t.Fatalf("got.Sync.Payload = %v, want %v", got.Sync.Payload, want.Sync.Payload)
		}
	case ProtocolTypeAwareness:
		if got.Awareness == nil || want.Awareness == nil {
			t.Fatalf("awareness payload mismatch: got=%#v want=%#v", got.Awareness, want.Awareness)
		}
		if got.Sync != nil || got.Auth != nil || got.QueryAwareness != nil {
			t.Fatalf("awareness decode returned extra payloads: %#v", got)
		}
		if len(got.Awareness.Clients) != len(want.Awareness.Clients) {
			t.Fatalf("len(got.Awareness.Clients) = %d, want %d", len(got.Awareness.Clients), len(want.Awareness.Clients))
		}
		for i := range want.Awareness.Clients {
			gotClient := got.Awareness.Clients[i]
			wantClient := want.Awareness.Clients[i]
			if gotClient.ClientID != wantClient.ClientID {
				t.Fatalf("got.Awareness.Clients[%d].ClientID = %d, want %d", i, gotClient.ClientID, wantClient.ClientID)
			}
			if gotClient.Clock != wantClient.Clock {
				t.Fatalf("got.Awareness.Clients[%d].Clock = %d, want %d", i, gotClient.Clock, wantClient.Clock)
			}
			if !bytes.Equal(gotClient.State, wantClient.State) {
				t.Fatalf("got.Awareness.Clients[%d].State = %s, want %s", i, gotClient.State, wantClient.State)
			}
		}
	case ProtocolTypeAuth:
		if got.Auth == nil || want.Auth == nil {
			t.Fatalf("auth payload mismatch: got=%#v want=%#v", got.Auth, want.Auth)
		}
		if got.Sync != nil || got.Awareness != nil || got.QueryAwareness != nil {
			t.Fatalf("auth decode returned extra payloads: %#v", got)
		}
		if got.Auth.Type != want.Auth.Type {
			t.Fatalf("got.Auth.Type = %v, want %v", got.Auth.Type, want.Auth.Type)
		}
		if got.Auth.Reason != want.Auth.Reason {
			t.Fatalf("got.Auth.Reason = %q, want %q", got.Auth.Reason, want.Auth.Reason)
		}
	case ProtocolTypeQueryAwareness:
		if got.QueryAwareness == nil || want.QueryAwareness == nil {
			t.Fatalf("query-awareness payload mismatch: got=%#v want=%#v", got.QueryAwareness, want.QueryAwareness)
		}
		if got.Sync != nil || got.Awareness != nil || got.Auth != nil {
			t.Fatalf("query-awareness decode returned extra payloads: %#v", got)
		}
	default:
		t.Fatalf("unexpected protocol in test: %v", want.Protocol)
	}
}
