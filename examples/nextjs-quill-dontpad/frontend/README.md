# frontend

Aplicacao `Next.js + Quill + Yjs` do exemplo `nextjs-quill-dontpad`.

## Executar em desenvolvimento

```bash
npm install
npm run dev
```

Abra `http://127.0.0.1:3000`.

Por padrao, o frontend tenta falar com o backend do exemplo em
`ws://127.0.0.1:8080/ws`.

## Variavel opcional

- `NEXT_PUBLIC_YJS_WS_URL`: sobrescreve o endpoint WebSocket do bridge

## Observacoes

- uploads ficam em `frontend/.uploads/`
- anexos sao um recurso de demo local e nao substituem storage externo
