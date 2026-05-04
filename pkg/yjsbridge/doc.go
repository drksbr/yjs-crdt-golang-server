// Package yjsbridge expõe a superfície pública estável de update/snapshot do
// projeto em um contrato de API consistente para quem consome a biblioteca.
//
// A API disponível hoje cobre snapshots persistíveis V2-canônicos, saídas V1
// de compatibilidade e utilitários associados:
//
//   - Tipos de snapshot (`Snapshot` e `PersistedSnapshot`);
//   - Conversão de updates para snapshots persistidos;
//   - Serialização e restauração de snapshot persistido (`EncodePersistedSnapshotV1`
//     e `DecodePersistedSnapshotV1`), além das variantes context-aware.
//   - Operações de formato, merge, diff, state vector e conversão para V1/V2;
//   - Extração/transformação de content IDs e funções auxiliares.
//
// Snapshots persistidos mantêm `UpdateV2` como forma canônica e materializam
// `UpdateV1` para leitores antigos. As operações sem sufixo continuam
// retornando bytes V1-compatible; para saída V2, use `ConvertUpdateToV2`,
// `ConvertUpdatesToV2`, `MergeUpdatesV2`, `DiffUpdateV2` e
// `IntersectUpdateWithContentIDsV2`.
//
// Os helpers variadicos tratam payloads vazios como operação no-op na agregação
// de updates, conforme contratos internos existentes.
package yjsbridge
