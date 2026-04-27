# examples/nextjs-quill-dontpad

Exemplo de backend colaborativo para um frontend `Next.js + Quill`, usando
`gin`, `pkg/yhttp` e `pkg/storage/postgres`.

## O que ele demonstra

- backend Gin com persistência em PostgreSQL
- endpoint de saúde em `/healthz`
- endpoint WebSocket em `/ws`
- resolução do documento a partir do slug enviado em `?doc=<slug>`
- handshake WebSocket aceitando `http://localhost:3000` e
  `http://127.0.0.1:3000` por padrão

O backend não serve o frontend; ele foi pensado para rodar ao lado do app Next
em desenvolvimento.

## Como executar o backend

```bash
export YJSBRIDGE_POSTGRES_DSN="postgres://user:pass@127.0.0.1:5432/yjsbridge?sslmode=disable"
go run ./examples/nextjs-quill-dontpad
```

Com o servidor no ar:

- health check: `http://127.0.0.1:8080/healthz`
- WebSocket base: `ws://127.0.0.1:8080/ws`

Exemplo de conexão:

```text
ws://127.0.0.1:8080/ws?doc=welcome&client=101&persist=1
```

O valor de `doc` é o slug do documento. Se duas abas usarem o mesmo slug, elas
compartilham o mesmo documento persistido no Postgres.

## Como rodar junto com o frontend Next

Em outro terminal:

```bash
cd examples/nextjs-quill-dontpad/frontend
npm install
npm run dev
```

O frontend deve rodar em `http://127.0.0.1:3000` ou `http://localhost:3000`
para passar no filtro de origem padrão do WebSocket.

Ao montar a URL do provider WebSocket, o frontend precisa apontar para
`ws://127.0.0.1:8080/ws` e incluir:

- `doc`: slug do documento
- `client`: client id numérico do peer Yjs
- `persist`: opcional; por padrão o backend persiste no fechamento da conexão

## Variáveis úteis

- `YJSBRIDGE_POSTGRES_DSN`: obrigatória
- `YJSBRIDGE_DEMO_ADDR`: default `:8080`
- `YJSBRIDGE_DEMO_SCHEMA`: default `yjs_bridge_nextjs_quill_dontpad`
- `YJSBRIDGE_ALLOWED_ORIGINS`: lista separada por vírgula para sobrescrever as
  origens permitidas no handshake WebSocket

## Observações

- o namespace persistido no backend é fixo em `nextjs-quill-dontpad`
- se `persist` vier vazio, o backend assume persistência habilitada
