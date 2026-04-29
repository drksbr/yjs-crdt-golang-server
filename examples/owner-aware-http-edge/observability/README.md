# Observability

Artefatos de referencia para operar o exemplo `owner-aware-http-edge` com Prometheus e Grafana.

## Arquivos

- `prometheus-rules.yml`: alertas por `node_id`/`deployment_role` para perda de autoridade, falhas de lease, erro de lookup/handshake, lag de recovery e compaction parada.
- `prometheus-slo-rules.yml`: recording rules e alertas SLO agregados por `env`/`region`/`tenant`/`deployment_role` para disponibilidade, latência, autoridade, lease e recovery lag.
- `grafana-dashboard.json`: dashboard operacional para um conjunto de nodes, com conexoes, decisoes de rota, handoff/rebind, lease, offsets, epoch, erros e latencias p95.
- `grafana-oracle-dashboard.json`: dashboard holistico para observar a plataforma por `node_id` e `deployment_role`.

## Uso

1. Configure Prometheus para coletar `/metrics` dos processos edge e owner.
2. Carregue `prometheus-rules.yml` no Prometheus ou no Alertmanager stack equivalente.
3. Carregue `prometheus-slo-rules.yml` para recording rules e alertas de SLO.
4. Importe `grafana-dashboard.json` no Grafana para operacao detalhada.
5. Importe `grafana-oracle-dashboard.json` no Grafana para a visao central/oraculo.

As expressoes assumem o namespace padrao dos adapters: `yjsbridge_*`.
O exemplo adiciona os labels constantes `node_id`, `deployment_role` e `env`
diretamente nos adapters Prometheus. Para usar os SLOs multi-tenant/regiao,
adicione tambem `region` e `tenant` em `ConstLabels`. Sem esses labels, as
series continuam validas, mas as agregacoes por `region`/`tenant` ficam sem
essas dimensoes no resultado.
