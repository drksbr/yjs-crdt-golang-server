# examples/owner-aware-http-edge

Este exemplo mostra como preparar uma borda HTTP/WebSocket com decisao de
ownership usando apenas as APIs publicas atuais:

- `pkg/storage/memory` como `DistributedStore`;
- `pkg/ycluster.DeterministicShardResolver`;
- `pkg/ycluster.StorageLeaseStore`;
- `pkg/ycluster.StorageOwnerLookup`;
- `pkg/yhttp.Server` + `pkg/yprotocol.Provider` para o owner local.

O exemplo semeia dois documentos:

- um documento owned localmente por `node-a`, que abre normalmente em `/ws`;
- um documento owned por `node-b`, que retorna metadados de roteamento em vez
  de materializar o room localmente.

## O que ele demonstra

1. Persistencia de `document -> shard` com `PlacementRecord`.
2. Persistencia de ownership por shard com `LeaseStore`.
3. Lookup de owner antes do upgrade WebSocket.
4. Delegacao ao `pkg/yhttp` apenas quando o owner resolvido e local.
5. Resposta de "route hint" para documento remoto enquanto forwarding inter-node
   ainda nao existe no repo.

## Como executar

```bash
cd examples/owner-aware-http-edge
go run .
```

O servidor sobe em `http://127.0.0.1:8080`.

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
- `notes-remote` resolve para `node-b` e recebe `421 Misdirected Request` com
  `X-Yjs-Owner-Node`, `X-Yjs-Owner-Websocket` e payload JSON indicando para
  onde o edge encaminharia o cliente em uma fase posterior.

## Limite intencional

O repo ainda nao publica transporte entre nos, handoff, cutover nem forwarding
de frames. Por isso o exemplo para na fronteira que ja existe hoje:
resolver owner com `pkg/ycluster` e impedir que um no nao-owner materialize o
room localmente.
