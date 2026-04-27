// Package yhttpprometheus expõe um adapter de métricas Prometheus para
// `pkg/yhttp`.
//
// O pacote mantém o núcleo de transporte desacoplado do cliente Prometheus:
// `Metrics` implementa `yhttp.Metrics`, então pode ser injetado em
// `yhttp.ServerConfig` e registrado em um `prometheus.Registerer` próprio.
//
// As métricas atuais cobrem:
//   - conexões abertas e ativas;
//   - frames e bytes lidos/escritos;
//   - duração de `HandleEncodedMessages`;
//   - duração de persistência no fechamento;
//   - erros por estágio da borda HTTP/WebSocket.
package yhttpprometheus
