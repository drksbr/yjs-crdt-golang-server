package yprotocol

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/yawareness"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestSessionRuntimeContracts(t *testing.T) {
	t.Run("session exposes v2 canonical state with v1 compatibility", func(t *testing.T) {
		session := NewSession(10)

		emptyV2 := session.UpdateV2()
		assertProtocolV2PayloadEquivalentToV1(t, emptyV2, yjsbridge.NewPersistedSnapshot().UpdateV1)

		update := buildGCOnlyUpdate(10, 2)
		if err := session.LoadUpdate(update); err != nil {
			t.Fatalf("LoadUpdate(update) unexpected error: %v", err)
		}
		if !bytes.Equal(session.UpdateV1(), update) {
			t.Fatalf("session.UpdateV1() = %x, want compatibility V1 %x", session.UpdateV1(), update)
		}
		assertProtocolV2PayloadEquivalentToV1(t, session.UpdateV2(), update)

		mutated := session.UpdateV2()
		mutated[0] ^= 0xff
		if bytes.Equal(session.UpdateV2(), mutated) {
			t.Fatal("session.UpdateV2() returned aliased storage")
		}
	})

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

	t.Run("sync step1 can opt into v2 step2 output", func(t *testing.T) {
		session := NewSession(12)

		update := buildGCOnlyUpdate(12, 2)
		if err := session.LoadUpdate(update); err != nil {
			t.Fatalf("LoadUpdate(update) unexpected error: %v", err)
		}
		expectedV1, err := yjsbridge.DiffUpdate(update, []byte{0x00})
		if err != nil {
			t.Fatalf("DiffUpdate() unexpected error: %v", err)
		}

		encodedResponse, err := session.HandleEncodedMessagesWithOptions(
			EncodeProtocolSyncStep1([]byte{0x00}),
			SessionHandleOptions{SyncOutputFormat: yjsbridge.UpdateFormatV2},
		)
		if err != nil {
			t.Fatalf("HandleEncodedMessagesWithOptions(v2) unexpected error: %v", err)
		}

		responses, err := DecodeProtocolMessages(encodedResponse)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
		}
		if len(responses) != 1 || responses[0].Sync == nil {
			t.Fatalf("responses = %#v, want single sync response", responses)
		}
		if responses[0].Sync.Type != SyncMessageTypeStep2 {
			t.Fatalf("responses[0].Sync.Type = %v, want %v", responses[0].Sync.Type, SyncMessageTypeStep2)
		}
		format, err := yjsbridge.FormatFromUpdate(responses[0].Sync.Payload)
		if err != nil {
			t.Fatalf("FormatFromUpdate(v2 step2) unexpected error: %v", err)
		}
		if format != yjsbridge.UpdateFormatV2 {
			t.Fatalf("FormatFromUpdate(v2 step2) = %s, want %s", format, yjsbridge.UpdateFormatV2)
		}
		gotV1, err := yjsbridge.ConvertUpdateToV1(responses[0].Sync.Payload)
		if err != nil {
			t.Fatalf("ConvertUpdateToV1(v2 step2) unexpected error: %v", err)
		}
		if !bytes.Equal(gotV1, expectedV1) {
			t.Fatalf("ConvertUpdateToV1(v2 step2) = %x, want %x", gotV1, expectedV1)
		}
		if !bytes.Equal(session.UpdateV1(), update) {
			t.Fatalf("session.UpdateV1() changed after v2 egress")
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

	t.Run("sync update and step2 keep v2 canonical state with v1 compatibility", func(t *testing.T) {
		session := NewSession(22)

		v2Update := mustDecodeProtocolHex(t, "000002a50100000104060374686901020101000001010000")
		v1Update, err := yjsbridge.ConvertUpdateToV1(v2Update)
		if err != nil {
			t.Fatalf("ConvertUpdateToV1(v2) unexpected error: %v", err)
		}
		v2Step2 := mustDecodeProtocolHex(t, "0000028a0300000104060374686901020101000001010000")
		v1Step2, err := yjsbridge.ConvertUpdateToV1(v2Step2)
		if err != nil {
			t.Fatalf("ConvertUpdateToV1(v2Step2) unexpected error: %v", err)
		}

		responses, err := session.HandleProtocolMessages(
			mustDecodeProtocolMessage(t, EncodeProtocolSyncUpdate(v2Update)),
			mustDecodeProtocolMessage(t, EncodeProtocolSyncStep2(v2Step2)),
		)
		if err != nil {
			t.Fatalf("HandleProtocolMessages(v2) unexpected error: %v", err)
		}
		if len(responses) != 0 {
			t.Fatalf("len(responses) = %d, want 0", len(responses))
		}

		want, err := yjsbridge.MergeUpdates(v1Update, v1Step2)
		if err != nil {
			t.Fatalf("MergeUpdates(converted v1) unexpected error: %v", err)
		}
		if got := session.UpdateV1(); !bytes.Equal(got, want) {
			t.Fatalf("session.UpdateV1() = %x, want %x", got, want)
		}
		assertProtocolV2PayloadEquivalentToV1(t, session.UpdateV2(), want)
		snapshot, err := session.PersistedSnapshot()
		if err != nil {
			t.Fatalf("PersistedSnapshot() unexpected error: %v", err)
		}
		if !bytes.Equal(snapshot.UpdateV1, want) {
			t.Fatalf("snapshot.UpdateV1 = %x, want %x", snapshot.UpdateV1, want)
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
