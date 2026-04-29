// Package ycluster define os contratos publicos minimos do control plane
// distribuido acima do provider local.
//
// A superficie inicial cobre:
//
//   - resolucao deterministica de shard a partir de `storage.DocumentKey`;
//   - tipos estaveis para placement, lease e owner resolution, incluindo
//     epoch monotônico e token opaco para fencing;
//   - interfaces pequenas para runtime local, lookup de owner e backends de
//     placement/lease;
//   - adapters storage-backed para wiring sobre `pkg/storage`;
//   - um `LeaseManager` opcional para acquire/renew/reacquire local de shards,
//     incluindo loop bloqueante de renovação com `context.Context`;
//   - um `StorageOwnershipCoordinator` opcional para claim/promoção/lookup/fence
//     storage-backed por documento, handoff atômico de lease/epoch e execução
//     bloqueante do lifecycle de ownership.
//   - um `DocumentOwnershipRuntime` opcional para compartilhar a mesma execução
//     de ownership entre múltiplos callers locais via ref-count.
//
// O pacote nao implementa transporte entre nos, eleicao, rebalanceamento,
// storage concreto nem coordenacao distribuida completa.
package ycluster
