package integration

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	"yjs-go-bridge/pkg/ycluster"
	"yjs-go-bridge/pkg/yprotocol"
)

func TestDistributedOwnerFailoverRecoversSnapshotTailAndEpoch(t *testing.T) {
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
		DocumentID: "distributed-owner-failover-recovery",
	}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}
	if _, err := store.SavePlacement(ctx, storage.PlacementRecord{
		Key:     key,
		ShardID: ycluster.StorageShardID(shardID),
		Version: 7,
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

	now := time.Now().UTC()
	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID: ycluster.StorageShardID(shardID),
		Owner: storage.OwnerInfo{
			NodeID: ycluster.StorageNodeID("node-a"),
			Epoch:  1,
		},
		Token:      "lease-node-a-epoch-1",
		AcquiredAt: now.Add(-15 * time.Second),
		ExpiresAt:  now.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("SaveLease(node-a active) unexpected error: %v", err)
	}

	initialResolution, err := nodeALookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner(node-a initial) unexpected error: %v", err)
	}
	if !initialResolution.Local {
		t.Fatal("LookupOwner(node-a initial).Local = false, want true")
	}
	if initialResolution.Placement.NodeID != "node-a" {
		t.Fatalf("LookupOwner(node-a initial).Placement.NodeID = %q, want %q", initialResolution.Placement.NodeID, "node-a")
	}
	if initialResolution.Placement.Lease == nil || initialResolution.Placement.Lease.Epoch != 1 {
		t.Fatalf("LookupOwner(node-a initial).Placement.Lease = %#v, want epoch 1", initialResolution.Placement.Lease)
	}

	baseUpdates := [][]byte{
		mustDecodeHex(t, "01020100040103646f630161030103646f6302112200"),
		mustDecodeHex(t, "01020300040103646f630162030103646f63013300"),
	}
	tailUpdates := [][]byte{
		buildIntegrationGCOnlyUpdate(19, 2),
		buildIntegrationGCOnlyUpdate(41, 1),
	}
	expected := mustMergeUpdates(t, append(append([][]byte(nil), baseUpdates...), tailUpdates...)...)

	ownerAProvider := yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store})
	ownerAConn, err := ownerAProvider.Open(ctx, key, "owner-a", 1001)
	if err != nil {
		t.Fatalf("provider.Open(owner-a) unexpected error: %v", err)
	}

	for _, update := range baseUpdates {
		if _, err := ownerAConn.HandleEncodedMessages(yprotocol.EncodeProtocolSyncUpdate(update)); err != nil {
			t.Fatalf("ownerAConn.HandleEncodedMessages(base) unexpected error: %v", err)
		}
	}

	checkpointRecord, err := ownerAConn.Persist(ctx)
	if err != nil {
		t.Fatalf("ownerAConn.Persist(checkpoint) unexpected error: %v", err)
	}
	checkpointSnapshot := mustPersistedSnapshotFromUpdates(t, baseUpdates...)
	if checkpointRecord == nil || checkpointRecord.Snapshot == nil {
		t.Fatalf("ownerAConn.Persist(checkpoint) = %#v, want persisted snapshot", checkpointRecord)
	}
	if !bytes.Equal(checkpointRecord.Snapshot.UpdateV1, checkpointSnapshot.UpdateV1) {
		t.Fatalf("checkpointRecord.Snapshot.UpdateV1 = %x, want %x", checkpointRecord.Snapshot.UpdateV1, checkpointSnapshot.UpdateV1)
	}

	for _, update := range tailUpdates {
		if _, err := ownerAConn.HandleEncodedMessages(yprotocol.EncodeProtocolSyncUpdate(update)); err != nil {
			t.Fatalf("ownerAConn.HandleEncodedMessages(tail) unexpected error: %v", err)
		}
	}
	if _, err := ownerAConn.Close(); err != nil {
		t.Fatalf("ownerAConn.Close() unexpected error: %v", err)
	}

	if _, err := store.SaveLease(ctx, storage.LeaseRecord{
		ShardID: ycluster.StorageShardID(shardID),
		Owner: storage.OwnerInfo{
			NodeID: ycluster.StorageNodeID("node-a"),
			Epoch:  1,
		},
		Token:      "lease-node-a-epoch-1",
		AcquiredAt: now.Add(-5 * time.Minute),
		ExpiresAt:  now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SaveLease(node-a expired) unexpected error: %v", err)
	}
	if _, err := nodeALookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ycluster.ErrLeaseExpired) {
		t.Fatalf("LookupOwner(node-a expired) error = %v, want %v", err, ycluster.ErrLeaseExpired)
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

	persistedLease, err := store.LoadLease(ctx, ycluster.StorageShardID(shardID))
	if err != nil {
		t.Fatalf("LoadLease(node-b) unexpected error: %v", err)
	}
	if persistedLease.Owner.NodeID != ycluster.StorageNodeID("node-b") || persistedLease.Owner.Epoch != 2 {
		t.Fatalf("LoadLease(node-b) = %#v, want owner=node-b epoch=2", persistedLease)
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
	if afterHandoff.Placement.Lease == nil || afterHandoff.Placement.Lease.Epoch != 2 {
		t.Fatalf("LookupOwner(node-a after handoff).Placement.Lease = %#v, want epoch 2", afterHandoff.Placement.Lease)
	}
	if afterHandoff.Placement.Lease.Token != acquiredB.Token {
		t.Fatalf("LookupOwner(node-a after handoff).Placement.Lease.Token = %q, want %q", afterHandoff.Placement.Lease.Token, acquiredB.Token)
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

	ownerBProvider := yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store})
	ownerBConn, err := ownerBProvider.Open(ctx, key, "owner-b", 1002)
	if err != nil {
		t.Fatalf("provider.Open(owner-b) unexpected error: %v", err)
	}

	reply, err := ownerBConn.HandleEncodedMessages(yprotocol.EncodeProtocolSyncStep1([]byte{0x00}))
	if err != nil {
		t.Fatalf("ownerBConn.HandleEncodedMessages(step1) unexpected error: %v", err)
	}
	assertSyncStep2MatchesUpdate(t, reply.Direct, expected)

	record, err := ownerBConn.Persist(ctx)
	if err != nil {
		t.Fatalf("ownerBConn.Persist() unexpected error: %v", err)
	}
	if record == nil || record.Snapshot == nil {
		t.Fatalf("ownerBConn.Persist() = %#v, want persisted snapshot", record)
	}
	if !bytes.Equal(record.Snapshot.UpdateV1, expected) {
		t.Fatalf("record.Snapshot.UpdateV1 = %x, want %x", record.Snapshot.UpdateV1, expected)
	}

	trimmed, err := store.ListUpdates(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("ListUpdates(after compaction) unexpected error: %v", err)
	}
	if len(trimmed) != 0 {
		t.Fatalf("len(ListUpdates(after compaction)) = %d, want 0", len(trimmed))
	}
}
