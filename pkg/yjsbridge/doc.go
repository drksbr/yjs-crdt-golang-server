// Package yjsbridge expõe a superfície pública estável de update/snapshot do
// projeto em um contrato de API consistente para quem consome a biblioteca.
//
// A API disponível hoje cobre o fluxo canônico baseado em Yjs Update V1 e
// utilitários associados:
//
//   - Tipos de snapshot (`Snapshot` e `PersistedSnapshot`);
//   - Conversão de updates para snapshots persistidos;
//   - Serialização e restauração de snapshot persistido (`EncodePersistedSnapshotV1`
//     e `DecodePersistedSnapshotV1`), além das variantes context-aware.
//   - Operações de formato, merge, diff, state vector e conversão para V1;
//   - Extração/transformação de content IDs e funções auxiliares.
//
// O pacote é V1-first. Updates V2 válidos são aceitos pelo reader fixture-backed
// e normalizados para V1 canônico nas operações públicas de snapshot,
// state vector, content ids, merge, diff e intersect.
//
// Os helpers variadicos tratam payloads vazios como operação no-op na agregação
// de updates, conforme contratos internos existentes.
package yjsbridge
