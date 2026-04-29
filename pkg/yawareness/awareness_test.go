package yawareness

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	internalProtocol "github.com/drksbr/yjs-crdt-golang-server/internal/yprotocol"
)

func TestPublicAwarenessEncodeDecodeContract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		update *Update
	}{
		{
			name: "payload round trip",
			update: &Update{
				Clients: []ClientState{
					{ClientID: 1, Clock: 2, State: json.RawMessage(`{"name":"ramon"}`)},
					{ClientID: 9, Clock: 4, State: nil},
				},
			},
		},
		{
			name:   "nil update",
			update: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := EncodeUpdate(tc.update)
			if err != nil {
				t.Fatalf("EncodeUpdate() unexpected error: %v", err)
			}

			decoded, err := DecodeUpdate(encoded)
			if err != nil {
				t.Fatalf("DecodeUpdate() unexpected error: %v", err)
			}

			if tc.update == nil {
				if len(decoded.Clients) != 0 {
					t.Fatalf("len(decoded.Clients) = %d, want 0", len(decoded.Clients))
				}
				return
			}

			if len(decoded.Clients) != 2 {
				t.Fatalf("len(decoded.Clients) = %d, want 2", len(decoded.Clients))
			}
			if !bytes.Equal(decoded.Clients[0].State, []byte(`{"name":"ramon"}`)) {
				t.Fatalf("decoded.Clients[0].State = %s", decoded.Clients[0].State)
			}
			if !decoded.Clients[1].IsNull() {
				t.Fatalf("decoded.Clients[1] = %+v, want tombstone", decoded.Clients[1])
			}
		})
	}
}

func TestPublicAwarenessAppendUpdateAppendsIntoBuffer(t *testing.T) {
	t.Parallel()

	prefix := []byte{0x11, 0x22}
	encoded, err := AppendUpdate(prefix, &Update{
		Clients: []ClientState{
			{ClientID: 7, Clock: 3, State: json.RawMessage(`{"name":"alice"}`)},
		},
	})
	if err != nil {
		t.Fatalf("AppendUpdate() unexpected error: %v", err)
	}

	if !bytes.Equal(encoded[:len(prefix)], prefix) {
		t.Fatalf("prefix changed, got=%v want=%v", encoded[:len(prefix)], prefix)
	}

	decoded, err := DecodeUpdate(encoded[len(prefix):])
	if err != nil {
		t.Fatalf("DecodeUpdate() unexpected error: %v", err)
	}
	if len(decoded.Clients) != 1 || decoded.Clients[0].ClientID != 7 {
		t.Fatalf("decoded = %#v, want one client=7", decoded)
	}
}

func TestPublicAwarenessProtocolRoundTrip(t *testing.T) {
	t.Parallel()

	update := &Update{
		Clients: []ClientState{
			{ClientID: 5, Clock: 1, State: json.RawMessage(`{"online":true}`)},
		},
	}

	encoded, err := EncodeProtocolUpdate(update)
	if err != nil {
		t.Fatalf("EncodeProtocolUpdate() unexpected error: %v", err)
	}

	decoded, err := DecodeProtocolUpdate(encoded)
	if err != nil {
		t.Fatalf("DecodeProtocolUpdate() unexpected error: %v", err)
	}
	if len(decoded.Clients) != 1 || decoded.Clients[0].ClientID != 5 {
		t.Fatalf("decoded = %#v, want one client=5", decoded)
	}
}

func TestPublicAwarenessProtocolEnvelopeContract(t *testing.T) {
	t.Parallel()

	payload, err := EncodeUpdate(&Update{
		Clients: []ClientState{
			{ClientID: 5, Clock: 1, State: json.RawMessage(`{"online":true}`)},
		},
	})
	if err != nil {
		t.Fatalf("EncodeUpdate() unexpected error: %v", err)
	}

	encoded, err := EncodeProtocolUpdate(&Update{
		Clients: []ClientState{
			{ClientID: 5, Clock: 1, State: json.RawMessage(`{"online":true}`)},
		},
	})
	if err != nil {
		t.Fatalf("EncodeProtocolUpdate() unexpected error: %v", err)
	}

	if _, err := DecodeProtocolUpdate(append(encoded, 0x00)); !errors.Is(err, ErrTrailingBytes) {
		t.Fatalf("DecodeProtocolUpdate() trailing bytes error = %v, want %v", err, ErrTrailingBytes)
	}

	wrongTypePayload := internalProtocol.AppendProtocolType(nil, internalProtocol.ProtocolTypeSync)
	wrongTypePayload = append(wrongTypePayload, payload...)
	_, err = DecodeProtocolUpdate(wrongTypePayload)
	if err == nil {
		t.Fatal("DecodeProtocolUpdate() expected protocol mismatch error")
	}
	if !errors.Is(err, internalProtocol.ErrUnexpectedProtocolType) {
		t.Fatalf("DecodeProtocolUpdate() error = %v, want %v", err, internalProtocol.ErrUnexpectedProtocolType)
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("DecodeProtocolUpdate() expected ParseError, got %T", err)
	}
}

