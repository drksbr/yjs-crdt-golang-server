# docs/yjs-go-port-context.md

## Resumo executivo

Este projeto existe para construir, em Go puro, uma camada de comunicação e compatibilidade com Yjs que possa servir de base para um servidor compatível com o ecossistema Yjs/YHub.

A ideia central não é criar um CRDT novo.  
A ideia é entender como o Yjs opera no lado do servidor e portar, de forma incremental e testável, as partes necessárias para compatibilidade real.

A análise feita até aqui mostrou que o servidor do YHub não é apenas um relay de WebSocket.  
Ele monta e manipula estado binário de documento no servidor, usa funções do runtime Yjs para merge, state vector, content ids e recursos avançados como rollback/activity/changeset.

---

## Objetivo deste documento

Registrar, de forma consolidada, o entendimento arquitetural obtido sobre:

- o papel do servidor no ecossistema Yjs/YHub
- o que fica no cliente e o que fica no backend
- o que o YHub faz de fato
- por que um port em Go é viável
- quais partes do Yjs precisam ser portadas primeiro

Este documento é um contexto operacional para agentes e desenvolvedores.

---

## Entendimento consolidado sobre Yjs

### 1. O motor principal de convergência fica no cliente
No Yjs, a lógica principal de merge e convergência do CRDT vive no cliente.

Cada cliente:
- mantém sua cópia do documento
- aplica updates localmente
- gera novos updates
- converge com os demais peers

O servidor não precisa ser, por definição, o resolvedor central de conflitos.

### 2. O servidor não é inútil
Mesmo que o merge conceitual esteja no cliente, o servidor continua importante para:

- relay de updates
- persistência
- sync inicial
- controle de acesso
- awareness/presença
- compactação
- features operacionais

### 3. Um backend mais sofisticado precisa entender updates
No caso do YHub, o servidor vai além do relay:
- ele reconstrói estado de documento
- calcula state vector
- serve documento consolidado
- executa operações analíticas e operacionais

Isso exige entendimento do formato binário do Yjs.

---

## O que foi entendido sobre o papel do servidor

### Modelo mínimo
Um servidor Yjs simples pode atuar como:
- WebSocket relay
- broadcast de awareness
- persistência de blobs de update

Esse modelo já é útil.

### Modelo YHub
O YHub faz mais do que isso:
- usa Redis como barramento/cache
- usa storage persistente
- monta o estado atual do documento no servidor
- compacta dados
- oferece endpoints REST operacionais
- calcula activity / changeset / rollback

Isso implica uso server-side de funções do runtime Yjs.

---

## Conclusão arquitetural principal

É plenamente possível construir um servidor em Go para o ecossistema Yjs.

Mas há dois níveis diferentes de ambição:

### Nível 1 — servidor compatível como infraestrutura
Go é excelente para:
- WebSocket
- rooms
- auth
- Redis
- workers
- persistência
- awareness
- escalabilidade

### Nível 2 — compatibilidade binária e operacional profunda
Para replicar o comportamento do YHub, o backend Go também precisará:
- interpretar updates Yjs
- extrair state vectors
- trabalhar com content ids
- fazer merge binário de updates
- futuramente lidar com attribution/content maps

Esse é o núcleo que este projeto pretende portar.

## Corte distribuído atual

Com as fundações públicas de `pkg/storage`, `pkg/ycluster`, `pkg/ynodeproto`,
`pkg/yprotocol` e `pkg/yhttp` já expostas, o corte atual da fase distribuída
já ligou três superfícies:

- `pkg/ynodeproto` continua como wire interno do cluster e já sustenta mensagens tipadas por classe semântica;
- `pkg/yprotocol.Provider` continua como runtime local do owner e já faz bootstrap/recovery a partir de snapshot base + replay do tail do `update log`;
- `pkg/yhttp` já opera em modo edge owner-aware: qualquer nó aceita HTTP/WS, autentica e resolve owner, mas só o owner local materializa `Session`/`Provider`.

Essa separação preserva dois wires distintos:

- `y-protocols` no perímetro cliente;
- `ynodeproto` entre nós, para forwarding, recovery, handoff e keepalive.

Também preserva a distinção entre estado durável e efêmero:

- documento autoritativo recuperado por `snapshot + update log`;
- awareness mantido como presença efêmera, fora do recovery persistido.

O próximo bloco não reabre esse corte; ele fecha forwarding remoto, handoff,
failover e fencing por epoch sobre essas mesmas superfícies.

---

## Descobertas centrais sobre o YHub

### O YHub monta o documento no servidor
A função central observada foi `getDoc(...)`.

