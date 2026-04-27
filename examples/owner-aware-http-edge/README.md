# examples/owner-aware-http-edge

Este exemplo mostra como preparar uma borda HTTP/WebSocket com decisao de
ownership usando apenas as APIs publicas atuais:

- `pkg/storage/memory` como `DistributedStore`;
- `pkg/ycluster.DeterministicShardResolver`;
- `pkg/ycluster.StorageLeaseStore`;
- `pkg/ycluster.StorageOwnerLookup`;
- `pkg/yhttp.OwnerAwareServer` + `OnRemoteOwner` para relay WebSocket.

O exemplo semeia dois documentos:

- um documento owned localmente por `node-a`, que abre normalmente em `/ws`;
- um documento owned por `node-b`, que atravessa o edge em `:8080` e e
  encaminhado ao owner remoto em `:9090`.

## O que ele demonstra

1. Persistencia de `document -> shard` com `PlacementRecord`.
2. Persistencia de ownership por shard com `LeaseStore`.
3. Lookup de owner antes do upgrade WebSocket.
4. Delegacao ao `pkg/yhttp` quando o owner resolvido e local.
5. Relay de WebSocket para owner remoto usando o hook `OnRemoteOwner`.
6. Endpoint `/owner` que continua expondo a rota resolvida e o epoch atual.

## Como executar

```bash
cd examples/owner-aware-http-edge
go run .
```

O servidor sobe em `http://127.0.0.1:8080`.
O owner remoto de demonstração sobe em `ws://127.0.0.1:9090/ws`.

## Como testar

Abra a raiz para ver os documentos escolhidos e as URLs prontas:

```text
http://127.0.0.1:8080/
```

Exemplos tipicos:

```text
http://127.0.0.1:8080/owner?doc=notes-local&client=101&persist=1
ws://127.0.0.1:8080/ws?doc=notes-local&client=101&persist=1
http://127.0.0.1:8080/owner?doc=notes-remote&client=101&persist=1
ws://127.0.0.1:8080/ws?doc=notes-remote&client=101&persist=1
```

Comportamento esperado:

- `notes-local` resolve para `node-a` e passa para o handler real de `pkg/yhttp`.
- `notes-remote` resolve para `node-b`; o endpoint `/owner` mostra a rota
  direta do owner e o endpoint `/ws` em `:8080` faz relay binario ate
  `ws://127.0.0.1:9090/ws`.
- O edge nao materializa rooms remotos localmente; ele apenas resolve owner e
  tunela os frames ate o servidor owner.

## Limite intencional

O relay continua sendo um tunel fino de WebSocket:

- nao existe replicacao inter-node fora do owner real;
- nao existe migracao de sessoes durante handoff;
- discovery de owners ainda vem de `PlacementStore + LeaseStore`.

Mesmo assim, o exemplo ja cobre o caminho que faltava na borda:
resolver owner com `pkg/ycluster`, impedir materializacao no no errado e
encaminhar a sessao ao owner remoto.
