package yprotocol

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/yjsbridge"
)

func TestProviderOpenRecoversSnapshotPlusTail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := storage.DocumentKey{
		Namespace:  "tests",
		DocumentID: "provider-recover-snapshot-tail",
	}
	store := memory.New()

	baseUpdate := buildGCOnlyUpdate(11, 1)
	tailUpdate := buildGCOnlyUpdate(22, 2)
	baseSnapshot, err := yjsbridge.PersistedSnapshotFromUpdate(baseUpdate)
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdate(baseUpdate) unexpected error: %v", err)
	}
	if _, err := store.SaveSnapshot(ctx, key, baseSnapshot); err != nil {
		t.Fatalf("SaveSnapshot() unexpected error: %v", err)
	}
	if _, err := store.AppendUpdate(ctx, key, tailUpdate); err != nil {
		t.Fatalf("AppendUpdate() unexpected error: %v", err)
	}

	provider := NewProvider(ProviderConfig{Store: store})
	conn, err := provider.Open(ctx, key, "conn-a", 801)
	if err != nil {
		t.Fatalf("provider.Open() unexpected error: %v", err)
	}

	reply, err := conn.HandleEncodedMessages(EncodeProtocolSyncStep1([]byte{0x00}))
	if err != nil {
		t.Fatalf("conn.HandleEncodedMessages(step1) unexpected error: %v", err)
	}
	messages, err := DecodeProtocolMessages(reply.Direct)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(reply.Direct) unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil {
		t.Fatalf("messages = %#v, want single sync step2", messages)
	}

	expectedUpdate, err := yjsbridge.MergeUpdates(baseUpdate, tailUpdate)
	if err != nil {
		t.Fatalf("MergeUpdates() unexpected error: %v", err)
	}
	expectedStep2, err := yjsbridge.DiffUpdate(expectedUpdate, []byte{0x00})
	if err != nil {
		t.Fatalf("DiffUpdate() unexpected error: %v", err)
	}
	if !bytes.Equal(messages[0].Sync.Payload, expectedStep2) {
		t.Fatalf("messages[0].Sync.Payload = %v, want %v", messages[0].Sync.Payload, expectedStep2)
	}
}

func TestProviderSyncUpdateAppendsLogAndPersistTrimsTail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := storage.DocumentKey{
		Namespace:  "tests",
		DocumentID: "provider-append-log-persist-trim",
	}
	store := memory.New()
	provider := NewProvider(ProviderConfig{Store: store})

	conn, err := provider.Open(ctx, key, "conn-a", 802)
	if err != nil {
		t.Fatalf("provider.Open() unexpected error: %v", err)
	}

	update := buildGCOnlyUpdate(31, 3)
	if _, err := conn.HandleEncodedMessages(EncodeProtocolSyncUpdate(update)); err != nil {
		t.Fatalf("conn.HandleEncodedMessages(sync-update) unexpected error: %v", err)
	}

	records, err := store.ListUpdates(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("store.ListUpdates() unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if !bytes.Equal(records[0].UpdateV1, update) {
		t.Fatalf("records[0].UpdateV1 = %v, want %v", records[0].UpdateV1, update)
	}

	record, err := conn.Persist(ctx)
	if err != nil {
		t.Fatalf("conn.Persist() unexpected error: %v", err)
	}
	if record == nil || record.Snapshot == nil {
		t.Fatalf("conn.Persist() = %#v, want persisted snapshot", record)
	}

	trimmed, err := store.ListUpdates(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("store.ListUpdates() after persist unexpected error: %v", err)
	}
	if len(trimmed) != 0 {
		t.Fatalf("len(trimmed) = %d, want 0", len(trimmed))
	}

	loaded, err := store.LoadSnapshot(ctx, key)
	if err != nil {
		t.Fatalf("store.LoadSnapshot() unexpected error: %v", err)
	}
	expectedSnapshot, err := yjsbridge.PersistedSnapshotFromUpdate(update)
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdate() unexpected error: %v", err)
	}
	if !bytes.Equal(loaded.Snapshot.UpdateV1, expectedSnapshot.UpdateV1) {
		t.Fatalf("loaded.Snapshot.UpdateV1 = %v, want %v", loaded.Snapshot.UpdateV1, expectedSnapshot.UpdateV1)
	}
}

