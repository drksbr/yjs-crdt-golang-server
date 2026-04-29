package integration

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/internal/varint"
	"github.com/drksbr/yjs-crdt-golang-server/internal/ytypes"
	"github.com/drksbr/yjs-crdt-golang-server/internal/yupdate"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/ycluster"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yjsbridge"
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
	if _, err := store.SaveSnapshotCheckpoint(ctx, key, checkpointSnapshot, checkpointOffset); err != nil {
		t.Fatalf("SaveSnapshotCheckpoint() unexpected error: %v", err)
	}
	if err := store.TrimUpdates(ctx, key, checkpointOffset); err != nil {
		t.Fatalf("TrimUpdates() unexpected error: %v", err)
	}

	recovered, err := storage.RecoverSnapshot(ctx, store, store, key, 0, 1)
	if err != nil {
		t.Fatalf("RecoverSnapshot() unexpected error: %v", err)
	}
	if recovered.CheckpointThrough != checkpointOffset {
		t.Fatalf("recovered.CheckpointThrough = %d, want %d", recovered.CheckpointThrough, checkpointOffset)
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
	if _, err := store.SaveSnapshotCheckpoint(ctx, leftKey, leftSnapshot, leftOffsets[0]); err != nil {
		t.Fatalf("SaveSnapshotCheckpoint(left) unexpected error: %v", err)
	}

	leftRecovered, err := storage.RecoverSnapshot(ctx, store, store, leftKey, 0, 1)
	if err != nil {
		t.Fatalf("RecoverSnapshot(left) unexpected error: %v", err)
	}
	if leftRecovered.CheckpointThrough != leftOffsets[0] {
		t.Fatalf("leftRecovered.CheckpointThrough = %d, want %d", leftRecovered.CheckpointThrough, leftOffsets[0])
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
		TTL:     2 * time.Minute,
		Token:   "lease-node-a",
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

	acquiredB, err := leases.HandoffLease(ctx, *acquiredA, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  "node-b",
		TTL:     2 * time.Minute,
		Token:   "lease-node-b",
	})
	if err != nil {
		t.Fatalf("HandoffLease(node-b) unexpected error: %v", err)
	}
	if acquiredB.Epoch != 2 {
		t.Fatalf("HandoffLease(node-b).Epoch = %d, want 2", acquiredB.Epoch)
	}
	if _, err := leases.HandoffLease(ctx, *acquiredA, ycluster.LeaseRequest{
		ShardID: shardID,
		Holder:  "node-c",
		TTL:     time.Minute,
		Token:   "lease-node-c",
	}); !errors.Is(err, ycluster.ErrLeaseTokenMismatch) {
		t.Fatalf("HandoffLease(stale node-a lease) error = %v, want %v", err, ycluster.ErrLeaseTokenMismatch)
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

func TestLeaseManagerRenewsAndReleasesStorageBackedOwnership(t *testing.T) {
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
		DocumentID: "cluster-lease-manager-renew",
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

	manager, err := ycluster.NewLeaseManager(ycluster.LeaseManagerConfig{
		Store:   leases,
		ShardID: shardID,
		Holder:  "node-a",
		TTL:     120 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewLeaseManager() unexpected error: %v", err)
	}
	lookup, err := ycluster.NewStorageOwnerLookup("node-a", resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup(node-a) unexpected error: %v", err)
	}

	first, changed, err := manager.Ensure(ctx, 40*time.Millisecond)
	if err != nil {
		t.Fatalf("manager.Ensure(initial) unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("manager.Ensure(initial).changed = false, want true")
	}
	if first.Epoch != 1 {
		t.Fatalf("manager.Ensure(initial).Epoch = %d, want 1", first.Epoch)
	}

	time.Sleep(90 * time.Millisecond)
	renewed, changed, err := manager.Ensure(ctx, 40*time.Millisecond)
	if err != nil {
		t.Fatalf("manager.Ensure(renew) unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("manager.Ensure(renew).changed = false, want true")
	}
	if renewed.Token != first.Token {
		t.Fatalf("manager.Ensure(renew).Token = %q, want %q", renewed.Token, first.Token)
	}
	if renewed.Epoch != first.Epoch {
		t.Fatalf("manager.Ensure(renew).Epoch = %d, want %d", renewed.Epoch, first.Epoch)
	}

	wait := time.Until(first.ExpiresAt) + 20*time.Millisecond
	if wait > 0 {
		time.Sleep(wait)
	}
	resolution, err := lookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner(after renew) unexpected error: %v", err)
	}
	if !resolution.Local {
		t.Fatal("LookupOwner(after renew).Local = false, want true")
	}
	if resolution.Placement.Lease == nil || resolution.Placement.Lease.Token != renewed.Token || resolution.Placement.Lease.Epoch != renewed.Epoch {
		t.Fatalf("LookupOwner(after renew).Placement.Lease = %#v, want token %q epoch %d", resolution.Placement.Lease, renewed.Token, renewed.Epoch)
	}

	if err := manager.Release(ctx); err != nil {
		t.Fatalf("manager.Release() unexpected error: %v", err)
	}
	if _, err := lookup.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ycluster.ErrOwnerNotFound) {
		t.Fatalf("LookupOwner(after release) error = %v, want %v", err, ycluster.ErrOwnerNotFound)
	}
}

func TestStorageOwnershipCoordinatorClaimsAdoptsAndHandsOffDocumentOwnership(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "coordinator-ownership",
	}

	nodeA, err := ycluster.NewStorageOwnershipCoordinator(ycluster.StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-a) unexpected error: %v", err)
	}
	first, err := nodeA.ClaimDocument(ctx, ycluster.ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "node-a-token",
		PlacementVersion: 1,
	})
	if err != nil {
		t.Fatalf("ClaimDocument(node-a) unexpected error: %v", err)
	}
	if first.Lease == nil || first.Lease.Holder != "node-a" || first.Lease.Epoch != 1 {
		t.Fatalf("ClaimDocument(node-a).Lease = %#v, want node-a epoch 1", first.Lease)
	}

	restartedNodeA, err := ycluster.NewStorageOwnershipCoordinator(ycluster.StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(restarted node-a) unexpected error: %v", err)
	}
	adopted, err := restartedNodeA.ClaimDocument(ctx, ycluster.ClaimDocumentRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("ClaimDocument(restarted node-a) unexpected error: %v", err)
	}
	if adopted.Lease.Token != first.Lease.Token || adopted.Lease.Epoch != first.Lease.Epoch {
		t.Fatalf("ClaimDocument(restarted node-a).Lease = %#v, want token %q epoch %d", adopted.Lease, first.Lease.Token, first.Lease.Epoch)
	}

	fence, err := restartedNodeA.ResolveAuthorityFence(ctx, key)
	if err != nil {
		t.Fatalf("ResolveAuthorityFence(node-a) unexpected error: %v", err)
	}
	if fence.Owner.NodeID != storage.NodeID("node-a") || fence.Owner.Epoch != first.Lease.Epoch || fence.Token != first.Lease.Token {
		t.Fatalf("ResolveAuthorityFence(node-a) = %#v, want node-a epoch %d token %q", fence, first.Lease.Epoch, first.Lease.Token)
	}

	nodeB, err := ycluster.NewStorageOwnershipCoordinator(ycluster.StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-b",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-b) unexpected error: %v", err)
	}
	if _, err := nodeB.ClaimDocument(ctx, ycluster.ClaimDocumentRequest{DocumentKey: key}); !errors.Is(err, ycluster.ErrLeaseHeld) {
		t.Fatalf("ClaimDocument(node-b while held) error = %v, want %v", err, ycluster.ErrLeaseHeld)
	}

	handoff, err := nodeB.HandoffDocument(ctx, ycluster.HandoffDocumentRequest{
		DocumentKey: key,
		Current:     *adopted.Lease,
		NextHolder:  "node-b",
		TTL:         time.Minute,
		Token:       "node-b-token",
	})
	if err != nil {
		t.Fatalf("HandoffDocument(node-b) unexpected error: %v", err)
	}
	if handoff.Lease == nil || handoff.Lease.Holder != "node-b" || handoff.Lease.Epoch != first.Lease.Epoch+1 {
		t.Fatalf("HandoffDocument(node-b).Lease = %#v, want node-b epoch %d", handoff.Lease, first.Lease.Epoch+1)
	}
	if handoff.Placement == nil || handoff.Placement.Version != 1 {
		t.Fatalf("HandoffDocument(node-b).Placement = %#v, want version 1", handoff.Placement)
	}

	nodeAView, err := restartedNodeA.LookupOwner(ctx, ycluster.OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner(node-a after handoff) unexpected error: %v", err)
	}
	if nodeAView.Local {
		t.Fatal("LookupOwner(node-a after handoff).Local = true, want false")
	}
	if nodeAView.Placement.NodeID != "node-b" || nodeAView.Placement.Lease == nil || nodeAView.Placement.Lease.Token != "node-b-token" {
		t.Fatalf("LookupOwner(node-a after handoff).Placement = %#v, want node-b/node-b-token", nodeAView.Placement)
	}
}

