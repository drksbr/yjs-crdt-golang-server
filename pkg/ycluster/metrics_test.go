package ycluster

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage/memory"
)

func TestControlPlaneMetricsObserveLookupAndLeaseOperations(t *testing.T) {
	t.Parallel()

	store := memory.New()
	recorder := newRecordingMetrics()
	leases := mustStorageLeaseStore(t, store).WithMetrics(recorder)
	now := time.Unix(7000, 0).UTC()
	leases.now = func() time.Time { return now }

	resolver, err := NewDeterministicShardResolver(32)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}
	key := storage.DocumentKey{Namespace: "tests", DocumentID: "ycluster-metrics"}
	shardID, err := resolver.ResolveShard(key)
	if err != nil {
		t.Fatalf("ResolveShard() unexpected error: %v", err)
	}
	if _, err := store.SavePlacement(context.Background(), storage.PlacementRecord{
		Key:     key,
		ShardID: StorageShardID(shardID),
		Version: 1,
	}); err != nil {
		t.Fatalf("SavePlacement() unexpected error: %v", err)
	}

	acquired, err := leases.AcquireLease(context.Background(), LeaseRequest{
		ShardID: shardID,
		Holder:  "node-a",
		TTL:     30 * time.Second,
	})
	if err != nil {
		t.Fatalf("AcquireLease() unexpected error: %v", err)
	}
	if _, err := leases.RenewLease(context.Background(), LeaseRequest{
		ShardID: shardID,
		Holder:  "node-a",
		TTL:     time.Minute,
		Token:   acquired.Token,
	}); err != nil {
		t.Fatalf("RenewLease() unexpected error: %v", err)
	}

	lookup, err := NewStorageOwnerLookup("node-a", resolver, store, store)
	if err != nil {
		t.Fatalf("NewStorageOwnerLookup() unexpected error: %v", err)
	}
	lookup.WithMetrics(recorder)
	lookup.now = func() time.Time { return now }

	resolution, err := lookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		t.Fatalf("LookupOwner(local) unexpected error: %v", err)
	}
	if !resolution.Local {
		t.Fatal("LookupOwner(local).Local = false, want true")
	}

	handoff, err := leases.HandoffLease(context.Background(), *acquired, LeaseRequest{
		ShardID: shardID,
		Holder:  "node-b",
		TTL:     time.Minute,
		Token:   "lease-b",
	})
	if err != nil {
		t.Fatalf("HandoffLease() unexpected error: %v", err)
	}

	now = now.Add(2 * time.Minute)
	if _, err := lookup.LookupOwner(context.Background(), OwnerLookupRequest{DocumentKey: key}); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("LookupOwner(expired) error = %v, want %v", err, ErrLeaseExpired)
	}
	if err := leases.ReleaseLease(context.Background(), *handoff); err != nil {
		t.Fatalf("ReleaseLease() unexpected error: %v", err)
	}

	snapshot := recorder.snapshot()
	if snapshot.ownerLookups[ownerLookupResultLocal] != 1 {
		t.Fatalf("ownerLookups[local] = %d, want 1", snapshot.ownerLookups[ownerLookupResultLocal])
	}
	if snapshot.ownerLookups[ownerLookupResultLeaseExpired] != 1 {
		t.Fatalf("ownerLookups[lease_expired] = %d, want 1", snapshot.ownerLookups[ownerLookupResultLeaseExpired])
	}
	if snapshot.leaseOperations[recordingLeaseOperationKey{operation: leaseOperationAcquire, result: metricsResultOK}] != 1 {
		t.Fatalf("leaseOperations(acquire/ok) = %#v, want 1", snapshot.leaseOperations)
	}
	if snapshot.leaseOperations[recordingLeaseOperationKey{operation: leaseOperationRenew, result: metricsResultOK}] != 1 {
		t.Fatalf("leaseOperations(renew/ok) = %#v, want 1", snapshot.leaseOperations)
	}
	if snapshot.leaseOperations[recordingLeaseOperationKey{operation: leaseOperationHandoff, result: metricsResultOK}] != 1 {
		t.Fatalf("leaseOperations(handoff/ok) = %#v, want 1", snapshot.leaseOperations)
	}
	if snapshot.leaseOperations[recordingLeaseOperationKey{operation: leaseOperationRelease, result: metricsResultOK}] != 1 {
		t.Fatalf("leaseOperations(release/ok) = %#v, want 1", snapshot.leaseOperations)
	}
}

