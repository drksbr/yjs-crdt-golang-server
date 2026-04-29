package ycluster

import (
	"errors"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

func TestNewDeterministicShardResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		shardCount uint32
		wantErr    error
	}{
		{
			name:       "valid",
			shardCount: 64,
			wantErr:    nil,
		},
		{
			name:       "zero_shards",
			shardCount: 0,
			wantErr:    ErrInvalidShardCount,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolver, err := NewDeterministicShardResolver(tt.shardCount)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewDeterministicShardResolver() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if resolver != nil {
					t.Fatalf("NewDeterministicShardResolver() resolver = %#v, want nil", resolver)
				}
				return
			}
			if resolver == nil {
				t.Fatal("NewDeterministicShardResolver() resolver = nil, want non-nil")
			}
			if got := resolver.ShardCount(); got != tt.shardCount {
				t.Fatalf("resolver.ShardCount() = %d, want %d", got, tt.shardCount)
			}
		})
	}
}

func TestDeterministicShardResolverResolveShardDeterministic(t *testing.T) {
	t.Parallel()

	left, err := NewDeterministicShardResolver(128)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver(left) unexpected error: %v", err)
	}
	right, err := NewDeterministicShardResolver(128)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver(right) unexpected error: %v", err)
	}

	tests := []struct {
		name string
		key  storage.DocumentKey
	}{
		{
			name: "namespace_and_document",
			key:  storage.DocumentKey{Namespace: "team-a", DocumentID: "doc-1"},
		},
		{
			name: "document_without_namespace",
			key:  storage.DocumentKey{DocumentID: "doc-2"},
		},
		{
			name: "unicode_safe_bytes",
			key:  storage.DocumentKey{Namespace: "europe", DocumentID: "doc-umlaut"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			first, err := left.ResolveShard(tt.key)
			if err != nil {
				t.Fatalf("left.ResolveShard() unexpected error: %v", err)
			}
			second, err := left.ResolveShard(tt.key)
			if err != nil {
				t.Fatalf("left.ResolveShard() second unexpected error: %v", err)
			}
			third, err := right.ResolveShard(tt.key)
			if err != nil {
				t.Fatalf("right.ResolveShard() unexpected error: %v", err)
			}

			if first != second {
				t.Fatalf("same resolver returned %v then %v for key %#v", first, second, tt.key)
			}
			if first != third {
				t.Fatalf("different resolvers returned %v and %v for key %#v", first, third, tt.key)
			}
			if first >= ShardID(left.ShardCount()) {
				t.Fatalf("resolved shard = %v, want < %d", first, left.ShardCount())
			}
		})
	}
}

func TestDeterministicShardResolverResolveShardRejectsInvalidKey(t *testing.T) {
	t.Parallel()

	resolver, err := NewDeterministicShardResolver(16)
	if err != nil {
		t.Fatalf("NewDeterministicShardResolver() unexpected error: %v", err)
	}

	_, err = resolver.ResolveShard(storage.DocumentKey{})
	if !errors.Is(err, storage.ErrInvalidDocumentKey) {
		t.Fatalf("ResolveShard() error = %v, want %v", err, storage.ErrInvalidDocumentKey)
	}
}

func TestDeterministicShardResolverNilReceiver(t *testing.T) {
	t.Parallel()

	var resolver *DeterministicShardResolver
	_, err := resolver.ResolveShard(storage.DocumentKey{DocumentID: "doc-1"})
	if !errors.Is(err, ErrInvalidShardCount) {
		t.Fatalf("nil resolver error = %v, want %v", err, ErrInvalidShardCount)
	}
}
