package ycluster

import (
	"context"
	"errors"
	"fmt"

	"yjs-go-bridge/pkg/storage"
)

// ResolveStorageAuthorityFence resolve o owner atual do documento e converte a
// resolução local em um fence autoritativo de storage.
func ResolveStorageAuthorityFence(ctx context.Context, lookup OwnerLookup, key storage.DocumentKey) (*storage.AuthorityFence, error) {
	if lookup == nil {
		return nil, fmt.Errorf("%w: owner lookup ausente", storage.ErrAuthorityLost)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	resolution, err := lookup.LookupOwner(ctx, OwnerLookupRequest{DocumentKey: key})
	if err != nil {
		if isAuthorityUnavailable(err) {
			return nil, fmt.Errorf("%w: %v", storage.ErrAuthorityLost, err)
		}
		return nil, err
	}
	return AuthorityFenceFromResolution(resolution)
}

// AuthorityFenceFromResolution converte uma resolução local válida em um fence
// de storage reaproveitável pelo runtime autoritativo.
func AuthorityFenceFromResolution(resolution *OwnerResolution) (*storage.AuthorityFence, error) {
	if resolution == nil {
		return nil, fmt.Errorf("%w: owner resolution ausente", storage.ErrAuthorityLost)
	}
	if !resolution.Local {
		return nil, fmt.Errorf("%w: owner remoto %q", storage.ErrAuthorityLost, resolution.Placement.NodeID)
	}
	if err := resolution.Placement.Validate(); err != nil {
		return nil, err
	}
	if resolution.Placement.Lease == nil {
		return nil, fmt.Errorf("%w: lease ausente para shard %s", storage.ErrAuthorityLost, resolution.Placement.ShardID)
	}

	fence := &storage.AuthorityFence{
		ShardID: StorageShardID(resolution.Placement.ShardID),
		Owner: storage.OwnerInfo{
			NodeID: StorageNodeID(resolution.Placement.NodeID),
			Epoch:  resolution.Placement.Lease.Epoch,
		},
		Token: resolution.Placement.Lease.Token,
	}
	if err := fence.Validate(); err != nil {
		return nil, err
	}
	return fence, nil
}

func isAuthorityUnavailable(err error) bool {
	return errors.Is(err, ErrOwnerNotFound) ||
		errors.Is(err, ErrPlacementNotFound) ||
		errors.Is(err, ErrLeaseExpired) ||
		errors.Is(err, ErrInvalidPlacement) ||
		errors.Is(err, ErrInvalidLease)
}
