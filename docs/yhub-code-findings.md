# docs/yhub-code-findings.md

## Objetivo

Registrar os achados técnicos obtidos a partir da leitura do código do YHub, com foco em:

- papel real do servidor
- pontos em que o servidor monta ou manipula o documento
- funções do runtime Yjs usadas diretamente pelo backend
- implicações para um port completo em Go

Este documento deve servir como referência de implementação e como base para decisões arquiteturais.

---

## Resumo executivo

O YHub não é apenas um relay de mensagens WebSocket.

O backend:
- recupera estado persistido
- recupera mensagens recentes do Redis
- reconstrói o estado atual do documento
- calcula state vectors
- serve documento consolidado via REST
- aplica patch no documento
- executa rollback
- calcula changeset e activity
- compacta e persiste documentos

Isso significa que um port completo para Go precisa implementar não apenas transporte, mas também parte do núcleo binário e operacional do Yjs usado no servidor.

---

## Arquivos principais analisados

### 1. `bin/yhub.js`
Arquivo de bootstrap/demo.
Mostra a criação de um YHub com:
- Redis
- Postgres
- server
- worker

Conclusão:
- o projeto é arquitetado como servidor + worker
- a infraestrutura é distribuível
- o servidor não é isolado do worker em termos conceituais

---

### 2. `src/index.js`
Arquivo central da classe `YHub`.

Pontos principais:
- inicializa stream, persistence e compute pool
- contém a função `getDoc(...)`
- contém `unsafePersistDoc(...)`
- orquestra worker de compactação
- inicializa servidor

Este é o arquivo mais importante para entender montagem e compactação do documento.

---

### 3. `src/server.js`
Arquivo central do servidor HTTP/WebSocket.

Pontos principais:
- autenticação
- endpoints REST
- upgrade WebSocket
- sync inicial
- recebimento de updates e awareness
- emissão de mensagens para stream

Este é o arquivo mais importante para entender protocolo, sync inicial e operações server-side sobre o documento.

---

## Achado central 1 — o documento é montado no servidor

A função mais importante para isso é:

- `getDoc(room, includeContent, { gcOnMerge })`

Essa função faz, em alto nível:

1. recuperar do storage persistente:
   - `gcDoc`
   - `nongcDoc`
   - `contentmap`
   - `contentids`
   - `references`
   - `lastClock`

2. recuperar do stream/Redis:
   - mensagens recentes da room
   - `lastClock` do stream

3. mesclar os dados:
   - incluir updates novos ainda não persistidos
   - mesclar `contentids`
   - filtrar atribuições conhecidas
   - consolidar awareness

4. produzir documento consolidado:
   - `gcDoc`
   - `nongcDoc`
   - `contentmap`
   - `contentids`
   - `awareness`
   - `lastClock`

### Conclusão
O YHub efetivamente reconstrói o estado do documento do lado servidor a partir de:
- snapshot persistido
- updates transitórios
- merge binário

---

## Achado central 2 — o sync inicial depende do documento reconstruído

No evento `open` do WebSocket, o servidor:

1. chama `getDoc(...)`
2. escolhe `gcDoc` ou `nongcDoc`
3. se não houver documento, gera update de documento vazio
4. envia:
   - `SyncStep1` usando state vector
   - `SyncStep2` usando o update consolidado
5. envia awareness consolidado
6. só depois assina a room no stream

### Conclusão
O sync inicial não depende apenas de replay de eventos.  
Ele depende de um documento já consolidado no servidor.

Isso é um requisito importante para compatibilidade em Go.

---

## Achado central 3 — o servidor usa diretamente funções do Yjs

A leitura do código mostrou chamadas server-side diretas a funções do runtime Yjs, especialmente em `src/index.js` e `src/server.js`.

### Funções observadas
- `Y.decodeContentMap`
- `Y.decodeContentIds`
- `Y.mergeContentIds`
- `Y.excludeContentMap`
- `Y.createContentIdsFromContentMap`
- `Y.insertIntoIdSet`
- `Y.encodeContentMap`
- `Y.mergeContentMaps`
- `Y.encodeContentIds`
- `Y.createContentIdsFromUpdate`
- `Y.encodeStateAsUpdate(new Y.Doc())`
- `Y.encodeStateVectorFromUpdate(ydoc)`
- `Y.mergeUpdates(...)`

### Conclusão
O backend do YHub depende diretamente do runtime Yjs para:
- merge binário
- metadata estrutural
- state vector
- content ids
- content map

Logo, um port 100% compatível em Go precisa cobrir essa camada.

---

## Achado central 4 — compute pool concentra operações pesadas

O `YHub` cria um `computePool`.

Esse pool é usado para operações como:
- `mergeUpdatesAndGc(...)`
- `mergeUpdates(...)`
- `patchYdoc(...)`
- `rollback(...)`
- `changeset(...)`
- `activity(...)`

