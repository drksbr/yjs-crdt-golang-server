# Yjs Go Bridge

`yjs-go-bridge` é uma implementação Go focada em interoperabilidade com o ecossistema Yjs.
O projeto cobre o núcleo de updates/snapshots, persistência, runtime local por documento e uma borda HTTP/WebSocket owner-aware para operação distribuída por ownership.

## O que já existe

- `pkg/yjsbridge`: merge, diff, state vector, content ids e snapshots persistíveis em V1.
- `pkg/storage`: contratos públicos para snapshot, update log, placement, lease, replay/recovery e fencing autoritativo.
- `pkg/storage/memory` e `pkg/storage/postgres`: backends de referência.
- `pkg/yprotocol`: runtime local por documento (`Provider`/`Connection`) com bootstrap por `snapshot + update log`.
- `pkg/yawareness`: estado efêmero de awareness/presence.
- `pkg/ycluster`: resolver de shard, owner lookup e adapters storage-backed.
- `pkg/ynodeproto`: wire binário tipado entre nós.
- `pkg/yhttp`: borda HTTP/WebSocket genérica, owner-aware e com relay edge -> owner.
- `pkg/yhttp/prometheus`: adapter Prometheus para métricas de transporte/owner-aware.

## Estado atual

Hoje o projeto já suporta este perfil:

- modo single-process estável para referência e desenvolvimento;
- persistência durável com PostgreSQL;
- recovery por `snapshot + update log`;
- ownership por documento/shard com `lease + epoch + token`;
- fencing autoritativo em storage e runtime do owner;
- cutover retryable (`503`/`1013`) quando um owner perde autoridade;
- relay remoto entre edge e owner via protocolo inter-node tipado.
- handoff transparente do browser entre `remote -> remote`, `remote -> local` e `local -> remote` no mesmo WebSocket.

## O que ainda falta na fase distribuída

- coordenação/autonomia de ownership acima de `lease`/`epoch` para promoção, renew e rebalance sem script externo;
- semântica final de handoff atômico por `epoch`, além do rebind/bootstrap já operacional;
- observabilidade operacional mais profunda para lease, replay, lag, checkpoint/compaction e recovery;
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
