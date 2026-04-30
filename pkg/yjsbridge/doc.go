// Package yjsbridge expõe a superfície pública estável de update/snapshot do
// projeto em um contrato de API consistente para quem consome a biblioteca.
//
// A API disponível hoje cobre o fluxo canônico baseado em Yjs Update V1,
// variantes explícitas de saída V2 e utilitários associados:
//
//   - Tipos de snapshot (`Snapshot` e `PersistedSnapshot`);
//   - Conversão de updates para snapshots persistidos;
//   - Serialização e restauração de snapshot persistido (`EncodePersistedSnapshotV1`
//     e `DecodePersistedSnapshotV1`), além das variantes context-aware.
//   - Operações de formato, merge, diff, state vector e conversão para V1/V2;
//   - Extração/transformação de content IDs e funções auxiliares.
//
// O pacote é V1-first: as operações sem sufixo continuam retornando V1
// canônico. Updates V2 válidos são aceitos pelo reader fixture-backed e
// normalizados para V1 canônico nas operações públicas de snapshot, state
// vector, content ids, merge, diff e intersect. Para saída V2, use as variantes
// opt-in `ConvertUpdateToV2`, `ConvertUpdatesToV2`, `MergeUpdatesV2`,
// `DiffUpdateV2` e `IntersectUpdateWithContentIDsV2`.
//
// Os helpers variadicos tratam payloads vazios como operação no-op na agregação
// de updates, conforme contratos internos existentes.
package yjsbridge
