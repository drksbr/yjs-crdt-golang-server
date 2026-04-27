# examples/protocol-awareness

Este exemplo demonstra a camada de protocolo e awareness em cenário multiplexado:

- `sync` (`SyncStep1` e `sync-update`)
- `auth` (`permission-denied`)
- `query-awareness`
- `awareness` no formato wire + runtime local de estado (`StateManager`)

Ele também mostra round-trip de mensagem awareness:
- serializa um update em nível de protocolo com `yawareness.EncodeProtocolUpdate`
- decodifica com `yawareness.DecodeProtocolUpdate`
- aplica no `yawareness.StateManager`
- gera resposta de consulta via `UpdateForClients` e reencoda.

## Como executar

```bash
cd examples/protocol-awareness
go run .
```

## O que esperar

Saída de exemplo:

```
runtime: estado atual do awareness
  client=7 clock=3 state={"name":"alice","color":"blue","cursor":12}
  client=13 clock=1 state={"name":"bob","focus":"sidebar"}
  client=42 clock=0 state={"name":"server","status":"present"}
decodificando protocolo multiplexado:
[1] protocol=sync
    sync: sync-step-1 payload=1 bytes
[2] protocol=sync
    sync: sync-update payload=4 bytes
[3] protocol=auth
    auth: permission-denied reason="invalid token"
[4] protocol=query-awareness
    query-awareness: solicitação de snapshot
[5] protocol=awareness
    awareness: 2 clientes
[6] protocol=awareness
    awareness: 3 clientes
    ...
```

## Observação

O exemplo usa a API pública atual em `pkg/yprotocol` e `pkg/yawareness`.
O escopo continua intencionalmente limitado a codec/envelope e runtime local em-processo, sem provider completo nem sincronização distribuída.
