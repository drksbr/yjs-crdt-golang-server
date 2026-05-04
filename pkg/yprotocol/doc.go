// Package yprotocol expõe a API pública do envelope y-protocols.
//
// A superfície atual cobre:
// - mensagens sync/auth/query-awareness;
// - decode do envelope combinado com awareness;
// - leitura incremental de mensagens via io.Reader.
// - encode tipado de `ProtocolMessage`;
// - runtime mínimo in-process via `Session`.
// - provider local single-process via `Provider`/`Connection`.
//
// Awareness payload/runtime continua em `pkg/yawareness`, e a `Session` compõe
// esse runtime para um fluxo local mínimo. O `Provider` sobe um nível acima da
// `Session`, mantendo snapshot autoritativo por documento, fanout local entre
// conexões do mesmo processo, apply context-aware e persistência explícita
// opcional via `pkg/storage`. Updates V2 válidos podem entrar no sync runtime,
// e o estado interno do documento permanece V2-canônico; bytes V1 são derivados
// apenas para APIs antigas, stores sem contrato V2 e egress de cliente não
// negociado.
package yprotocol
