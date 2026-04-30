# examples/owner-aware-http-edge

Este exemplo mostra como preparar uma borda HTTP/WebSocket com decisao de
ownership usando apenas as APIs publicas atuais:

- `pkg/storage/memory` como `DistributedStore`;
- `pkg/ycluster.DeterministicShardResolver`;
- `pkg/ycluster.StorageLeaseStore`;
- `pkg/ycluster.StorageOwnerLookup`;
- `pkg/ycluster.DocumentOwnershipRuntime`;
- `pkg/yhttp.OwnerAwareServer`;
- `pkg/yhttp.NewWebSocketRemoteOwnerDialer`;
- `pkg/yhttp.NewRemoteOwnerForwardHandler`;
- `pkg/yhttp.NewRemoteOwnerEndpoint`;
- adapters Prometheus de `pkg/storage`, `pkg/yprotocol`, `pkg/ycluster` e `pkg/yhttp`.

O exemplo semeia dois documentos:

- um documento owned localmente por `node-a`, que abre normalmente em `/ws`;
- um documento owned por `node-b`, que atravessa o edge em `:8080` e e
  encaminhado ao owner remoto em `:9090/node`.

## O que ele demonstra

1. Persistencia de `document -> shard` com `PlacementRecord`.
2. Persistencia de ownership por shard com `LeaseStore`.
3. Runtime de ownership por documento com renew automatico e compartilhamento com sessoes ativas.
4. Lookup de owner antes do upgrade WebSocket.
5. Delegacao ao `pkg/yhttp` quando o owner resolvido e local.
6. Forward tipado edge -> owner usando `NodeMessageStream`.
7. Endpoint owner-side `/node` compartilhando o mesmo runtime/fanout de `/ws`.
8. Hook de autenticação inter-node em `RemoteOwnerEndpointConfig.Authenticate`
   antes de materializar a sessão no owner remoto.
9. Endpoint `/owner` que continua expondo a rota browser do owner e o epoch atual.
10. Endpoint `/metrics` nos dois servidores de exemplo com métricas de transporte,
   owner lookup, lease, replay/recovery, lag, epoch e persistência.
11. Bundle `observability/` com alertas Prometheus, dashboard Grafana operacional
    e dashboard central/oráculo por node.

## Como executar

```bash
cd examples/owner-aware-http-edge
go run .
```

O servidor sobe em `http://127.0.0.1:8080`.
O owner remoto de demonstração sobe com:

- browser/publico em `ws://127.0.0.1:9090/ws`
- inter-node tipado em `ws://127.0.0.1:9090/node`
- métricas em `http://127.0.0.1:9090/metrics`

O edge expõe métricas em `http://127.0.0.1:8080/metrics`.

## Observabilidade

O diretório `observability/` inclui:

- `prometheus-rules.yml` para perda de autoridade, falhas de lease, lookup/handshake,
  lag de recovery, compaction parada e closes inesperados.
- `prometheus-slo-rules.yml` com recording rules e alertas SLO agregados por
  `env`, `region`, `tenant` e `deployment_role`.
- `grafana-dashboard.json` com painéis de conexões, rotas, handoff/rebind, leases,
  offsets, epochs, erros e latências p95.
- `grafana-oracle-dashboard.json` com visão holística por `node_id` e
  `deployment_role`.

As regras assumem scrape dos endpoints `/metrics` do edge e do owner remoto.
O exemplo rotula as métricas com `node_id`, `deployment_role` e `env`, simulando
o que um Prometheus central veria ao coletar todos os nodes da plataforma.

## Segurança

Este exemplo demonstra apenas o seam inter-node: o endpoint `/node` do owner
remoto usa `RemoteOwnerEndpointConfig.Authenticate` para rejeitar handshakes que
não venham do `node-a` esperado. Isso valida o ponto de extensão, mas não é uma
política de produção.

Para expor esta topologia publicamente, a aplicação deve configurar também:

- `Authenticator`/`Authorizer` no `Server` local e no `OwnerAwareServer`, por
  exemplo `BearerTokenAuthenticator` com `TenantAuthorizer` para isolar
  `DocumentKey.Namespace`;
- `RateLimiter` na borda HTTP/WebSocket, preferencialmente com backend
  distribuído quando houver mais de um edge;
- quotas distribuídas por tenant/documento/conexão para forwarding, owner
  lookup e custo de replay/storage; `pkg/yhttp.LocalQuotaLimiter` cobre apenas
  a referência local por processo;
- política inter-node obrigatória com identidade de nó, bearer/HMAC com
  `key_id`/mTLS, expiração curta, distribuição segura de chaves e proteção contra
  replay;
- redaction de namespaces, document ids, principals, tokens e connection ids em
  logs, labels de métricas, erros e dashboards.

## Como testar

Abra a raiz para ver os documentos escolhidos e as URLs prontas:

```text
http://127.0.0.1:8080/
```

Exemplos tipicos:

```text
http://127.0.0.1:8080/owner?doc=notes-local&client=101&persist=1
ws://127.0.0.1:8080/ws?doc=notes-local&client=101&persist=1
http://127.0.0.1:8080/owner?doc=notes-remote&client=101&persist=1
ws://127.0.0.1:8080/ws?doc=notes-remote&client=101&persist=1
```

Comportamento esperado:

- `notes-local` resolve para `node-a` e passa para o handler real de `pkg/yhttp`.
- `notes-remote` resolve para `node-b`; o endpoint `/owner` mostra a rota
  browser do owner e o endpoint `/ws` em `:8080` abre um stream tipado ate
  `ws://127.0.0.1:9090/node`.
- O owner local ou remoto adota as leases semeadas e passa a renova-las pelo
  `DocumentOwnershipRuntime`; conexoes e streams compartilham o mesmo runtime.
- Documentos novos podem ser promovidos localmente pelo edge quando nao existe
  owner ativo e o runtime de ownership local esta configurado.
- O edge nao materializa rooms remotos localmente; ele apenas resolve owner e
  encaminha a sessao ao endpoint owner-side do no remoto.

## Limite intencional

O transporte entre nos agora e tipado, mas o escopo continua deliberadamente
enxuto:

- nao existe discovery dinamico entre nos;
- ha rebind transparente de sessoes WebSocket no fluxo owner-aware durante handoff; fora desse caminho nao existe migracao generica de sessoes;
- discovery de owners ainda vem de `PlacementStore + LeaseStore`.

Mesmo assim, o exemplo ja cobre o caminho que faltava na borda:
resolver owner com `pkg/ycluster`, impedir materializacao no no errado e
encaminhar a sessao ao owner remoto sem relay bruto/manual de WebSocket.
