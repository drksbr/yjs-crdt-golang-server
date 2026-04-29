// Package yhttp expõe uma borda mínima de transporte HTTP/WebSocket acima de
// `pkg/yprotocol.Provider`.
//
// O pacote foi desenhado para permanecer neutro em relação ao framework:
// `Server` implementa `http.Handler`, então pode ser usado diretamente com
// `net/http` e adaptado em frameworks como Gin e Echo sem duplicar a lógica de
// sync/awareness.
//
// Escopo atual:
//   - upgrade WebSocket via `net/http`;
//   - roteamento de conexão para um `storage.DocumentKey`;
//   - resolução opcional de owner antes do provider local via
//     `OwnerAwareServer`;
//   - promoção local opt-in via
//     `OwnerAwareServerConfig.PromoteLocalOnOwnerUnavailable`, exigindo
//     `ServerConfig.OwnershipRuntime`;
//   - claim/release opcional de ownership local por conexão/stream via
//     `ServerConfig.OwnershipRuntime`;
//   - seam opcional de forwarding remoto via `RemoteOwnerDialer`/
//     `NodeMessageStream`, plugável atrás de `OwnerAwareServerConfig.OnRemoteOwner`;
//   - terminador owner-side via `RemoteOwnerEndpoint`, permitindo anexar peers
//     encaminhados ao mesmo fanout/registry local do `Server`;
//   - transporte inter-node concreto sobre WebSocket binário via
//     `NewWebSocketRemoteOwnerDialer`;
//   - leitura/escrita de frames binários do protocolo Yjs;
//   - fanout local de broadcasts produzidos por `pkg/yprotocol.Provider`;
//   - persistência opcional do snapshot no fechamento da conexão;
//   - hooks opcionais de observabilidade via `Metrics`, com adapter Prometheus
//     disponível em `pkg/yhttp/prometheus`.
//
// O pacote ainda não implementa discovery automático entre nós, replicação
// completa entre processos ou suporte operacional a Update V2; quando o owner
// resolvido é remoto, o caller pode injetar um `OnRemoteOwner` apropriado ou
// manter o fallback retryable com metadados do owner.
package yhttp
