# Yjs CRDT Golang Server

`yjs-crdt-golang-server` é uma implementação Go focada em interoperabilidade com o ecossistema Yjs.
O projeto cobre o núcleo de updates/snapshots, persistência, runtime local por documento e uma borda HTTP/WebSocket owner-aware para operação distribuída por ownership.

## O que já existe

- `pkg/yjsbridge`: merge, diff, state vector, content ids e snapshots persistíveis em V1, com entrada V2 válida e saídas V2 opt-in (`ConvertUpdateToV2`, `MergeUpdatesV2`, `DiffUpdateV2`).
- `pkg/storage`: contratos públicos para snapshot, update log, placement, lease, replay/recovery/compaction e fencing autoritativo.
- `pkg/storage/memory` e `pkg/storage/postgres`: backends de referência.
- `pkg/yprotocol`: runtime local por documento (`Provider`/`Connection`) com bootstrap por `snapshot + update log`, apply fenced/context-aware e normalização de sync V2 válido para V1 canônico.
- `pkg/storage/prometheus`, `pkg/yprotocol/prometheus` e `pkg/ycluster/prometheus`: adapters Prometheus com labels constantes opcionais para replay/recovery/compaction, lag/offset/epoch, lifecycle local do provider e control plane de lease/owner lookup.
- `pkg/yawareness`: estado efêmero de awareness/presence.
- `pkg/ycluster`: resolver de shard, owner lookup, adapters storage-backed, fonte de documentos via placement store, membership/health para targets dinâmicos, `LeaseManager`, `StorageOwnershipCoordinator` e `DocumentOwnershipRuntime` para claim/promoção/lookup/fence/rebalance planejado e periódico/lifecycle compartilhado de ownership por documento.
- `pkg/ynodeproto`: wire binário tipado entre nós.
- `pkg/yhttp`: borda HTTP/WebSocket genérica, owner-aware, com auth/authz/rate limit/quotas/origin policy/redaction plugáveis, tenant boundary opt-in, relay edge -> owner, normalização V2 -> V1 antes do wire inter-node, promoção local opt-in e ownership runtime opcional por conexão.
- `pkg/yhttp/prometheus`: adapter Prometheus para métricas de transporte/owner-aware, também com labels constantes opcionais.

## Estado atual

Hoje o projeto já suporta este perfil:

- modo single-process estável para referência e desenvolvimento;
- persistência durável com PostgreSQL;
- recovery por `snapshot + update log`;
- ownership por documento/shard com `placement + lease + epoch + token`, incluindo claim/lookup storage-backed;
- rebalance explícito por documento, planejamento determinístico e control loop periódico com no-op seguro, promoção opt-in quando o owner está ausente/expirado e handoff atômico de lease/epoch para owner ativo;
- fonte operacional de documentos baseada em listagem de placements persistidos;
- seleção dinâmica de targets saudáveis por membership/health no `RebalanceController`;
- lifecycle bloqueante de ownership por documento com renovação e release controlado de lease;
- runtime local ref-counted de ownership para compartilhar uma lease entre múltiplos callers do mesmo documento;
- integração opcional desse runtime na borda HTTP/WebSocket local, incluindo promoção local quando não há owner ativo;
- fencing autoritativo em storage e runtime do owner;
- cutover retryable (`503`/`1013`) quando um owner perde autoridade;
- revalidação imediata de autoridade na borda a partir do resultado do `RebalanceController`, acionando close retryable ou rebind transparente quando configurado;
- relay remoto entre edge e owner via protocolo inter-node tipado.
- handoff transparente do browser entre `remote -> remote`, `remote -> local` e `local -> remote` no mesmo WebSocket.
- seam de autenticação e validação de epoch no handshake inter-node owner-side.
- autenticação/autorização HTTP/WebSocket opt-in por `Authenticator`/`Authorizer`, com `BearerTokenAuthenticator` e `TenantAuthorizer` para isolar `DocumentKey.Namespace`.
- rate limit HTTP/WebSocket opt-in por `RateLimiter`, incluindo `FixedWindowRateLimiter` em memória e chaves por principal/IP, tenant ou documento como referência local.
- quotas HTTP/WebSocket opt-in por `QuotaLimiter`, incluindo `LocalQuotaLimiter` para conexões simultâneas por tenant/documento e budgets de bytes por conexão.
- política de Origin/CORS opt-in por `OriginPolicy`, com `StaticOriginPolicy` para allowlist exata, preflight e validação de método/headers.
- redaction opt-in de requests em métricas/handlers de erro por `RequestRedactor`, com `HashingRequestRedactor` para hashing salgado de ids sensíveis.
- autenticação inter-node por bearer dedicado ou HMAC com `key_id`, timestamp/nonce, replay protection local e suporte a rotação de segredo, além de validadores fail-closed para configs de produção.
- reader/encoder V2 fixture-backed, com conversão V1 <-> V2, saídas públicas V2 opt-in e payloads de sync ainda normalizados para V1 canônico em protocolo/storage.

## O que ainda falta na fase distribuída

- evolução do oráculo observacional para SLOs reais, multi-região e multi-tenant;
- completar hardening operacional para ambiente público multi-tenant: enforcement distribuído de quotas, mTLS/rotação/gestão de chaves inter-node, rollout auditado de defaults fail-closed e calibração de limites por tráfego real.

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
- `examples/owner-aware-http-edge/observability`: dashboards Grafana, alertas Prometheus por node e regras SLO por `env`/`region`/`tenant`.
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
- lacunas por função: [docs/known-gaps-by-function.md](docs/known-gaps-by-function.md)
