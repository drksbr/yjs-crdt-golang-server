# examples/postgres

Este exemplo demonstra a store PostgreSQL para snapshots.

## O que ele faz

1. Lê a variável `YJSBRIDGE_POSTGRES_DSN`.
2. Cria a store PostgreSQL com `pgstore.New(...)`.
3. Gera um snapshot inicial vazio com `yjsbridge.PersistedSnapshotFromUpdates()`.
4. Salva e carrega o snapshot, exibindo:
   - chave do documento,
   - data de armazenamento,
   - tamanho do `UpdateV1`,
   - estado vazio.

## Como executar

Defina a variável de ambiente com uma DSN válida e rode:

```bash
export YJSBRIDGE_POSTGRES_DSN="postgres://user:pass@host:5432/dbname?sslmode=disable"
go run .
```

## Requisitos

- PostgreSQL acessível na DSN informada.
- Dependência do driver PostgreSQL para a store `pkg/storage/postgres` já está no projeto.

A própria store aplica migration automática do schema usado pelo exemplo (`yjs_bridge_example`).
