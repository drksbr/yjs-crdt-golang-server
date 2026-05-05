# Desenvolvimento local (Next + backend Go)

Este fluxo usa o frontend `Next.js` apenas para UI, com backend completo em Go
(auth, PIN, persistência CRDT em Postgres, realtime sync e awareness).

## Pré-requisitos

- Node.js 20+
- Go 1.26+
- PostgreSQL local

## 1) Suba o backend Go

Na raiz do repositório:

```bash
export DATABASE_URL=postgres://USER@127.0.0.1:5432/dontpadbr3
export DONTPAD_ADDR=:8080
export DONTPAD_SCHEMA=dontpadbr3
export DONTPAD_NAMESPACE=dontpadbr3
go run ./apps/backend
```

## 2) Suba o frontend Next

No diretório `apps/frontend`:

```bash
cp .env.local.example .env.local
npm install
npm run dev
```

Para subir backend e frontend juntos:

```bash
npm run dev:local
```

Por padrão:

- Next: `http://127.0.0.1:3000`
- Backend Go: `http://127.0.0.1:8080`
- WebSocket: `ws://127.0.0.1:8080/ws`

## Observações

- O Next usa proxy para encaminhar `/api/*` para o backend Go via
  `DONTPAD_BACKEND_URL`.
- O backend persiste snapshots/update logs no Postgres e usa
  `storage/data` para anexos/áudios.
