// Package storage define contratos públicos para persistência de snapshots e
// extensões opcionais para uma topologia distribuída.
//
// SnapshotStore continua sendo o contrato mínimo compatível para armazenar e
// recuperar snapshots persistidos em V1. Quando o runtime precisar separar log
// de updates, placement de shards e leases temporárias de ownership, o pacote
// também expõe interfaces independentes para essas responsabilidades, sem
// acoplar a API pública a um backend concreto.
//
// O pacote também já inclui helpers públicos de replay/recovery/compaction
// (`ReplaySnapshot`, `RecoverSnapshot`, `CompactUpdateLog` e
// `CompactUpdateLogAuthoritative`) para compor `snapshot + update log` sem
// depender de um runtime distribuído completo, além de `LeaseHandoffStore` para
// troca atômica de owner/epoch quando um runtime externo coordenar o handoff.
package storage
