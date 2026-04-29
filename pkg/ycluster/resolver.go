package ycluster

import (
	"hash/fnv"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
)

// DeterministicShardResolver resolve documentos para shards usando FNV-1a de
// forma estavel. O separador nulo entre namespace e document id evita colisoes
// triviais por concatenacao.
type DeterministicShardResolver struct {
	shardCount uint32
}

// NewDeterministicShardResolver cria um resolver com espaco fixo de shards.
func NewDeterministicShardResolver(shardCount uint32) (*DeterministicShardResolver, error) {
	if shardCount == 0 {
		return nil, ErrInvalidShardCount
	}
	return &DeterministicShardResolver{
		shardCount: shardCount,
	}, nil
}

// ShardCount retorna o tamanho do espaco logico de shards.
func (r *DeterministicShardResolver) ShardCount() uint32 {
	if r == nil {
		return 0
	}
	return r.shardCount
}

// ResolveShard mapeia deterministicamente a chave informada para um shard.
func (r *DeterministicShardResolver) ResolveShard(key storage.DocumentKey) (ShardID, error) {
	if r == nil || r.shardCount == 0 {
		return 0, ErrInvalidShardCount
	}
	if err := key.Validate(); err != nil {
		return 0, err
	}

	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(key.Namespace))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(key.DocumentID))

	return ShardID(hasher.Sum64() % uint64(r.shardCount)), nil
}
