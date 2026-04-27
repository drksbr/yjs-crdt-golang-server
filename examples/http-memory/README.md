# examples/http-memory

Este exemplo demonstra a borda HTTP/WebSocket genérica de `pkg/yhttp` usando
`pkg/yprotocol.Provider` com persistência em memória.

## O que ele faz

1. Cria uma `memory` store para snapshots.
2. Inicializa um `yprotocol.Provider` em processo.
3. Expõe um endpoint WebSocket em `/ws` por `net/http`.
4. Resolve `doc`, `client`, `conn` e `persist` via query string.

## Como executar

```bash
cd examples/http-memory
go run .
```

Depois conecte um cliente WebSocket em:

```text
ws://127.0.0.1:8080/ws?doc=notes&client=101&persist=1
```

## Observação

O handler é um `http.Handler`, então o mesmo objeto pode ser adaptado para Gin,
Echo e outras bibliotecas compatíveis com `net/http`.
