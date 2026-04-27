// Package yawareness expõe a API pública de awareness do runtime base do projeto.
//
// A superfície pública coberta hoje é mínima e estável:
//
// - codec awareness em V1 (payload + envelope `ProtocolTypeAwareness`);
// - erros de parsing com contexto e offset em contrato (`ParseError`);
// - gerência local de estados recentes via `StateManager`.
//
// O pacote não implementa transporte de rede, provider ou distribuição entre nós.
// O estado remoto é apenas modelado para uso in-process e pode ser integrado por
// camadas de protocolo/ws conforme o projeto exigir.
package yawareness
