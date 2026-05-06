# DontPad Frontend2

Vite + React SPA preparada para ser embutida no executavel Go do backend.

## Desenvolvimento

```bash
npm install
DONTPAD_BACKEND_URL=http://127.0.0.1:8080 npm run dev
```

O Vite encaminha `/api/*` e `/ws` para `DONTPAD_BACKEND_URL`.

## Build embutido

```bash
cd examples/DontPadBR3/apps/frontend2
npm run build

cd ../backend
go build .
```

O pacote Go em `apps/frontend2/embed.go` embute `dist/**`. Em um clone limpo,
`dist/.gitkeep` mantém o pacote compilavel, mas o binario de release deve ser
gerado depois do `npm run build` para carregar a SPA real.

O build usa `base: "/"` porque o backend serve os assets em `/assets/*` e as
rotas como `/usuario/subnota` sao resolvidas pela SPA. Usar caminhos relativos
quebra o carregamento dos bundles em rotas profundas.
