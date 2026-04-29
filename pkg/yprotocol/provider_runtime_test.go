package yprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yawareness"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
)

func TestProviderRuntimeContracts(t *testing.T) {
	t.Run("handshake between two connections uses room snapshot", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-handshake"}
		store := memory.New()

		seedUpdate := buildGCOnlyUpdate(7, 3)
		seedSnapshot, err := yjsbridge.PersistedSnapshotFromUpdate(seedUpdate)
		if err != nil {
			t.Fatalf("PersistedSnapshotFromUpdate() unexpected error: %v", err)
		}
		if _, err := store.SaveSnapshot(ctx, key, seedSnapshot); err != nil {
			t.Fatalf("SaveSnapshot() unexpected error: %v", err)
		}

		provider := NewProvider(ProviderConfig{Store: store})
		left, err := provider.Open(ctx, key, "conn-a", 101)
		if err != nil {
			t.Fatalf("provider.Open(conn-a) unexpected error: %v", err)
		}
		right, err := provider.Open(ctx, key, "conn-b", 202)
		if err != nil {
			t.Fatalf("provider.Open(conn-b) unexpected error: %v", err)
		}

		expectedStep2, err := yjsbridge.DiffUpdate(seedSnapshot.UpdateV1, []byte{0x00})
		if err != nil {
			t.Fatalf("DiffUpdate() unexpected error: %v", err)
		}

		for _, connection := range []*Connection{left, right} {
			result, err := connection.HandleEncodedMessages(EncodeProtocolSyncStep1([]byte{0x00}))
			if err != nil {
				t.Fatalf("%s.HandleEncodedMessages(step1) unexpected error: %v", connection.ID(), err)
			}
			if len(result.Broadcast) != 0 {
				t.Fatalf("%s broadcast = %d bytes, want 0", connection.ID(), len(result.Broadcast))
			}

			messages, err := DecodeProtocolMessages(result.Direct)
			if err != nil {
				t.Fatalf("DecodeProtocolMessages(%s direct) unexpected error: %v", connection.ID(), err)
			}
			if len(messages) != 1 || messages[0].Sync == nil {
				t.Fatalf("%s direct messages = %#v, want single sync response", connection.ID(), messages)
			}
			if messages[0].Sync.Type != SyncMessageTypeStep2 {
				t.Fatalf("%s direct sync type = %v, want %v", connection.ID(), messages[0].Sync.Type, SyncMessageTypeStep2)
			}
			if !bytes.Equal(messages[0].Sync.Payload, expectedStep2) {
				t.Fatalf("%s direct sync payload = %v, want %v", connection.ID(), messages[0].Sync.Payload, expectedStep2)
			}
		}
	})

	t.Run("sync update is broadcast to peers", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-broadcast"}
		provider := NewProvider(ProviderConfig{})

		sender, err := provider.Open(ctx, key, "conn-a", 301)
		if err != nil {
			t.Fatalf("provider.Open(conn-a) unexpected error: %v", err)
		}
		peer, err := provider.Open(ctx, key, "conn-b", 302)
		if err != nil {
			t.Fatalf("provider.Open(conn-b) unexpected error: %v", err)
		}

		update := buildGCOnlyUpdate(19, 2)
		result, err := sender.HandleEncodedMessages(EncodeProtocolSyncUpdate(update))
		if err != nil {
			t.Fatalf("sender.HandleEncodedMessages(sync-update) unexpected error: %v", err)
		}
		if len(result.Direct) != 0 {
			t.Fatalf("len(result.Direct) = %d, want 0", len(result.Direct))
		}
		if len(result.Broadcast) == 0 {
			t.Fatal("len(result.Broadcast) = 0, want sync update broadcast")
		}

		broadcastMessages, err := DecodeProtocolMessages(result.Broadcast)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages(broadcast) unexpected error: %v", err)
		}
		if len(broadcastMessages) != 1 || broadcastMessages[0].Sync == nil {
			t.Fatalf("broadcastMessages = %#v, want single sync broadcast", broadcastMessages)
		}
		if broadcastMessages[0].Sync.Type != SyncMessageTypeUpdate {
			t.Fatalf("broadcast sync type = %v, want %v", broadcastMessages[0].Sync.Type, SyncMessageTypeUpdate)
		}
		if !bytes.Equal(broadcastMessages[0].Sync.Payload, update) {
			t.Fatalf("broadcast sync payload = %v, want %v", broadcastMessages[0].Sync.Payload, update)
		}

		probe, err := peer.HandleEncodedMessages(EncodeProtocolSyncStep1([]byte{0x00}))
		if err != nil {
			t.Fatalf("peer.HandleEncodedMessages(step1) unexpected error: %v", err)
		}
		probeMessages, err := DecodeProtocolMessages(probe.Direct)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages(peer direct) unexpected error: %v", err)
		}
		if len(probeMessages) != 1 || probeMessages[0].Sync == nil {
			t.Fatalf("probeMessages = %#v, want single sync step2", probeMessages)
		}
		if probeMessages[0].Sync.Type != SyncMessageTypeStep2 {
			t.Fatalf("probe sync type = %v, want %v", probeMessages[0].Sync.Type, SyncMessageTypeStep2)
		}

		expectedStep2, err := yjsbridge.DiffUpdate(update, []byte{0x00})
		if err != nil {
			t.Fatalf("DiffUpdate() unexpected error: %v", err)
		}
		if !bytes.Equal(probeMessages[0].Sync.Payload, expectedStep2) {
			t.Fatalf("probe sync payload = %v, want %v", probeMessages[0].Sync.Payload, expectedStep2)
		}
	})

	t.Run("query-awareness uses aggregated room awareness and close removes peer state", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-awareness"}
		provider := NewProvider(ProviderConfig{})

		left, err := provider.Open(ctx, key, "conn-a", 401)
		if err != nil {
			t.Fatalf("provider.Open(conn-a) unexpected error: %v", err)
		}
		right, err := provider.Open(ctx, key, "conn-b", 402)
		if err != nil {
			t.Fatalf("provider.Open(conn-b) unexpected error: %v", err)
		}

		leftState := json.RawMessage(`{"name":"left","cursor":1}`)
		rightState := json.RawMessage(`{"name":"right","cursor":2}`)
		leftUpdate := &yawareness.Update{
			Clients: []yawareness.ClientState{{
				ClientID: left.ClientID(),
				Clock:    1,
				State:    leftState,
			}},
		}
		rightUpdate := &yawareness.Update{
			Clients: []yawareness.ClientState{{
				ClientID: right.ClientID(),
				Clock:    1,
				State:    rightState,
			}},
		}

		leftEncoded, err := EncodeProtocolAwarenessUpdate(leftUpdate)
		if err != nil {
			t.Fatalf("EncodeProtocolAwarenessUpdate(left) unexpected error: %v", err)
		}
		rightEncoded, err := EncodeProtocolAwarenessUpdate(rightUpdate)
		if err != nil {
			t.Fatalf("EncodeProtocolAwarenessUpdate(right) unexpected error: %v", err)
		}

		leftResult, err := left.HandleEncodedMessages(leftEncoded)
		if err != nil {
			t.Fatalf("left.HandleEncodedMessages(awareness) unexpected error: %v", err)
		}
		if len(leftResult.Broadcast) == 0 {
			t.Fatal("left awareness broadcast is empty, want peer fanout")
		}

		rightResult, err := right.HandleEncodedMessages(rightEncoded)
		if err != nil {
			t.Fatalf("right.HandleEncodedMessages(awareness) unexpected error: %v", err)
		}
		if len(rightResult.Broadcast) == 0 {
			t.Fatal("right awareness broadcast is empty, want peer fanout")
		}

		query, err := left.HandleEncodedMessages(EncodeProtocolQueryAwareness())
		if err != nil {
			t.Fatalf("left.HandleEncodedMessages(query-awareness) unexpected error: %v", err)
		}

		queryMessages, err := DecodeProtocolMessages(query.Direct)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages(query direct) unexpected error: %v", err)
		}
		if len(queryMessages) != 1 || queryMessages[0].Awareness == nil {
			t.Fatalf("queryMessages = %#v, want single awareness response", queryMessages)
		}

		got := awarenessStatesByClient(queryMessages[0].Awareness)
		if len(got) != 2 {
			t.Fatalf("len(awareness states) = %d, want 2", len(got))
		}
		if !bytes.Equal(got[left.ClientID()], leftState) {
			t.Fatalf("left awareness state = %s, want %s", got[left.ClientID()], leftState)
		}
		if !bytes.Equal(got[right.ClientID()], rightState) {
			t.Fatalf("right awareness state = %s, want %s", got[right.ClientID()], rightState)
		}

		closeResult, err := right.Close()
		if err != nil {
			t.Fatalf("right.Close() unexpected error: %v", err)
		}
		if len(closeResult.Broadcast) == 0 {
			t.Fatal("right.Close() broadcast is empty, want tombstone awareness")
		}

		queryAfterClose, err := left.HandleEncodedMessages(EncodeProtocolQueryAwareness())
		if err != nil {
			t.Fatalf("left.HandleEncodedMessages(query-awareness after close) unexpected error: %v", err)
		}
		queryAfterCloseMessages, err := DecodeProtocolMessages(queryAfterClose.Direct)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages(query after close direct) unexpected error: %v", err)
		}
		if len(queryAfterCloseMessages) != 1 || queryAfterCloseMessages[0].Awareness == nil {
			t.Fatalf("queryAfterCloseMessages = %#v, want single awareness response", queryAfterCloseMessages)
		}

		afterClose := awarenessStatesByClient(queryAfterCloseMessages[0].Awareness)
		if len(afterClose) != 1 {
			t.Fatalf("len(awareness states after close) = %d, want 1", len(afterClose))
		}
		if !bytes.Equal(afterClose[left.ClientID()], leftState) {
			t.Fatalf("left awareness after close = %s, want %s", afterClose[left.ClientID()], leftState)
		}
		if _, ok := afterClose[right.ClientID()]; ok {
			t.Fatalf("right awareness still present after close: %#v", afterClose)
		}
	})

	t.Run("persist explicit using memory store restores snapshot after reopen", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		key := storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-persist"}
		store := memory.New()
		update := buildGCOnlyUpdate(29, 4)

		provider := NewProvider(ProviderConfig{Store: store})
		first, err := provider.Open(ctx, key, "conn-a", 501)
		if err != nil {
			t.Fatalf("provider.Open(conn-a) unexpected error: %v", err)
		}

		dispatch, err := first.HandleEncodedMessages(EncodeProtocolSyncUpdate(update))
		if err != nil {
			t.Fatalf("first.HandleEncodedMessages(sync-update) unexpected error: %v", err)
		}
		if len(dispatch.Broadcast) == 0 {
			t.Fatal("sync update broadcast is empty, want encoded outbound update")
		}

		record, err := first.Persist(ctx)
		if err != nil {
			t.Fatalf("first.Persist() unexpected error: %v", err)
		}
		if record == nil || record.Snapshot == nil {
			t.Fatalf("first.Persist() = %#v, want persisted snapshot", record)
		}
		if record.Through != 1 {
			t.Fatalf("first.Persist().Through = %d, want 1", record.Through)
		}

		expectedSnapshot, err := yjsbridge.PersistedSnapshotFromUpdate(update)
		if err != nil {
			t.Fatalf("PersistedSnapshotFromUpdate() unexpected error: %v", err)
		}
		if !bytes.Equal(record.Snapshot.UpdateV1, expectedSnapshot.UpdateV1) {
			t.Fatalf("record.Snapshot.UpdateV1 = %v, want %v", record.Snapshot.UpdateV1, expectedSnapshot.UpdateV1)
		}
		loaded, err := store.LoadSnapshot(ctx, key)
		if err != nil {
			t.Fatalf("store.LoadSnapshot() unexpected error: %v", err)
		}
		if loaded.Through != 1 {
			t.Fatalf("store.LoadSnapshot().Through = %d, want 1", loaded.Through)
		}

		if _, err := first.Close(); err != nil {
			t.Fatalf("first.Close() unexpected error: %v", err)
		}

		reopenedProvider := NewProvider(ProviderConfig{Store: store})
		reopened, err := reopenedProvider.Open(ctx, key, "conn-b", 502)
		if err != nil {
			t.Fatalf("reopenedProvider.Open(conn-b) unexpected error: %v", err)
		}

		handshake, err := reopened.HandleEncodedMessages(EncodeProtocolSyncStep1([]byte{0x00}))
		if err != nil {
			t.Fatalf("reopened.HandleEncodedMessages(step1) unexpected error: %v", err)
		}
		handshakeMessages, err := DecodeProtocolMessages(handshake.Direct)
		if err != nil {
			t.Fatalf("DecodeProtocolMessages(reopened direct) unexpected error: %v", err)
		}
		if len(handshakeMessages) != 1 || handshakeMessages[0].Sync == nil {
			t.Fatalf("handshakeMessages = %#v, want single sync step2", handshakeMessages)
		}
		if handshakeMessages[0].Sync.Type != SyncMessageTypeStep2 {
			t.Fatalf("reopened sync type = %v, want %v", handshakeMessages[0].Sync.Type, SyncMessageTypeStep2)
		}

		expectedStep2, err := yjsbridge.DiffUpdate(expectedSnapshot.UpdateV1, []byte{0x00})
		if err != nil {
			t.Fatalf("DiffUpdate() unexpected error: %v", err)
		}
		if !bytes.Equal(handshakeMessages[0].Sync.Payload, expectedStep2) {
			t.Fatalf("reopened sync payload = %v, want %v", handshakeMessages[0].Sync.Payload, expectedStep2)
		}
	})
}

func awarenessStatesByClient(update *yawareness.Update) map[uint32][]byte {
	if update == nil {
		return map[uint32][]byte{}
	}

	clientIDs := make([]int, 0, len(update.Clients))
	for _, client := range update.Clients {
		clientIDs = append(clientIDs, int(client.ClientID))
	}
	sort.Ints(clientIDs)

	out := make(map[uint32][]byte, len(update.Clients))
	for _, clientID := range clientIDs {
		for _, client := range update.Clients {
			if client.ClientID != uint32(clientID) {
				continue
			}
			out[client.ClientID] = append([]byte(nil), client.State...)
			break
		}
	}
	return out
}
