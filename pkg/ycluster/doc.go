// Package ycluster define os contratos publicos minimos do control plane
// distribuido acima do provider local.
//
// A superficie inicial cobre:
//
//   - resolucao deterministica de shard a partir de `storage.DocumentKey`;
//   - tipos estaveis para placement, lease e owner resolution;
//   - interfaces pequenas para runtime local, lookup de owner e backends de
//     placement/lease;
//   - adapters storage-backed para wiring sobre `pkg/storage`.
//
// O pacote nao implementa transporte entre nos, eleicao, rebalanceamento,
// storage concreto nem coordenacao distribuida completa.
package ycluster
