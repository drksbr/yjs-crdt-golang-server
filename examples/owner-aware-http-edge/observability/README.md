# Observability

Artefatos de referencia para operar o exemplo `owner-aware-http-edge` com Prometheus e Grafana.

## Arquivos

- `prometheus-rules.yml`: alertas por `node_id`/`deployment_role` para perda de autoridade, falhas de lease, erro de lookup/handshake, lag de recovery e compaction parada.
- `grafana-dashboard.json`: dashboard operacional para um conjunto de nodes, com conexoes, decisoes de rota, handoff/rebind, lease, offsets, epoch, erros e latencias p95.
- `grafana-oracle-dashboard.json`: dashboard holistico para observar a plataforma por `node_id` e `deployment_role`.

## Uso

1. Configure Prometheus para coletar `/metrics` dos processos edge e owner.
2. Carregue `prometheus-rules.yml` no Prometheus ou no Alertmanager stack equivalente.
3. Importe `grafana-dashboard.json` no Grafana para operacao detalhada.
4. Importe `grafana-oracle-dashboard.json` no Grafana para a visao central/oraculo.

As expressoes assumem o namespace padrao dos adapters: `yjsbridge_*`.
O exemplo adiciona os labels constantes `node_id`, `deployment_role` e `env`
diretamente nos adapters Prometheus.