func TestLeaseManagerMetricsObserveActions(t *testing.T) {
	t.Parallel()

	store := memory.New()
	leases := mustStorageLeaseStore(t, store)
	now := time.Unix(8000, 0).UTC()
	leases.now = func() time.Time { return now }
	recorder := newRecordingMetrics()

	manager, err := NewLeaseManager(LeaseManagerConfig{
		Store:   leases,
		ShardID: 17,
		Holder:  "node-a",
		TTL:     20 * time.Second,
		Metrics: recorder,
	})
	if err != nil {
		t.Fatalf("NewLeaseManager() unexpected error: %v", err)
	}
	manager.now = leases.now

	if _, _, err := manager.Ensure(context.Background(), 5*time.Second); err != nil {
		t.Fatalf("Ensure(acquire) unexpected error: %v", err)
	}
	if _, _, err := manager.Ensure(context.Background(), 5*time.Second); err != nil {
		t.Fatalf("Ensure(noop) unexpected error: %v", err)
	}

	now = now.Add(16 * time.Second)
	if _, _, err := manager.Ensure(context.Background(), 5*time.Second); err != nil {
		t.Fatalf("Ensure(renew) unexpected error: %v", err)
	}

	now = now.Add(21 * time.Second)
	if _, _, err := manager.Ensure(context.Background(), 5*time.Second); err != nil {
		t.Fatalf("Ensure(reacquire) unexpected error: %v", err)
	}
	if err := manager.Release(context.Background()); err != nil {
		t.Fatalf("Release() unexpected error: %v", err)
	}

	snapshot := recorder.snapshot()
	if snapshot.leaseManagerActions[recordingLeaseManagerActionKey{action: leaseManagerActionAcquire, result: metricsResultOK}] != 1 {
		t.Fatalf("leaseManagerActions(acquire/ok) = %#v, want 1", snapshot.leaseManagerActions)
	}
	if snapshot.leaseManagerActions[recordingLeaseManagerActionKey{action: leaseManagerActionNoop, result: metricsResultOK}] != 1 {
		t.Fatalf("leaseManagerActions(noop/ok) = %#v, want 1", snapshot.leaseManagerActions)
	}
	if snapshot.leaseManagerActions[recordingLeaseManagerActionKey{action: leaseManagerActionRenew, result: metricsResultOK}] != 1 {
		t.Fatalf("leaseManagerActions(renew/ok) = %#v, want 1", snapshot.leaseManagerActions)
	}
	if snapshot.leaseManagerActions[recordingLeaseManagerActionKey{action: leaseManagerActionReacquire, result: metricsResultOK}] != 1 {
		t.Fatalf("leaseManagerActions(reacquire/ok) = %#v, want 1", snapshot.leaseManagerActions)
	}
	if snapshot.leaseManagerActions[recordingLeaseManagerActionKey{action: leaseManagerActionRelease, result: metricsResultOK}] != 1 {
		t.Fatalf("leaseManagerActions(release/ok) = %#v, want 1", snapshot.leaseManagerActions)
	}
}

type recordingMetrics struct {
	mu                  sync.Mutex
	ownerLookups        map[string]int
	leaseOperations     map[recordingLeaseOperationKey]int
	leaseManagerActions map[recordingLeaseManagerActionKey]int
}

type recordingMetricsSnapshot struct {
	ownerLookups        map[string]int
	leaseOperations     map[recordingLeaseOperationKey]int
	leaseManagerActions map[recordingLeaseManagerActionKey]int
}

type recordingLeaseOperationKey struct {
	operation string
	result    string
}

type recordingLeaseManagerActionKey struct {
	action string
	result string
}

func newRecordingMetrics() *recordingMetrics {
	return &recordingMetrics{
		ownerLookups:        make(map[string]int),
		leaseOperations:     make(map[recordingLeaseOperationKey]int),
		leaseManagerActions: make(map[recordingLeaseManagerActionKey]int),
	}
}

func (r *recordingMetrics) OwnerLookup(_ time.Duration, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ownerLookups[result]++
}

func (r *recordingMetrics) LeaseOperation(_ ShardID, operation string, _ time.Duration, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.leaseOperations[recordingLeaseOperationKey{
		operation: operation,
		result:    result,
	}]++
}

func (r *recordingMetrics) LeaseManagerAction(_ ShardID, action string, _ time.Duration, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.leaseManagerActions[recordingLeaseManagerActionKey{
		action: action,
		result: result,
	}]++
}

func (r *recordingMetrics) snapshot() recordingMetricsSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	ownerLookups := make(map[string]int, len(r.ownerLookups))
	for key, total := range r.ownerLookups {
		ownerLookups[key] = total
	}
	leaseOperations := make(map[recordingLeaseOperationKey]int, len(r.leaseOperations))
	for key, total := range r.leaseOperations {
		leaseOperations[key] = total
	}
	leaseManagerActions := make(map[recordingLeaseManagerActionKey]int, len(r.leaseManagerActions))
	for key, total := range r.leaseManagerActions {
		leaseManagerActions[key] = total
	}

	return recordingMetricsSnapshot{
		ownerLookups:        ownerLookups,
		leaseOperations:     leaseOperations,
		leaseManagerActions: leaseManagerActions,
	}
}
