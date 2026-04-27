package integration

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
	"time"

	"yjs-go-bridge/internal/varint"
	"yjs-go-bridge/internal/ytypes"
	"yjs-go-bridge/internal/yupdate"
	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/ycluster"
	"yjs-go-bridge/pkg/yjsbridge"
)

func TestDistributedRecoveryReplaysTrimmedTailFromCheckpoint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "distributed-recovery-tail",
	}

	updates := [][]byte{
		mustDecodeHex(t, "01020100040103646f630161030103646f6302112200"),
		mustDecodeHex(t, "01020300040103646f630162030103646f63013300"),
		buildIntegrationGCOnlyUpdate(19, 2),
		buildIntegrationGCOnlyUpdate(41, 1),
	}
	expected := mustMergeUpdates(t, updates...)

	offsets := appendUpdates(t, ctx, store, key, updates...)
	checkpointOffset := offsets[1]

	checkpointSnapshot := mustPersistedSnapshotFromUpdates(t, updates[:2]...)
	if _, err := store.SaveSnapshot(ctx, key, checkpointSnapshot); err != nil {
		t.Fatalf("SaveSnapshot() unexpected error: %v", err)
	}
	if err := store.TrimUpdates(ctx, key, checkpointOffset); err != nil {
		t.Fatalf("TrimUpdates() unexpected error: %v", err)
	}

	recovered, err := storage.RecoverSnapshot(ctx, store, store, key, checkpointOffset, 1)
	if err != nil {
		t.Fatalf("RecoverSnapshot() unexpected error: %v", err)
	}
	if !bytes.Equal(recovered.Snapshot.UpdateV1, expected) {
		t.Fatalf("recovered.Snapshot.UpdateV1 = %x, want %x", recovered.Snapshot.UpdateV1, expected)
	}
	if recovered.LastOffset != offsets[3] {
		t.Fatalf("recovered.LastOffset = %d, want %d", recovered.LastOffset, offsets[3])
	}

	recoveredSV := mustStateVector(t, recovered.Snapshot.UpdateV1)
	expectedSV := mustStateVector(t, expected)
	if !reflect.DeepEqual(recoveredSV, expectedSV) {
		t.Fatalf("recovered state vector = %#v, want %#v", recoveredSV, expectedSV)
	}

	recoveredIDs := mustContentIDsFromUpdate(t, recovered.Snapshot.UpdateV1)
	expectedIDs := mustContentIDsFromUpdate(t, expected)
	if !reflect.DeepEqual(recoveredIDs.InsertRanges(), expectedIDs.InsertRanges()) {
		t.Fatalf("recovered insert ranges = %#v, want %#v", recoveredIDs.InsertRanges(), expectedIDs.InsertRanges())
	}
	if !reflect.DeepEqual(recoveredIDs.DeleteRanges(), expectedIDs.DeleteRanges()) {
		t.Fatalf("recovered delete ranges = %#v, want %#v", recoveredIDs.DeleteRanges(), expectedIDs.DeleteRanges())
	}
}

func TestDistributedRecoverySeparatesDocumentsAcrossSharedStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	leftKey := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "distributed-doc-left",
	}
	rightKey := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "distributed-doc-right",
	}

	leftUpdates := [][]byte{
		mustDecodeHex(t, "01020100040103646f630161030103646f6302112200"),
		buildIntegrationGCOnlyUpdate(77, 2),
	}
	rightUpdates := [][]byte{
		mustDecodeHex(t, "01020300040103646f630162030103646f63013300"),
		buildIntegrationGCOnlyUpdate(88, 1),
	}

	leftExpected := mustMergeUpdates(t, leftUpdates...)
	rightExpected := mustMergeUpdates(t, rightUpdates...)

	leftOffsets := appendUpdates(t, ctx, store, leftKey, leftUpdates[0])
	appendUpdates(t, ctx, store, rightKey, rightUpdates[0])
	appendUpdates(t, ctx, store, leftKey, leftUpdates[1])
	appendUpdates(t, ctx, store, rightKey, rightUpdates[1])

	leftSnapshot := mustPersistedSnapshotFromUpdates(t, leftUpdates[0])
	if _, err := store.SaveSnapshot(ctx, leftKey, leftSnapshot); err != nil {
		t.Fatalf("SaveSnapshot(left) unexpected error: %v", err)
	}

	leftRecovered, err := storage.RecoverSnapshot(ctx, store, store, leftKey, leftOffsets[0], 1)
	if err != nil {
		t.Fatalf("RecoverSnapshot(left) unexpected error: %v", err)
	}
	if !bytes.Equal(leftRecovered.Snapshot.UpdateV1, leftExpected) {
		t.Fatalf("leftRecovered = %x, want %x", leftRecovered.Snapshot.UpdateV1, leftExpected)
	}

	rightRecovered, err := storage.RecoverSnapshot(ctx, store, store, rightKey, 0, 1)
	if err != nil {
		t.Fatalf("RecoverSnapshot(right) unexpected error: %v", err)
	}
	if !bytes.Equal(rightRecovered.Snapshot.UpdateV1, rightExpected) {
		t.Fatalf("rightRecovered = %x, want %x", rightRecovered.Snapshot.UpdateV1, rightExpected)
	}
}