Essa função:
- lê o estado persistido
- lê mensagens recentes do Redis
- filtra mensagens novas
- mescla updates
- produz documento consolidado
- produz awareness consolidado
- trabalha com `contentids` e `contentmap`

Isso mostra que o YHub usa o servidor como ponto de reconstrução operacional do estado atual do documento.

### O sync inicial depende disso
Na abertura do WebSocket, o YHub:
- obtém o documento consolidado
- calcula state vector a partir dele
- envia `SyncStep1`
- envia `SyncStep2`
- envia awareness

Logo, o sync inicial não é apenas replay cego de eventos.

### Features avançadas também dependem disso
As rotas REST do YHub usam estado consolidado e utilitários do Yjs para:
- recuperar documento
- aplicar patch
- rollback
- changeset
- activity

---

## Funções do Yjs já identificadas como relevantes

### Núcleo mais imediato
- `encodeStateVectorFromUpdate`
- `createContentIdsFromUpdate`
- `mergeUpdates`
- `IdSet`
- operações de merge e interseção por ranges
- protocolo de sync
- awareness protocol

### Núcleo avançado
- `mergeContentIds`
- `mergeContentMaps`
- `excludeContentMap`
- `createContentMapFromContentIds`
- attribution manager

---

## O que foi aprendido sobre o código do Yjs

### `encodeStateVectorFromUpdate`
Essa função percorre structs do update binário e calcula, por client, o maior clock contínuo conhecido.

### `createContentIdsFromUpdate`
Essa função extrai:
- inserts
- deletes

a partir do update binário e do delete set.

### `mergeUpdates`
Faz parte da camada binária central do Yjs e depende de:
- leitura lazy de structs
- escrita lazy
- manipulação de `Item`, `GC`, `Skip`
- delete sets
- ordenação por client/clock

### `IdSet`
Modela ranges por client id, servindo de base para:
- content ids
- merges
- interseções
- exclusões

### Attribution/content map
Essa camada é usada pelo YHub para auditoria, activity, rollback e changeset.  
Ela é uma fase posterior do port.

---

## Decisão estratégica do projeto

A estratégia definida é:

### Primeiro portar o que não depende de materialização completa de `Y.Doc`
Começar por:
- utilitários binários
- varint
- parser de update
- state vector
- content ids
- protocolo de sync
- awareness

### Depois portar operações binárias mais complexas
Seguir com:
- merge de updates
- diff
- lazy writer

### Só então avançar para content map e attribution
Isso reduz risco e aumenta verificabilidade.

---

## Por que Go faz sentido aqui

Go é uma boa escolha porque o problema do backend é majoritariamente de:

- rede
- concorrência
- parse binário
- persistência
- workers
- throughput
- controle de memória
- infraestrutura de servidor

Além disso, o port do núcleo binário do Yjs é viável em Go por ser essencialmente:
- parsing estruturado
- encoding/decoding
- manipulação de intervalos
- merge determinístico

---

## O que este projeto não deve fazer cedo demais

- não começar pelo servidor WebSocket completo
- não misturar Redis/Postgres antes de validar núcleo binário
- não tentar portar attribution/content map logo no início
- não declarar compatibilidade sem testes
- não assumir comportamento com base em intuição

---

## Ordem de implementação consolidada

### Etapa 1
- binário
- varint
- erros de parsing

### Etapa 2
- tipos básicos (`ID`, `Item`, `GC`, `Skip`)
- delete set
- leitura suficiente de update

### Etapa 3
- `encodeStateVectorFromUpdate`

### Etapa 4
- `IdSet`

### Etapa 5
- `createContentIdsFromUpdate`

### Etapa 6
- protocolo de sync

### Etapa 7
- awareness

### Etapa 8
- `mergeUpdates`

### Etapa 9
- content maps / attribution

---

## Resultado esperado deste contexto

Após leitura deste documento, o agente deve entender que:

1. o projeto busca compatibilidade real com Yjs/YHub
2. o YHub faz montagem de documento no servidor
3. o núcleo inicial é binário/protocolo, não storage/distribuição
4. o port para Go é viável
5. a implementação deve seguir ordem incremental e testável

---

## Resumo final

O projeto deve ser visto como um port progressivo de capacidades do Yjs no lado servidor.

A meta não é “fazer um servidor colaborativo qualquer”.  
A meta é construir, em Go, uma base compatível com:

- updates binários Yjs
- state vectors
- content ids
- sync protocol
- evolução futura para merge e recursos avançados do YHub

Toda implementação deve ser guiada por código-fonte real, testes e documentação incremental.