### Conclusão
Existe uma distinção clara entre:
- camada de transporte/servidor
- camada de computação sobre updates/documentos

Para o port em Go, essa separação deve ser preservada.

---

## Achado central 5 — o worker não é opcional no desenho completo

O worker:
- reclama tasks do stream
- executa compactação
- chama `getDoc(...)`
- persiste resultado consolidado
- limpa referências
- faz trim do Redis

### Conclusão
No desenho completo do YHub, compactação e persistência são parte do fluxo normal do sistema.

No port em Go, isso sugere uma separação futura entre:
- gateway/server
- compute/merge engine
- persistence worker

---

## Achado central 6 — awareness é tratada separadamente do documento

O YHub lida com awareness como mensagens separadas:
- tipo `awareness:v1`
- merge por `protocol.mergeAwarenessUpdates(...)`
- broadcast separado do update de documento
- remoção explícita ao desconectar usuário

### Conclusão
Awareness não deve ser misturada com estado persistente principal do documento.

Essa distinção deve existir também no port em Go.

---

## Achado central 7 — updates recebidos do cliente viram mensagens persistíveis

No WebSocket `message`, quando chega update do cliente:

1. o servidor lê a mensagem do protocolo
2. extrai o update binário
3. gera `contentmap` baseado em:
   - `createContentIdsFromUpdate(update)`
   - userid
   - custom attributions
4. adiciona isso ao stream como:
   - `type: 'ydoc:update:v1'`
   - `contentmap`
   - `update`

### Conclusão
O servidor não apenas retransmite bytes.  
Ele enriquece o update com metadata de attribution antes de persistir/distribuir.

---

## Achado central 8 — a API REST também depende do núcleo Yjs

### `GET /ydoc`
Recupera documento consolidado via `getDoc(...)`.

### `PATCH /ydoc`
- recupera documento atual
- roda `patchYdoc(...)`
- gera update resultante
- grava novo update no stream

### `POST /rollback`
- recupera documento e contentmap
- roda `rollback(...)`
- grava update de rollback no stream

### `GET /changeset`
- recupera documento e contentmap
- roda `changeset(...)`

### `GET /activity`
- recupera documento e contentmap
- roda `activity(...)`

### Conclusão
A API operacional do YHub depende de uma engine server-side sobre updates Yjs.

---

## Implicações para o port em Go

## O que pode ser portado quase diretamente
Essas partes são infraestrutura e desenho de sistema:

- WebSocket server
- REST server
- auth
- rooms
- Redis streams
- worker queue
- Postgres/S3 persistence
- awareness relay
- callbacks
- controle de acesso

---

## O que exige port algorítmico
Essas partes precisam de implementação compatível com o Yjs:

- parser de update
- state vector a partir de update
- content ids
- merge de updates
- lazy read/write de structs
- delete sets
- content map
- attribution
- patch/rollback/activity/changeset

---

## Separação de camadas recomendada para Go

### Camada 1 — infraestrutura
Responsável por:
- HTTP/WebSocket
- rooms
- auth
- Redis
- worker orchestration

### Camada 2 — compatibilidade Yjs
Responsável por:
- parsing binário
- updates
- state vectors
- id sets
- merge
- protocolo sync
- awareness

### Camada 3 — recursos avançados YHub
Responsável por:
- attribution
- content maps
- patch
- rollback
- activity
- changeset

---

## Decisões derivadas desta análise

### Decisão 1
O projeto não deve começar pelo servidor completo.

### Decisão 2
O primeiro foco deve ser o núcleo binário e de protocolo.

### Decisão 3
A separação entre infraestrutura e engine de compatibilidade deve existir desde cedo.

### Decisão 4
Compatibilidade total com YHub exige portar também a camada avançada de metadata, não apenas `mergeUpdates`.

---

## Checklist de achados já confirmados

- [x] O YHub reconstrói documento no servidor
- [x] O sync inicial depende desse documento
- [x] O servidor usa funções diretas do runtime Yjs
- [x] Awareness é tratada separadamente
- [x] Worker participa da compactação e persistência
- [x] A API REST depende da engine server-side
- [x] O port em Go é viável, mas exige núcleo binário compatível

---

## Resumo final

A leitura do código do YHub confirma que:

1. o servidor faz trabalho real sobre o documento
2. o documento consolidado é uma peça central do sistema
3. o runtime Yjs é usado diretamente no backend
4. um port completo em Go precisa implementar:
   - parsing binário
   - state vectors
   - content ids
   - merge de updates
   - protocolo sync
   - awareness
   - futuramente attribution/content map

O caminho correto para o projeto é portar primeiro o núcleo binário/protocolo e só depois avançar para a infraestrutura completa e para os recursos avançados.