func TestClusterControlPlaneTracksStorageBackedOwnerHandoff(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	leases, err := ycluster.NewStorageLeaseStore(store)
	if err != nil {
		t.Fatalf("NewStorageLeaseStore() unexpected error: %v", err)
	}

	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "cluster-owner-handoff",
	}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}
	if _, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     key,
		ShardID: ycluster.StorageShardID(shardID),
		Version: 1,
	}); err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}

	nodeALookup, err := ycluster.NewStorageOwnerLookup("node-a", resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup(node-a) unexpected error: %v", err)
	}
	nodeBLookup, err := ycluster.NewStorageOwnerLookup("node-b", resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup(node-b) unexpected error: %v", err)
	}

	acquiredA, err := leases.AcquireLease(ctx, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  "node-a",
		TTL:     750 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-a) unexpected error: %v", err)
	}

	nodeAResolution, err := nodeALookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner(node-a) unexpected error: %v", err)
	}
	if !nodeAResolution.Local {
		t.Fatal("LookupOwner(node-a).Local = false, want true")
	}
	if nodeAResolution.Placement.NodeID != "node-a" {
		t.Fatalf("LookupOwner(node-a).Placement.NodeID = %q, want %q", nodeAResolution.Placement.NodeID, "node-a")
	}
	if nodeAResolution.Placement.Lease == nil || nodeAResolution.Placement.Lease.Epoch != 1 {
		t.Fatalf("LookupOwner(node-a).Placement.Lease = %#v, want epoch 1", nodeAResolution.Placement.Lease)
	}

	nodeBResolution, err := nodeBLookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner(node-b) unexpected error: %v", err)
	}
	if nodeBResolution.Local {
		t.Fatal("LookupOwner(node-b).Local = true, want false")
	}

	wait := time.Until(acquiredA.ExpiresAt) + 20*time.Millisecond
	if wait > 0 {
		time.Sleep(wait)
	}
	if _, err := nodeALookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ycluster.ErrLeaseExpired) {
		t.Fatalf("LookupOwner(node-a after expiry) error = %v, want %v", err, ycluster.ErrLeaseExpired)
	}

	acquiredB, err := leases.AcquireLease(ctx, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  "node-b",
		TTL:     2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("AcquireLease(node-b) unexpected error: %v", err)
	}
	if acquiredB.Epoch != 2 {
		t.Fatalf("AcquireLease(node-b).Epoch = %d, want 2", acquiredB.Epoch)
	}

	afterHandoff, err := nodeALookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner(node-a after handoff) unexpected error: %v", err)
	}
	if afterHandoff.Local {
		t.Fatal("LookupOwner(node-a after handoff).Local = true, want false")
	}
	if afterHandoff.Placement.NodeID != "node-b" {
		t.Fatalf("LookupOwner(node-a after handoff).Placement.NodeID = %q, want %q", afterHandoff.Placement.NodeID, "node-b")
	}
	if afterHandoff.Placement.Lease == nil || afterHandoff.Placement.Lease.Token != acquiredB.Token || afterHandoff.Placement.Lease.Epoch != 2 {
		t.Fatalf("LookupOwner(node-a after handoff).Placement.Lease = %#v, want token %q epoch 2", afterHandoff.Placement.Lease, acquiredB.Token)
	}

	nodeBLocal, err := nodeBLookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner(node-b after handoff) unexpected error: %v", err)
	}
	if !nodeBLocal.Local {
		t.Fatal("LookupOwner(node-b after handoff).Local = false, want true")
	}
	if nodeBLocal.Placement.Lease == nil || nodeBLocal.Placement.Lease.Epoch != 2 {
		t.Fatalf("LookupOwner(node-b after handoff).Placement.Lease = %#v, want epoch 2", nodeBLocal.Placement.Lease)
	}
}

func appendUpdates(t *testing.T, ctx context.Context, store storage.UpdateLogStore, key storage.DocumentKey, updates ...[]byte) []storage.UpdateOffset {
	t.Helper()

	offsets := make([]storage.UpdateOffset, 0, len(updates))
	for _, update := range updates {
		record, err := store.AppendUpdate(ctx, key, update)
		if err != nil {
			t.Fatalf("AppendUpdate() unexpected error: %v", err)
		}
		offsets = append(offsets, record.Offset)
	}
	return offsets
}

func mustMergeUpdates(t *testing.T, updates ...[]byte) []byte {
	t.Helper()

	merged, err := yjsbridge.MergeUpdates(updates...)
	if err != nil {
		t.Fatalf("MergeUpdates() unexpected error: %v", err)
	}
	return merged
}

func mustPersistedSnapshotFromUpdates(t *testing.T, updates ...[]byte) *yjsbridge.PersistedSnapshot {
	t.Helper()

	snapshot, err := yjsbridge.PersistedSnapshotFromUpdates(updates...)
	if err != nil {
		t.Fatalf("PersistedSnapshotFromUpdates() unexpected error: %v", err)
	}
	return snapshot
}

func mustStateVector(t *testing.T, update []byte) map[uint32]uint32 {
	t.Helper()

	stateVector, err := yjsbridge.StateVectorFromUpdate(update)
	if err != nil {
		t.Fatalf("StateVectorFromUpdate() unexpected error: %v", err)
	}
	return stateVector
}

func mustContentIDsFromUpdate(t *testing.T, update []byte) *yjsbridge.ContentIDs {
	t.Helper()

	contentIDs, err := yjsbridge.CreateContentIDsFromUpdate(update)
	if err != nil {
		t.Fatalf("CreateContentIDsFromUpdate() unexpected error: %v", err)
	}
	return contentIDs
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("hex.DecodeString(%q) unexpected error: %v", value, err)
	}
	return decoded
}

func buildIntegrationGCOnlyUpdate(client, length uint32) []byte {
	update := varint.Append(nil, 1)
	update = varint.Append(update, 1)
	update = varint.Append(update, client)
	update = varint.Append(update, 0)
	update = append(update, 0)
	update = varint.Append(update, length)
	return append(update, yupdate.EncodeDeleteSetBlockV1(ytypes.NewDeleteSet())...)
}
