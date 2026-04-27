# examples/protocol-session

Este exemplo demonstra o runtime mínimo em-processo de `pkg/yprotocol` com `Session` e `HandleEncodedMessages`.

## O que ele faz

1. Cria uma sessão local com estado de documento e awareness em memória.
2. Monta mensagens codificadas com os encoders públicos de envelope em `pkg/yprotocol`.
3. Processa um fluxo multiplexado de `sync`, `awareness`, `auth` e `query-awareness`.
4. Coleta as respostas produzidas pela sessão e imprime o envelope de entrada/saída.

## Como executar

```bash
cd examples/protocol-session
go run .
```

## O que o exemplo cobre

- `Session` como runtime mínimo para uma conexão em processo.
- `HandleEncodedMessages` para consumir bytes já protocolados.
- encode/decode público de envelopes em `pkg/yprotocol`.
- integração local com awareness, sem transporte externo.

## Observação

O escopo é intencionalmente pequeno: o exemplo mostra o contrato básico de sessão e roteamento local de mensagens, não um provider completo nem sincronização distribuída.
