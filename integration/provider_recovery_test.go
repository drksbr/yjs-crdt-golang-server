package integration

import (
	"bytes"
	"context"
	"testing"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/yjsbridge"
	"yjs-go-bridge/pkg/yprotocol"
)

func TestProviderRecoveryLoadsRecoveredSnapshotFromCheckpointAndTail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "provider-recovery-checkpoint-tail",
	}

	updates := [][]byte{
		mustDecodeHex(t, "01020100040103646f630161030103646f6302112200"),
		mustDecodeHex(t, "01020300040103646f630162030103646f63013300"),
		buildIntegrationGCOnlyUpdate(19, 2),
		buildIntegrationGCOnlyUpdate(41, 1),
	}
	expected := mustMergeUpdates(t, updates...)

	offsets := appendUpdates(t, ctx, store, key, updates...)
	checkpoint := mustPersistedSnapshotFromUpdates(t, updates[:2]...)
	if _, err := store.SaveSnapshotCheckpoint(ctx, key, checkpoint, offsets[1]); err != nil {
		t.Fatalf("SaveSnapshotCheckpoint(checkpoint) unexpected error: %v", err)
	}
	if err := store.TrimUpdates(ctx, key, offsets[1]); err != nil {
		t.Fatalf("TrimUpdates(checkpoint) unexpected error: %v", err)
	}

	recovered, err := storage.RecoverSnapshot(ctx, store, store, key, 0, 1)
	if err != nil {
		t.Fatalf("RecoverSnapshot() unexpected error: %v", err)
	}
	if recovered.CheckpointThrough != offsets[1] {
		t.Fatalf("recovered.CheckpointThrough = %d, want %d", recovered.CheckpointThrough, offsets[1])
	}
	if len(recovered.Updates) != 2 {
		t.Fatalf("len(recovered.Updates) = %d, want 2", len(recovered.Updates))
	}
	if recovered.LastOffset != offsets[3] {
		t.Fatalf("recovered.LastOffset = %d, want %d", recovered.LastOffset, offsets[3])
	}
	if !bytes.Equal(recovered.Snapshot.UpdateV1, expected) {
		t.Fatalf("recovered.Snapshot.UpdateV1 = %x, want %x", recovered.Snapshot.UpdateV1, expected)
	}

	if _, err := store.SaveSnapshot(ctx, key, recovered.Snapshot); err != nil {
		t.Fatalf("SaveSnapshot(recovered) unexpected error: %v", err)
	}

	provider := yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store})
	connection, err := provider.Open(ctx, key, "recovered-conn", 901)
	if err != nil {
		t.Fatalf("provider.Open(recovered-conn) unexpected error: %v", err)
	}

	handshake, err := connection.HandleEncodedMessages(yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	if err != nil {
		t.Fatalf("connection.HandleEncodedMessages(step1) unexpected error: %v", err)
	}
	assertSyncStep2MatchesUpdate(t, handshake.Direct, expected)

	record, err := connection.Persist(ctx)
	if err != nil {
		t.Fatalf("connection.Persist() unexpected error: %v", err)
	}
	if record == nil || record.Snapshot == nil {
		t.Fatalf("connection.Persist() = %#v, want persisted snapshot", record)
	}
	if !bytes.Equal(record.Snapshot.UpdateV1, expected) {
		t.Fatalf("record.Snapshot.UpdateV1 = %x, want %x", record.Snapshot.UpdateV1, expected)
	}
	if record.Through != offsets[3] {
		t.Fatalf("record.Through = %d, want %d", record.Through, offsets[3])
	}
}

func assertSyncStep2MatchesUpdate(t *testing.T, encoded []byte, expectedUpdate []byte) {
	t.Helper()

	messages, err := yprotocol.DecodeProtocolMessages(encoded)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages() unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil {
		t.Fatalf("messages = %#v, want single sync step2", messages)
	}
	if messages[0].Sync.Type != yprotocol.SyncMessageTypeStep2 {
		t.Fatalf("sync type = %v, want %v", messages[0].Sync.Type, yprotocol.SyncMessageTypeStep2)
	}

	expectedStep2, err := yjsbridge.DiffUpdate(expectedUpdate, []byte{0x00})
	if err != nil {
		t.Fatalf("DiffUpdate() unexpected error: %v", err)
	}
	if !bytes.Equal(messages[0].Sync.Payload, expectedStep2) {
		t.Fatalf("sync payload = %x, want %x", messages[0].Sync.Payload, expectedStep2)
	}
}
