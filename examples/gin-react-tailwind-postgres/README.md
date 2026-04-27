# examples/gin-react-tailwind-postgres

Exemplo full-stack com `vite`, `react`, `tailwindcss`, `gin`, `pkg/yhttp` e
`pkg/storage/postgres`.

## O que ele demonstra

- sincronização de texto em tempo real usando `Y.Doc` + `Y.Text`
- presença (`awareness`) com nome, cor e seleção atual de cada pessoa
- backend Gin usando o adapter `pkg/yhttp/gin`
- persistência do documento em PostgreSQL no fechamento das conexões

O editor é um texto colaborativo simples, focado em mostrar a integração fim a
fim entre o frontend do ecossistema Yjs e a borda Go deste repositório.

## Como executar

Backend:

```bash
export YJSBRIDGE_POSTGRES_DSN="postgres://user:pass@127.0.0.1:5432/yjsbridge?sslmode=disable"
go run ./examples/gin-react-tailwind-postgres
```

Frontend em desenvolvimento:

```bash
cd examples/gin-react-tailwind-postgres/frontend
npm install
npm run dev
```

Abra `http://127.0.0.1:5173`.

## Como servir tudo pelo Gin

Depois de instalar as dependências do frontend:

```bash
cd examples/gin-react-tailwind-postgres/frontend
npm run build
cd ../../..
go run ./examples/gin-react-tailwind-postgres
```

Com `frontend/dist` presente, o Gin passa a servir a SPA em
`http://127.0.0.1:8080`.

## Variáveis úteis

- `YJSBRIDGE_POSTGRES_DSN`: obrigatória
- `YJSBRIDGE_DEMO_ADDR`: default `:8080`
- `YJSBRIDGE_DEMO_SCHEMA`: default `yjs_bridge_gin_react_demo`
- `YJSBRIDGE_ALLOWED_ORIGINS`: lista separada por vírgula para o handshake
  WebSocket em desenvolvimento
