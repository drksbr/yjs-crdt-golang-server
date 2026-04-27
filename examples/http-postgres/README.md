# examples/http-postgres

Este exemplo demonstra a borda HTTP/WebSocket genérica de `pkg/yhttp` usando
persistência PostgreSQL.

## O que ele faz

1. Lê `YJSBRIDGE_POSTGRES_DSN`.
2. Cria uma store PostgreSQL e um `yprotocol.Provider`.
3. Expõe o endpoint WebSocket em `/ws` por `net/http`.
4. Reaproveita snapshots persistidos no PostgreSQL entre reinícios.

## Como executar

```bash
cd examples/http-postgres
export YJSBRIDGE_POSTGRES_DSN="postgres://user:pass@host:5432/dbname?sslmode=disable"
go run .
```

Depois conecte um cliente WebSocket em:

```text
ws://127.0.0.1:8080/ws?doc=notes&client=101&persist=1
```
