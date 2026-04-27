# examples/gin-postgres

Este exemplo demonstra o adapter de `pkg/yhttp/gin` com persistência
PostgreSQL.

## O que ele faz

1. Lê `YJSBRIDGE_POSTGRES_DSN`.
2. Cria uma store PostgreSQL e um `yhttp.Server`.
3. Adapta o handler para Gin com `yhttp/gin.Handler(...)`.
4. Expõe o endpoint WebSocket em `/ws`.

## Como executar

```bash
cd examples/gin-postgres
export YJSBRIDGE_POSTGRES_DSN="postgres://user:pass@host:5432/dbname?sslmode=disable"
go run .
```

Depois conecte um cliente WebSocket em:

```text
ws://127.0.0.1:8080/ws?doc=notes&client=101&persist=1
```
