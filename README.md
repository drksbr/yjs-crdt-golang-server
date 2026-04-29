# Yjs CRDT Golang Server

`yjs-crdt-golang-server` é uma implementação Go focada em interoperabilidade com o ecossistema Yjs.
O projeto cobre o núcleo de updates/snapshots, persistência, runtime local por documento e uma borda HTTP/WebSocket owner-aware para operação distribuída por ownership.

## O que já existe

- `pkg/yjsbridge`: merge, diff, state vector, content ids e snapshots persistíveis em V1.
- `pkg/storage`: contratos públicos para snapshot, update log, placement, lease, replay/recovery/compaction e fencing autoritativo.
- `pkg/storage/memory` e `pkg/storage/postgres`: backends de referência.
- `pkg/yprotocol`: runtime local por documento (`Provider`/`Connection`) com bootstrap por `snapshot + update log` e apply fenced/context-aware.
- `pkg/storage/prometheus`, `pkg/yprotocol/prometheus` e `pkg/ycluster/prometheus`: adapters Prometheus com labels constantes opcionais para replay/recovery/compaction, lag/offset/epoch, lifecycle local do provider e control plane de lease/owner lookup.
- `pkg/yawareness`: estado efêmero de awareness/presence.
- `pkg/ycluster`: resolver de shard, owner lookup, adapters storage-backed, `LeaseManager`, `StorageOwnershipCoordinator` e `DocumentOwnershipRuntime` para claim/promoção/lookup/fence/lifecycle compartilhado de ownership por documento.
- `pkg/ynodeproto`: wire binário tipado entre nós.
- `pkg/yhttp`: borda HTTP/WebSocket genérica, owner-aware, com relay edge -> owner, promoção local opt-in e ownership runtime opcional por conexão.
- `pkg/yhttp/prometheus`: adapter Prometheus para métricas de transporte/owner-aware, também com labels constantes opcionais.

## Estado atual

Hoje o projeto já suporta este perfil:

- modo single-process estável para referência e desenvolvimento;
- persistência durável com PostgreSQL;
- recovery por `snapshot + update log`;
- ownership por documento/shard com `placement + lease + epoch + token`, incluindo claim/lookup storage-backed;
- lifecycle bloqueante de ownership por documento com renovação e release controlado de lease;
- runtime local ref-counted de ownership para compartilhar uma lease entre múltiplos callers do mesmo documento;
- integração opcional desse runtime na borda HTTP/WebSocket local, incluindo promoção local quando não há owner ativo;
- fencing autoritativo em storage e runtime do owner;
- cutover retryable (`503`/`1013`) quando um owner perde autoridade;
- relay remoto entre edge e owner via protocolo inter-node tipado.
- handoff transparente do browser entre `remote -> remote`, `remote -> local` e `local -> remote` no mesmo WebSocket.
- seam de autenticação e validação de epoch no handshake inter-node owner-side.

## O que ainda falta na fase distribuída

- coordenação/autonomia de ownership acima do runtime local atual (`LeaseManager`/`StorageOwnershipCoordinator`/`DocumentOwnershipRuntime`) para rebalance multi-nó;
- semântica final de handoff atômico por `epoch`, além do rebind/bootstrap já operacional;
- evolução do oráculo observacional para SLOs reais, multi-região e multi-tenant;
- hardening de segurança e operação para ambiente público multi-tenant.

## Estrutura principal

```text
pkg/
  yjsbridge/
  storage/
    memory/
    postgres/
  yprotocol/
  yawareness/
  ycluster/
  ynodeproto/
  yhttp/
    chi/
    echo/
    gin/
    prometheus/
integration/
examples/
```

## Exemplos

- `examples/http-memory`
- `examples/http-postgres`
- `examples/owner-aware-http-edge`
- `examples/owner-aware-http-edge/observability`
- `examples/nextjs-quill-dontpad`

## Testes

```bash
go test ./...
```

Para o exemplo frontend:

```bash
cd examples/nextjs-quill-dontpad/frontend
npm install
npm run lint
npm run build
```

## Documentação de trabalho

- roadmap detalhado: [TASK.md](TASK.md)
- especificação de produto/técnica: [SPEC.md](SPEC.md)
