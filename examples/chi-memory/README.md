# examples/chi-memory

Este exemplo demonstra o helper de `pkg/yhttp/chi` em cima do handler genérico
de `pkg/yhttp`.

## O que ele faz

1. Cria um `yhttp.Server` com provider local e store em memória.
2. Monta o handler no router Chi com `yhttp/chi.Mount(...)`.
3. Expõe o endpoint WebSocket em `/ws`.

## Como executar

```bash
cd examples/chi-memory
go run .
```

Depois conecte um cliente WebSocket em:

```text
ws://127.0.0.1:8080/ws?doc=notes&client=101&persist=1
```
