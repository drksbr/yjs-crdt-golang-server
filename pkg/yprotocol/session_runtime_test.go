package yprotocol

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yawareness"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestSessionRuntimeContracts(t *testing.T) {
	t.Run("sync step1 returns sync step2 via encoded handler", func(t *testing.T) {
		session := NewSession(1)

		left := buildGCOnlyUpdate(1, 2)
		right := buildGCOnlyUpdate(4, 1)
		merged, err := yjsbridge.MergeUpdates(left, right)
		if err != nil {
			t.Fatalf("MergeUpdates() unexpected error: %v", err)
		}
		if err := session.LoadUpdate(merged); err != nil {
			t.Fatalf("LoadUpdate(merged) unexpected error: %v", err)
		}
		expected, err := yjsbridge.DiffUpdate(merged, []byte{0x00})
		if err != nil {
			t.Fatalf("DiffUpdate() unexpected error: %v", err)
		}

		encodedResponse, err := session.HandleEncodedMessages(EncodeProtocolSyncStep1([]byte{0x00}))
		if err != nil {
			t.Fatalf("HandleEncodedMessages() unexpected error: %v", err)
		}

		responses, err := DecodeProtocolMessages(encodedResponse)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
		}
		if len(responses) != 1 {
			t.Fatalf("len(responses) = %d, want 1", len(responses))
		}
		if responses[0].Protocol != ProtocolTypeSync || responses[0].Sync == nil {
			t.Fatalf("responses[0] = %#v, want sync response", responses[0])
		}
		if responses[0].Sync.Type != SyncMessageTypeStep2 {
			t.Fatalf("responses[0].Sync.Type = %v, want %v", responses[0].Sync.Type, SyncMessageTypeStep2)
		}
		if !bytes.Equal(responses[0].Sync.Payload, expected) {
			t.Fatalf("responses[0].Sync.Payload = %v, want %v", responses[0].Sync.Payload, expected)
		}
	})

	t.Run("sync update and step2 are merged into persisted snapshot", func(t *testing.T) {
		session := NewSession(2)

		base := buildGCOnlyUpdate(1, 1)
		incremental := buildGCOnlyUpdate(3, 2)
		step2Update := buildGCOnlyUpdate(8, 1)

		if err := session.LoadUpdate(base); err != nil {
			t.Fatalf("LoadUpdate(base) unexpected error: %v", err)
		}

		messages := []*ProtocolMessage{
			mustDecodeProtocolMessage(t, EncodeProtocolSyncUpdate(incremental)),
			mustDecodeProtocolMessage(t, EncodeProtocolSyncStep2(step2Update)),
		}

		responses, err := session.HandleProtocolMessages(messages...)
		if err != nil {
			t.Fatalf("HandleProtocolMessages() unexpected error: %v", err)
		}
		if len(responses) != 0 {
			t.Fatalf("len(responses) = %d, want 0", len(responses))
		}

		snapshot, err := session.PersistedSnapshot()
		if err != nil {
			t.Fatalf("PersistedSnapshot() unexpected error: %v", err)
		}

		expected, err := yjsbridge.MergeUpdates(base, incremental, step2Update)
		if err != nil {
			t.Fatalf("MergeUpdates() unexpected error: %v", err)
		}
		if !bytes.Equal(snapshot.UpdateV1, expected) {
			t.Fatalf("snapshot.UpdateV1 = %v, want %v", snapshot.UpdateV1, expected)
		}
	})

	t.Run("query-awareness returns the current awareness snapshot", func(t *testing.T) {
		session := NewSession(3)

		incoming := &yawareness.Update{
			Clients: []yawareness.ClientState{
				{
					ClientID: 77,
					Clock:    3,
					State:    json.RawMessage(`{"name":"runtime","online":true}`),
				},
			},
		}
		encodedIncoming, err := EncodeProtocolAwarenessUpdate(incoming)
		if err != nil {
			t.Fatalf("EncodeProtocolAwarenessUpdate() unexpected error: %v", err)
		}

		responses, err := session.HandleProtocolMessage(mustDecodeProtocolMessage(t, encodedIncoming))
		if err != nil {
			t.Fatalf("HandleProtocolMessage(awareness) unexpected error: %v", err)
		}
		if len(responses) != 0 {
			t.Fatalf("len(responses) after awareness update = %d, want 0", len(responses))
		}

		responses, err = session.HandleProtocolMessage(mustDecodeProtocolMessage(t, EncodeProtocolQueryAwareness()))
		if err != nil {
			t.Fatalf("HandleProtocolMessage(query-awareness) unexpected error: %v", err)
		}
		if len(responses) != 1 {
			t.Fatalf("len(responses) = %d, want 1", len(responses))
		}
		if responses[0].Protocol != ProtocolTypeAwareness || responses[0].Awareness == nil {
			t.Fatalf("responses[0] = %#v, want awareness snapshot", responses[0])
		}
		if len(responses[0].Awareness.Clients) != 1 {
			t.Fatalf("len(responses[0].Awareness.Clients) = %d, want 1", len(responses[0].Awareness.Clients))
		}

		client := responses[0].Awareness.Clients[0]
		if client.ClientID != 77 || client.Clock != 3 {
			t.Fatalf("client = %#v, want clientID=77 clock=3", client)
		}
		if !bytes.Equal(client.State, []byte(`{"name":"runtime","online":true}`)) {
			t.Fatalf("client.State = %s, want awareness snapshot state", client.State)
		}
	})

	t.Run("encoded query-awareness handshake applies remote clock zero snapshot", func(t *testing.T) {
		server := NewSession(11)
		client := NewSession(12)

		if err := server.Awareness().SetLocalState(json.RawMessage(`{"name":"server","status":"ready"}`)); err != nil {
			t.Fatalf("server.Awareness().SetLocalState() unexpected error: %v", err)
		}

		hello, err := EncodeProtocolEnvelopes(
			&ProtocolMessage{
				Protocol:       ProtocolTypeQueryAwareness,
				QueryAwareness: &QueryAwarenessMessage{},
			},
		)
		if err != nil {
			t.Fatalf("EncodeProtocolEnvelopes() unexpected error: %v", err)
		}

		reply, err := server.HandleEncodedMessages(hello)
		if err != nil {
			t.Fatalf("server.HandleEncodedMessages() unexpected error: %v", err)
		}
		if len(reply) == 0 {
			t.Fatal("server.HandleEncodedMessages() returned empty reply, want awareness snapshot")
		}

		followUp, err := client.HandleEncodedMessages(reply)
		if err != nil {
			t.Fatalf("client.HandleEncodedMessages() unexpected error: %v", err)
		}
		if len(followUp) != 0 {
			t.Fatalf("len(followUp) = %d, want 0", len(followUp))
		}

		snapshot := client.Awareness().Snapshot()
		if len(snapshot.Clients) != 1 {
			t.Fatalf("len(snapshot.Clients) = %d, want 1", len(snapshot.Clients))
		}
		if snapshot.Clients[0].ClientID != 11 || snapshot.Clients[0].Clock != 0 {
			t.Fatalf("snapshot.Clients[0] = %#v, want clientID=11 clock=0", snapshot.Clients[0])
		}
		if !bytes.Equal(snapshot.Clients[0].State, []byte(`{"name":"server","status":"ready"}`)) {
			t.Fatalf("snapshot.Clients[0].State = %s, want server awareness payload", snapshot.Clients[0].State)
		}
	})

	t.Run("auth is a no-op for state and response", func(t *testing.T) {
		session := NewSession(4)

		update := buildGCOnlyUpdate(2, 3)
		if err := session.LoadUpdate(update); err != nil {
			t.Fatalf("LoadUpdate() unexpected error: %v", err)
		}

		before, err := session.PersistedSnapshot()
		if err != nil {
			t.Fatalf("PersistedSnapshot() unexpected error: %v", err)
		}

		responses, err := session.HandleProtocolMessage(mustDecodeProtocolMessage(t, EncodeProtocolAuthPermissionDenied("ignored")))
		if err != nil {
			t.Fatalf("HandleProtocolMessage(auth) unexpected error: %v", err)
		}
		if len(responses) != 0 {
			t.Fatalf("len(responses) = %d, want 0", len(responses))
		}

		after, err := session.PersistedSnapshot()
		if err != nil {
			t.Fatalf("PersistedSnapshot() unexpected error: %v", err)
		}
		if !bytes.Equal(after.UpdateV1, before.UpdateV1) {
			t.Fatalf("after.UpdateV1 = %v, want %v", after.UpdateV1, before.UpdateV1)
		}
	})

	t.Run("persisted snapshot can restore a new session", func(t *testing.T) {
		original := NewSession(5)

		left := buildGCOnlyUpdate(5, 1)
		right := buildGCOnlyUpdate(9, 2)
		if err := original.LoadUpdate(left); err != nil {
			t.Fatalf("LoadUpdate(left) unexpected error: %v", err)
		}
		responses, err := original.HandleProtocolMessages([]*ProtocolMessage{
			mustDecodeProtocolMessage(t, EncodeProtocolSyncUpdate(right)),
		}...)
		if err != nil {
			t.Fatalf("HandleProtocolMessages() unexpected error: %v", err)
		}
		if len(responses) != 0 {
			t.Fatalf("len(responses) = %d, want 0", len(responses))
		}

		stored, err := original.PersistedSnapshot()
		if err != nil {
			t.Fatalf("PersistedSnapshot() unexpected error: %v", err)
		}

		restored := NewSession(6)
		if err := restored.LoadPersistedSnapshot(stored); err != nil {
			t.Fatalf("LoadPersistedSnapshot(stored) unexpected error: %v", err)
		}

		snapshot, err := restored.PersistedSnapshot()
		if err != nil {
			t.Fatalf("PersistedSnapshot() unexpected error: %v", err)
		}
		if !bytes.Equal(snapshot.UpdateV1, stored.UpdateV1) {
			t.Fatalf("snapshot.UpdateV1 = %v, want %v", snapshot.UpdateV1, stored.UpdateV1)
		}

		encodedResponse, err := restored.HandleEncodedMessages(EncodeProtocolSyncStep1([]byte{0x00}))
		if err != nil {
			t.Fatalf("HandleEncodedMessages() unexpected error: %v", err)
		}
		messages, err := DecodeProtocolMessages(encodedResponse)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
		}
		if len(messages) != 1 || messages[0].Sync == nil || messages[0].Sync.Type != SyncMessageTypeStep2 {
			t.Fatalf("messages = %#v, want single sync step2 response", messages)
		}
		expected, err := yjsbridge.DiffUpdate(stored.UpdateV1, []byte{0x00})
		if err != nil {
			t.Fatalf("DiffUpdate() unexpected error: %v", err)
		}
		if !bytes.Equal(messages[0].Sync.Payload, expected) {
			t.Fatalf("messages[0].Sync.Payload = %v, want %v", messages[0].Sync.Payload, expected)
		}
	})
}

func mustDecodeProtocolMessage(t *testing.T, src []byte) *ProtocolMessage {
	t.Helper()

	message, err := DecodeProtocolMessage(src)
	if err != nil {
		t.Fatalf("DecodeProtocolMessage() unexpected error: %v", err)
	}
	return message
}