func TestProviderPersistCompactsRecoveredSnapshotPlusTail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := storage.DocumentKey{
		Namespace:  "tests",
		DocumentID: "provider-persist-compacts-recovered-tail",
	}
	store := memory.New()

	baseUpdate := buildGCOnlyUpdate(32, 1)
	tailUpdate := buildGCOnlyUpdate(33, 2)
	baseSnapshot, err := yjsbridge.PersistedSnapshotFromUpdate(baseUpdate)
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdate(baseUpdate) unexpected error: %v", err)
	}
	if _, err := store.SaveSnapshot(ctx, key, baseSnapshot); err != nil {
		t.Fatalf("store.SaveSnapshot() unexpected error: %v", err)
	}
	if _, err := store.AppendUpdate(ctx, key, tailUpdate); err != nil {
		t.Fatalf("store.AppendUpdate() unexpected error: %v", err)
	}

	provider := NewProvider(ProviderConfig{Store: store})
	conn, err := provider.Open(ctx, key, "conn-a", 805)
	if err != nil {
		t.Fatalf("provider.Open() unexpected error: %v", err)
	}

	record, err := conn.Persist(ctx)
	if err != nil {
		t.Fatalf("conn.Persist() unexpected error: %v", err)
	}
	if record == nil || record.Snapshot == nil {
		t.Fatalf("conn.Persist() = %#v, want persisted snapshot", record)
	}

	expectedSnapshot, err := yjsbridge.PersistedSnapshotFromUpdates(baseUpdate, tailUpdate)
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}
	if !bytes.Equal(record.Snapshot.UpdateV1, expectedSnapshot.UpdateV1) {
		t.Fatalf("record.Snapshot.UpdateV1 = %v, want %v", record.Snapshot.UpdateV1, expectedSnapshot.UpdateV1)
	}

	records, err := store.ListUpdates(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("store.ListUpdates() unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("len(records) = %d, want 0", len(records))
	}

	reopened := NewProvider(ProviderConfig{Store: store})
	recovered, err := reopened.Open(ctx, key, "conn-b", 806)
	if err != nil {
		t.Fatalf("reopened.Open() unexpected error: %v", err)
	}

	reply, err := recovered.HandleEncodedMessages(EncodeProtocolSyncStep1([]byte{0x00}))
	if err != nil {
		t.Fatalf("recovered.HandleEncodedMessages(step1) unexpected error: %v", err)
	}
	messages, err := DecodeProtocolMessages(reply.Direct)
	if err != nil {
		t.Fatalf("DecodeProtocolMessages(reply.Direct) unexpected error: %v", err)
	}
	if len(messages) != 1 || messages[0].Sync == nil {
		t.Fatalf("messages = %#v, want single sync step2", messages)
	}

	expectedStep2, err := yjsbridge.DiffUpdate(expectedSnapshot.UpdateV1, []byte{0x00})
	if err != nil {
		t.Fatalf("DiffUpdate() unexpected error: %v", err)
	}
	if !bytes.Equal(messages[0].Sync.Payload, expectedStep2) {
		t.Fatalf("messages[0].Sync.Payload = %v, want %v", messages[0].Sync.Payload, expectedStep2)
	}
}

func TestProviderSyncUpdateDoesNotAdvanceSessionsWhenAppendFails(t *testing.T) {
	t.Parallel()

	appendErr := errors.New("append failed")
	store := failingDistributedStore{appendErr: appendErr}
	provider := NewProvider(ProviderConfig{Store: store})
	key := storage.DocumentKey{
		Namespace:  "tests",
		DocumentID: "provider-append-failure",
	}

	sender, err := provider.Open(context.Background(), key, "conn-a", 803)
	if err != nil {
		t.Fatalf("provider.Open(sender) unexpected error: %v", err)
	}
	peer, err := provider.Open(context.Background(), key, "conn-b", 804)
	if err != nil {
		t.Fatalf("provider.Open(peer) unexpected error: %v", err)
	}

	beforeSender := sender.session.UpdateV1()
	beforePeer := peer.session.UpdateV1()
	update := buildGCOnlyUpdate(41, 2)

	if _, err := sender.HandleEncodedMessages(EncodeProtocolSyncUpdate(update)); !errors.Is(err, appendErr) {
		t.Fatalf("sender.HandleEncodedMessages() error = %v, want %v", err, appendErr)
	}
	if !bytes.Equal(sender.session.UpdateV1(), beforeSender) {
		t.Fatalf("sender.session.UpdateV1() = %v, want %v", sender.session.UpdateV1(), beforeSender)
	}
	if !bytes.Equal(peer.session.UpdateV1(), beforePeer) {
		t.Fatalf("peer.session.UpdateV1() = %v, want %v", peer.session.UpdateV1(), beforePeer)
	}
}

type failingDistributedStore struct {
	appendErr error
}

func (f failingDistributedStore) SaveSnapshot(context.Context, storage.DocumentKey, *yjsbridge.PersistedSnapshot) (*storage.SnapshotRecord, error) {
	return &storage.SnapshotRecord{
		Snapshot: yjsbridge.NewPersistedSnapshot(),
	}, nil
}

func (f failingDistributedStore) LoadSnapshot(context.Context, storage.DocumentKey) (*storage.SnapshotRecord, error) {
	return nil, storage.ErrSnapshotNotFound
}

func (f failingDistributedStore) AppendUpdate(context.Context, storage.DocumentKey, []byte) (*storage.UpdateLogRecord, error) {
	return nil, f.appendErr
}

func (f failingDistributedStore) ListUpdates(context.Context, storage.DocumentKey, storage.UpdateOffset, int) ([]*storage.UpdateLogRecord, error) {
	return nil, nil
}

func (f failingDistributedStore) TrimUpdates(context.Context, storage.DocumentKey, storage.UpdateOffset) error {
	return nil
}