func TestPublicAwarenessStateManagerLocalProtectionContract(t *testing.T) {
	t.Parallel()

	manager := NewStateManager(7)
	start := time.Unix(1700001000, 0)
	if err := manager.SetLocalStateAt(json.RawMessage(`{"cursor":1}`), start); err != nil {
		t.Fatalf("SetLocalStateAt() unexpected error: %v", err)
	}

	updated, err := manager.RenewLocalIfDueAt(start.Add(OutdatedTimeout/2), OutdatedTimeout)
	if err != nil {
		t.Fatalf("RenewLocalIfDueAt() unexpected error: %v", err)
	}
	if !updated {
		t.Fatal("RenewLocalIfDueAt() = false, want true")
	}

	manager.ApplyAt(&Update{
		Clients: []ClientState{
			{ClientID: 7, Clock: 1, State: nil},
		},
	}, start.Add(time.Second))
	client, ok := manager.Get(7)
	if !ok || !bytes.Equal(client.State, []byte(`{"cursor":1}`)) {
		t.Fatalf("local client should be preserved, got=%#v ok=%v", client, ok)
	}

	snapshot := manager.Snapshot()
	if len(snapshot.Clients) != 1 || snapshot.Clients[0].ClientID != 7 {
		t.Fatalf("Snapshot() = %#v, want local client 7", snapshot)
	}
}

func TestPublicAwarenessStateManagerChangeContract(t *testing.T) {
	t.Parallel()

	manager := NewStateManager(11)
	start := time.Unix(1700001010, 0)

	change, err := manager.SetLocalStateWithChangeAt(json.RawMessage(`{"cursor":1}`), start)
	if err != nil {
		t.Fatalf("SetLocalStateWithChangeAt() unexpected error: %v", err)
	}
	if len(change.Added) != 1 || change.Added[0] != 11 || change.Empty() {
		t.Fatalf("local change = %#v, want added client 11", change)
	}

	change, err = manager.SetLocalStateFieldWithChangeAt("name", json.RawMessage(`"alice"`), start.Add(time.Second))
	if err != nil {
		t.Fatalf("SetLocalStateFieldWithChangeAt() unexpected error: %v", err)
	}
	if len(change.Updated) != 1 || change.Updated[0] != 11 {
		t.Fatalf("field change = %#v, want updated client 11", change)
	}

	remoteChange := manager.ApplyWithChangeAt(&Update{
		Clients: []ClientState{
			{ClientID: 22, Clock: 1, State: json.RawMessage(`{"name":"remote"}`)},
			{ClientID: 22, Clock: 2, State: nil},
		},
	}, start.Add(2*time.Second))
	if len(remoteChange.Removed) != 1 || remoteChange.Removed[0] != 22 {
		t.Fatalf("remoteChange = %#v, want removed client 22", remoteChange)
	}
}

func TestPublicAwarenessErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid-json-encode", func(t *testing.T) {
		t.Parallel()
		_, err := EncodeUpdate(&Update{
			Clients: []ClientState{{ClientID: 1, Clock: 1, State: json.RawMessage(`{`)}},
		})
		if !errors.Is(err, ErrInvalidJSON) {
			t.Fatalf("EncodeUpdate() error = %v, want %v", err, ErrInvalidJSON)
		}
	})

	t.Run("invalid-json-decode", func(t *testing.T) {
		t.Parallel()
		invalid := []byte{0x01, 0x01, 0x01, 0x08, 'n', 'o', 't', '-', 'j', 's', 'o', 'n'}
		_, err := DecodeUpdate(invalid)
		if !errors.Is(err, ErrInvalidJSON) {
			t.Fatalf("DecodeUpdate() error = %v, want %v", err, ErrInvalidJSON)
		}
	})
}