func TestAuthoritativeCompactionCheckpointsEpochAndRejectsStaleFence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	resolver, err := ycluster.NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	key := storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: "authoritative-compaction",
	}

	nodeA, err := ycluster.NewStorageOwnershipCoordinator(ycluster.StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-a",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-a) unexpected error: %v", err)
	}
	ownedA, err := nodeA.ClaimDocument(ctx, ycluster.ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "lease-node-a",
		PlacementVersion: 1,
	})
	if err != nil {
		t.Fatalf("ClaimDocument(node-a) unexpected error: %v", err)
	}
	fenceA, err := nodeA.ResolveAuthorityFence(ctx, key)
	if err != nil {
		t.Fatalf("ResolveAuthorityFence(node-a) unexpected error: %v", err)
	}

	firstUpdate := buildIntegrationGCOnlyUpdate(91, 1)
	secondUpdate := buildIntegrationGCOnlyUpdate(92, 2)
	if _, err := store.AppendUpdateAuthoritative(ctx, key, firstUpdate, *fenceA); err != nil {
		t.Fatalf("AppendUpdateAuthoritative(first) unexpected error: %v", err)
	}
	secondRecord, err := store.AppendUpdateAuthoritative(ctx, key, secondUpdate, *fenceA)
	if err != nil {
		t.Fatalf("AppendUpdateAuthoritative(second) unexpected error: %v", err)
	}

	compactedA, err := storage.CompactUpdateLogAuthoritativeContext(ctx, store, key, nil, 0, 1, *fenceA)
	if err != nil {
		t.Fatalf("CompactUpdateLogAuthoritativeContext(node-a) unexpected error: %v", err)
	}
	if compactedA.Record == nil {
		t.Fatal("CompactUpdateLogAuthoritativeContext(node-a).Record = nil, want checkpoint")
	}
	if compactedA.Record.Through != secondRecord.Offset {
		t.Fatalf("checkpoint Through = %d, want %d", compactedA.Record.Through, secondRecord.Offset)
	}
	if compactedA.Record.Epoch != fenceA.Owner.Epoch {
		t.Fatalf("checkpoint Epoch = %d, want %d", compactedA.Record.Epoch, fenceA.Owner.Epoch)
	}
	expectedSnapshot := mustPersistedSnapshotFromUpdates(t, firstUpdate, secondUpdate)
	if !bytes.Equal(compactedA.Record.Snapshot.UpdateV1, expectedSnapshot.UpdateV1) {
		t.Fatalf("checkpoint snapshot = %x, want %x", compactedA.Record.Snapshot.UpdateV1, expectedSnapshot.UpdateV1)
	}
	remaining, err := store.ListUpdates(ctx, key, 0, 0)
	if err != nil {
		t.Fatalf("ListUpdates(after compaction) unexpected error: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("len(remaining after compaction) = %d, want 0", len(remaining))
	}

	if err := ownedA.Manager.Release(ctx); err != nil {
		t.Fatalf("node-a release unexpected error: %v", err)
	}
	nodeB, err := ycluster.NewStorageOwnershipCoordinator(ycluster.StorageOwnershipCoordinatorConfig{
		LocalNode:  "node-b",
		Resolver:   resolver,
		Placements: store,
		Leases:     store,
		TTL:        time.Minute,
	})
	if err != nil {
		t.Fatalf("NewStorageOwnershipCoordinator(node-b) unexpected error: %v", err)
	}
	ownedB, err := nodeB.ClaimDocument(ctx, ycluster.ClaimDocumentRequest{
		DocumentKey:      key,
		Token:            "lease-node-b",
		PlacementVersion: 2,
	})
	if err != nil {
		t.Fatalf("ClaimDocument(node-b) unexpected error: %v", err)
	}
	t.Cleanup(func() {
		_ = ownedB.Manager.Release(context.Background())
	})
	fenceB, err := nodeB.ResolveAuthorityFence(ctx, key)
	if err != nil {
		t.Fatalf("ResolveAuthorityFence(node-b) unexpected error: %v", err)
	}
	thirdUpdate := buildIntegrationGCOnlyUpdate(93, 1)
	thirdRecord, err := store.AppendUpdateAuthoritative(ctx, key, thirdUpdate, *fenceB)
	if err != nil {
		t.Fatalf("AppendUpdateAuthoritative(third) unexpected error: %v", err)
	}

	loadedCheckpoint, err := store.LoadSnapshot(ctx, key)
	if err != nil {
		t.Fatalf("LoadSnapshot() unexpected error: %v", err)
	}
	if _, err := storage.CompactUpdateLogAuthoritativeContext(ctx, store, key, loadedCheckpoint.Snapshot, loadedCheckpoint.Through, 1, *fenceA); !errors.Is(err, storage.ErrAuthorityLost) {
		t.Fatalf("CompactUpdateLogAuthoritativeContext(stale fence) error = %v, want %v", err, storage.ErrAuthorityLost)
	}

	compactedB, err := storage.CompactUpdateLogAuthoritativeContext(ctx, store, key, loadedCheckpoint.Snapshot, loadedCheckpoint.Through, 1, *fenceB)
	if err != nil {
		t.Fatalf("CompactUpdateLogAuthoritativeContext(node-b) unexpected error: %v", err)
	}
	if compactedB.Record == nil {
		t.Fatal("CompactUpdateLogAuthoritativeContext(node-b).Record = nil, want checkpoint")
	}
	if compactedB.Record.Through != thirdRecord.Offset {
		t.Fatalf("node-b checkpoint Through = %d, want %d", compactedB.Record.Through, thirdRecord.Offset)
	}
	if compactedB.Record.Epoch != fenceB.Owner.Epoch {
		t.Fatalf("node-b checkpoint Epoch = %d, want %d", compactedB.Record.Epoch, fenceB.Owner.Epoch)
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
