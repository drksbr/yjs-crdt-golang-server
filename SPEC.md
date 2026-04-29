# SPEC.md

## Visão geral

Este projeto tem como objetivo construir, em Go puro, uma camada de comunicação e compatibilidade com Yjs que possa servir de base para um servidor compatível com o ecossistema Yjs/YHub.

O foco não é criar um editor colaborativo nem uma abstração genérica de CRDT.  
O foco é implementar compatibilidade técnica com os formatos, protocolos e operações binárias relevantes do Yjs, começando pelo núcleo mínimo necessário para sync e manipulação de updates.

---

## Objetivo principal

Implementar uma base em Go que permita:

- interpretar updates binários produzidos por clientes Yjs
- extrair state vectors a partir de updates
- extrair content ids a partir de updates
- implementar o protocolo de sync básico compatível com Yjs
- suportar merge, diff e interseção mínimos sobre updates V1
- preparar a evolução para lazy writer, snapshots binários persistíveis em V1 e funcionalidades avançadas do YHub

---

## Objetivos secundários

- permitir construção futura de um servidor WebSocket compatível com Yjs
- permitir evolução para execução distribuída com owner único por documento/shard
- permitir persistência e recuperação de snapshots binários persistíveis em V1 com API pública e armazenamento operacional
- servir de base para features avançadas como changeset, rollback e activity
- manter independência de runtime Node.js em produção

---

## Não objetivos iniciais

Nesta fase inicial, o projeto não tem como objetivo:

- implementar um editor
- implementar UI
- implementar armazenamento distribuído completo
- implementar todos os recursos avançados do YHub imediatamente
- portar toda a camada de attribution/content map na primeira etapa
- reimplementar todo o Yjs de uma só vez

---

## Contexto técnico

A análise do código do YHub mostrou que o servidor não atua apenas como relay de mensagens.

O YHub monta o documento no servidor, principalmente por meio de `getDoc(...)`, juntando:

- estado persistido
- mensagens recentes do Redis
- merge binário de updates
- estruturas auxiliares como `contentids` e `contentmap`

O sync inicial do WebSocket depende desse estado consolidado para enviar:

- `SyncStep1` com state vector
- `SyncStep2` com o documento codificado como update

Além disso, APIs como `GET /ydoc`, `PATCH /ydoc`, `rollback`, `activity` e `changeset` dependem de operações server-side sobre updates Yjs.

Logo, um backend Go totalmente compatível com o ecossistema Yjs/YHub precisa implementar mais do que transporte de WebSocket.  
Ele precisa compreender e operar sobre os updates binários e sobre partes do modelo de dados usado pelo Yjs no lado do servidor.

---

## Princípios arquiteturais

### 1. Compatibilidade incremental
A compatibilidade será construída por camadas, começando no núcleo binário e evoluindo para recursos mais complexos.

### 2. Separação de responsabilidades
O projeto deve separar claramente:
- utilitários binários
- parsing/encoding
- tipos estruturais
- id sets
- protocolo de sync
- awareness
- compatibilidade de alto nível

### 3. Testabilidade
Cada bloco deve ser verificável isoladamente com testes unitários e round-trip tests quando aplicável.

### 4. Determinismo
Toda transformação binária ou estrutural deve ser determinística.

### 5. Go puro
A solução deve rodar em Go puro no ambiente de produção.

---

## Escopo por fases

### Status de execução

Projeto em **Fase 3 (Meta técnica 9)**, com a **Fase 4 distribuída (Meta técnica 10)** aberta no roadmap.

- Metas 1 a 8 já foram consolidadas no corte mínimo.
- Meta técnica 9 está em execução com foco em consolidar a API pública de update em `pkg/yjsbridge` em V1, além de snapshots V1 e persistência operacional, da exposição pública de sync/awareness em V1 (`pkg/yprotocol` e `pkg/yawareness`), do runtime in-process mínimo de protocolo em `pkg/yprotocol` e da camada mínima de provider acima de `Session`, ainda em escopo single-process.
- Existe um ciclo funcional público com `pkg/yjsbridge` expondo `PersistedSnapshot` e utilitários de conversão/codificação.
- A hidratação reversa de `PersistedSnapshot` está operacionalizada com stores persistentes em `pkg/storage` (memória e Postgres).
- O branch atual já entrega os cortes operacionais iniciais da fase distribuída: contratos de `snapshot + update log`/placement/lease em `pkg/storage`, backends concretos em memória/Postgres, helpers públicos de replay/recovery, control plane storage-backed mínimo em `pkg/ycluster` já com coordenador por documento, mensagens inter-node tipadas em `pkg/ynodeproto`, bootstrap/recovery do owner local em `pkg/yprotocol.Provider`, borda owner-aware em `pkg/yhttp`, seam typed de forwarding remoto, lifecycle de lease endurecido com `epoch` monotônico, owner lookup dependente de lease ativa e uma fronteira pública de fencing autoritativo no storage para append/persist/trim sob `AuthorityFence`.
- O próximo ciclo passa a preparar a arquitetura distribuída autoritativa: owner único por documento/shard com lease/epoch/fencing consistentes, forwarding edge->owner pelo wire inter-node e rebalance/failover usando a primitiva de handoff atômico já definida.

## Fase 1 — núcleo mínimo compatível

Status: **concluída no corte mínimo**.

### Entregáveis
- utilitários binários
- varint encode/decode
- decoder básico de update
- tipos mínimos de struct
- `encodeStateVectorFromUpdate`
- `createContentIdsFromUpdate`
- `IdSet` básico
- wire format do protocolo de sync
- wire format de awareness

### Resultado esperado
Capacidade de:
- consumir updates Yjs
- extrair metadados estruturais úteis
- participar do sync básico no protocolo Yjs

---

## Fase 2 — operações binárias de documento

Status: **em execução (Meta 9)**.

### Entregáveis
- endurecimento de compatibilidade de `merge/diff/intersect`
- lazy writer
- merge incremental de updates
- compatibilidade V2 e conversões de formato
- corte funcional de snapshots binários persistidos V1
- preparação para armazenamento operacional de snapshots

### Resultado esperado
Capacidade de:
- consolidar documento binário com menos gaps de compatibilidade
- responder syncs iniciais com consistência estrutural maior
- preparar backend para persistência e compaction

### Corte funcional V1
- `ConvertUpdateToV1` e `ConvertUpdatesToV1` normalizam payload para V1 canônico com validação agregada.
- `PersistedSnapshotFromUpdate`, `PersistedSnapshotFromUpdates` e `PersistedSnapshotFromUpdatesContext` materializam um snapshot persistível em memória, com:
  - `UpdateV1` canônico consolidado
  - `StateVector` e `DeleteSet` derivados em `Snapshot`
- `SnapshotFromUpdate`/`SnapshotFromUpdates` permanece cobrindo extração de estado e delete set para V1; V2 ainda não possui materialização.
- `EncodePersistedSnapshotV1`, `DecodePersistedSnapshotV1` e `DecodePersistedSnapshotV1Context` fecham o ciclo mínimo de persistência/restore em V1; V2 ainda não possui materialização nem restore.

---

## Fase 3 — recursos compatíveis com YHub

Status: **em execução (promoção da API pública de update em `pkg/yjsbridge`, protocolo em `pkg/yprotocol`, awareness em `pkg/yawareness`, runtime in-process mínimo em `pkg/yprotocol` e camada mínima de provider no mesmo pacote)**.

### Entregáveis
- content ids avançado
- content maps
- attribution layer
- rollback
- activity
- changeset
- recursos auxiliares para auditoria e histórico
- exposição estável de `merge/diff/intersect`/`state vector`/`content ids` em `pkg/yjsbridge` em V1 (sem suporte V2)
- exposição estável da superfície de protocolo sync em `pkg/yprotocol` para `SyncStep1`, `SyncStep2` e envelope de mensagens websocket, em V1 (sem provider completo e sem suporte V2)
- runtime in-process mínimo em `pkg/yprotocol` para composição local de sessão/protocolo com `Session`, `HandleProtocolMessage`, `HandleEncodedMessages`, `HandleEncodedMessagesContext` no provider e encode público de `ProtocolMessage`, ainda sem provider completo e sem suporte V2
- camada mínima de provider em `pkg/yprotocol` com `Provider`, `Open`, `Connection`, `DispatchResult`, `Persist` e `Close`, ainda sem provider completo, sem transporte distribuído e sem suporte V2
- exposição estável da superfície awareness em `pkg/yawareness` para wire format e runtime básico em V1 (sem provider completo e sem suporte V2)

### Resultado esperado
Capacidade de:
- aproximar compatibilidade funcional com o YHub
- permitir composição de um servidor single-process ou de referência em cima da API pública, incluindo um provider mínimo em processo, sem ainda assumir provider completo
- suportar recursos analíticos e operacionais avançados

---

## Fase 4 — arquitetura distribuída por ownership

Status: **em execução, com fundações distribuídas operacionais, bootstrap owner-aware por `snapshot + update log` e um slice de fencing autoritativo na fronteira de storage já expostos (Meta técnica 10)**.

### Entregáveis
- owner único por documento/shard lógico
- lease renovável e revogável para ownership do room
- propagação de `epoch` monotônico e fencing token por toda operação autoritativa, começando por append/persist/trim no storage
- modelo `snapshot + update log` para hidratação, replay, recovery e handoff
- bootstrap do owner em `pkg/yprotocol.Provider` a partir de snapshot base + replay do tail do log, com offset/high-water mark observável para checkpoint e handoff
- camada de mensagens inter-node tipadas e versionadas acima do framing de `pkg/ynodeproto`, separada do `y-protocols`, para handshake, forwarding, hydrate/handoff e recuperação
- modo edge owner-aware em `pkg/yhttp`: qualquer nó aceita HTTP/WS, autentica e resolve owner, mas só o owner local materializa o room
- handoff seguro com bootstrap por snapshot, replay do tail do log e corte atômico por epoch
- observabilidade para roteamento, lease, forwarding, replay e troca de owner, com dashboard/alertas de referência

### Resultado esperado
Capacidade de:
- manter um único runtime autoritativo por documento/shard mesmo em cluster multi-nó
- aceitar clientes e requests HTTP/WS em qualquer nó sem duplicar processamento do room
- recuperar ou promover owner com replay determinístico de `snapshot + update log`, preservando offsets observáveis para catch-up e checkpoint
- separar explicitamente o wire de cliente (`y-protocols`) do wire inter-node tipado consumido por edge/owner
- impedir split-brain e escrita obsoleta via `epoch` monotônico e fencing propagados do storage ao runtime/handoff

### Cortes já entregues

Antes do runtime distribuído completo, o repositório já publicou os contratos
que vão sustentar a próxima etapa:

- `pkg/storage` já separa `SnapshotStore` do scaffolding distribuído (`UpdateLogStore`, `PlacementStore`, `LeaseStore`, `DistributedStore`) e dos registros `UpdateLogRecord`, `PlacementRecord`, `LeaseRecord` e `OwnerInfo`;
- `pkg/storage` agora também expõe a fronteira pública de fencing autoritativo com `AuthorityFence`, `AuthoritativeUpdateLogStore`, `AuthoritativeSnapshotStore` e `ErrAuthorityLost`;
- `pkg/storage` também já expõe `ReplaySnapshot`, `RecoverSnapshot`, `ReplayUpdateLog`, `CompactUpdateLog` e `CompactUpdateLogAuthoritative` para reconstrução pública via `snapshot + update log` com compaction fenced quando há owner ativo;
- `pkg/storage/memory` e `pkg/storage/postgres` já materializam esses contratos distribuídos de snapshot, update log, placement e lease, com `OwnerInfo.Epoch` obrigatório, `ErrLeaseConflict`/`ErrLeaseStaleEpoch`, preservação da última geração após release, `LeaseHandoffStore` para troca atômica de owner/epoch e validação de placement + lease + token + expiração para append/persist/trim autoritativos;
- `pkg/ycluster` já expõe tipos estáveis de cluster, `DeterministicShardResolver`, `StaticLocalNode`, `PlacementOwnerLookup`, `StorageOwnerLookup`, `StorageLeaseStore`, `LeaseManager` com loop autônomo de renovação, `StorageOwnershipCoordinator`, `DocumentOwnershipRuntime`, métricas opcionais e interfaces mínimas de `Runtime`, resolvendo owner apenas a partir de lease ativa e válida e já compondo claim/promoção/handoff/lookup/fence/execução storage-backed compartilhada por documento;
- `pkg/ynodeproto` já expõe o framing binário versionado do wire inter-node e payloads tipados para handshake/ack com `clientID`, sync, document update, awareness, `query-awareness`, `disconnect`, `close` e ping/pong;
- `pkg/yprotocol.Provider` já atua como runtime local de referência do owner, com bootstrap/recovery via `snapshot + update log`, caminho fenced e context-aware em `apply`/persist/cutover, checkpoint/high-water mark persistidos com metadata de `epoch` e hooks opcionais de observabilidade para persistência/revalidação/perda de autoridade;
- `pkg/yhttp` já expõe `OwnerAwareServer` como borda pública HTTP/WebSocket para resolver owner antes do provider local, além de promoção local opt-in quando não há owner ativo, integração opcional com `DocumentOwnershipRuntime` no `Server` local, endpoint owner-side e takeover `remote -> local`, seam typed de forwarding remoto via `RemoteOwnerDialer`/`NodeMessageStream`, hook de autenticação e validação de epoch do handshake inter-node owner-side e observabilidade opcional para lookup de owner, decisão de rota e relay remoto;
- `pkg/storage`, `pkg/yprotocol` e `pkg/ycluster` já também expõem adapters opcionais de observabilidade com labels constantes para replay/recovery/compaction, offsets, lag de tail, epoch observado, lifecycle local do owner e control plane de lease/owner lookup; `examples/owner-aware-http-edge/observability` entrega alertas Prometheus e dashboards Grafana operacional/oráculo de referência; a próxima etapa concentra rebalance acima do runtime local atual (`LeaseManager`/`StorageOwnershipCoordinator`/`DocumentOwnershipRuntime`) e coordenação final do lifecycle por `epoch`.

---

## Referências técnicas prioritárias

As referências mais importantes deste projeto são:

### Código e docs
- código-fonte do Yjs
- código-fonte do YHub
- protocolo oficial `y-protocols`

### Funções prioritárias do Yjs
- `encodeStateVectorFromUpdate`
- `createContentIdsFromUpdate`
- `mergeUpdates`
- operações sobre `IdSet`
- protocolo de sync
- awareness protocol
- `SyncStep1`, `SyncStep2` e envelope de mensagem de sync/awareness

### Funções de fases posteriores
- `mergeContentIds`
- `mergeContentMaps`
- `excludeContentMap`
- `createContentMapFromContentIds`
- attribution manager

---

## Capacidades esperadas do núcleo em Go

A base em Go deverá ser capaz de representar, no mínimo:

- updates binários
- state vectors
- id ranges
- delete sets
- structs do update
- mensagens do protocolo de sync
- mensagens de awareness

---

## Modelo conceitual do domínio

### Update
Bloco binário que representa mudanças no documento Yjs.

### Struct
Unidade estrutural lida do update.  
No núcleo inicial, deve contemplar pelo menos:
- `Item`
- `GC`
- `Skip`

### State Vector
Resumo por cliente do maior clock contínuo conhecido.

### Content IDs
Conjunto de ranges identificando conteúdo inserido e conteúdo deletado.

### IdSet
Estrutura usada para representar ranges por client id.

### Delete Set
Conjunto de intervalos removidos representado no update.

### Sync Message
Mensagem do protocolo de sincronização usada pelo Yjs.

### Awareness Message
Mensagem efêmera de presença/estado de usuário.

### Room / Shard
Unidade lógica de roteamento e ownership usada para decidir onde o documento é processado.

### Owner
Nó que materializa o runtime autoritativo do room e executa sync, awareness, merge/persistência e fanout.

### Lease
Concessão temporária que autoriza um nó a atuar como owner de um room.

### Epoch / Fencing
Versão monotônica do lease usada para invalidar owners antigos e cercar operações autoritativas.

### Update Log
Sequência append-only de updates aplicada sobre um snapshot base para replay, recovery e handoff.

### Inter-node Message
Mensagem interna do cluster transportada por `pkg/ynodeproto`, separada do
`y-protocols` de cliente e tipada por classe semântica (`handshake`,
`document-sync-*`, `document-update`, `awareness-update`, `ping/pong`).

### Edge Node
Nó que aceita HTTP/WS publicamente, autentica a request e resolve owner, mas
não materializa `Session`/`Provider` do room quando não detém a ownership local.

---

## Restrições de implementação

- não usar Node.js em runtime
- não criar arquivos com mais de 300 linhas
- evitar dependências desnecessárias
- validar todos os dados binários de entrada
- evitar panics em parsing de input externo
- manter API interna pequena e coesa

---

## Estratégia de validação

A compatibilidade deve ser validada por:

### 1. Testes de parsing
Verificar leitura correta de updates válidos e rejeição segura de updates inválidos.

### 2. Testes de round-trip
Quando aplicável, codificar e decodificar deve preservar significado estrutural.

### 3. Testes cruzados com vetores conhecidos
Comparar resultados com comportamento esperado do Yjs.

### 4. Testes de invariantes
Exemplos:
- state vector consistente
- ranges ordenados ou normalizados
- delete set bem formado
- merge determinístico

---

## Critério de pronto por fase

### Fase 1 pronta quando:
- [x] updates podem ser lidos no nível necessário
- [x] state vector pode ser extraído de update
- [x] content ids podem ser extraídos de update
- [x] protocolo básico de sync está implementado
- [x] awareness wire format está implementado
- [x] merge/diff/interseção mínimos sobre update V1 existem com testes
- [x] há testes cobrindo os principais casos

### Fase 2 pronta quando:
- [x] múltiplos updates podem ser mesclados em cenários amplos
- [x] estado consolidado pode ser produzido e divergências estruturais conhecidas são registradas
- [ ] writer lazy com compatibilidade estrutural suficiente para o escopo da Fase 2
- [x] corte funcional V1 de snapshot persistido disponível (`PersistedSnapshotFromUpdate(s)` com `UpdateV1` canônico)
- [x] ciclo de hidratação reversa/restore de `PersistedSnapshot` V1 está encapsulado e validado para storage operacional
- [ ] compatibilidade V2 e conversão de formato possuem corte funcional verificável
- [x] integração de persistência operacional de snapshot está implementada

### Fase 3 pronta quando:
- attribution/content map funcionam de forma verificável
- endpoints e recursos equivalentes ao YHub podem ser sustentados por essa base

### Fase 4 pronta quando:
- qualquer nó pode aceitar HTTP e WebSocket para um room em modo edge owner-aware
- apenas um owner ativo por documento/shard processa o room por vez
- mensagens inter-node tipadas e versionadas cobrem handshake, forward, recovery e handoff sem reaproveitar `y-protocols`
- lease, `epoch` e fencing de storage/runtime evitam split-brain e escrita obsoleta
- `snapshot + update log` permitem bootstrap, replay e handoff previsíveis, preservando offset/high-water mark observável
- protocolo inter-node próprio sustenta forwarding, recovery e troca de owner

## Backlog imediato da transição Fase 3 -> Fase 4

1. Consolidar cenários de `merge/diff/intersect` com composição estrutural mais rica.
2. Ampliar e endurecer a integração do lazy writer no fluxo de atualização.
3. Concluir o mapa de lacunas de compatibilidade para V2 e conversões de formato.
4. Ligar `AuthorityFence`/`ResolveAuthorityFence` ao caminho autoritativo do provider, incluindo `apply`, persist, recovery e respostas de handoff/cutover.
5. Evoluir failover/rebalance end-to-end em cima do bootstrap/recovery já operacional por `snapshot + update log` e da troca atômica de `epoch` já definida no storage/control plane.
6. Conectar o wire tipado já exposto em `pkg/ynodeproto` ao forwarding/cutover end-to-end entre edge e owner, reduzindo seams ad hoc.
7. Evoluir o bundle de observabilidade para ambientes reais multi-nó, com labels de deployment e SLOs por tenant quando houver multi-tenancy.
8. Atualizar continuamente os documentos principais conforme novas divergências ou invariantes distribuídas forem observadas.

---

## Organização inicial sugerida

A estrutura de pacotes em operação agora é:

```text
internal/
  binary/
  varint/
  ytypes/
  yupdate/
  yidset/
  yprotocol/
  yawareness/
  ycompat/
pkg/
  yjsbridge/
  ycluster/
  ynodeproto/
  yprotocol/
  yawareness/
  storage/
    memory/
    postgres/
```

Pacotes públicos em `pkg/` já estão ativos para snapshots (`pkg/yjsbridge`), sync (`pkg/yprotocol`), awareness (`pkg/yawareness`), storage (`pkg/storage`) e para o scaffolding inicial da fase distribuída (`pkg/ycluster` e `pkg/ynodeproto`).

## Decisões arquiteturais iniciais

### Decisão 1
Começar por compatibilidade de leitura e protocolo, não por servidor completo.

### Decisão 2
Implementar primeiro funções deriváveis diretamente de updates sem exigir materialização completa de `Y.Doc`.

### Decisão 3
Tratar content maps e attribution como etapa posterior.

### Decisão 4
Manter a implementação preparada para uso futuro em servidor estilo YHub, com persistência operacional limitada em `pkg/storage` para `PersistedSnapshot`, exposição pública de sync/awareness em V1 e usando o provider local atual como embrião do futuro owner distribuído, sem amarrar o núcleo `internal/` a uma estratégia específica de coordenação multi-nó antes da hora.

### Decisão 5
Publicar cedo os contratos de persistência distribuída, control plane e framing inter-node, além da fronteira pública de fencing autoritativo e handoff atômico em storage, para congelar a superfície pública sem quebrar o modo single-process já operacional.

## Riscos técnicos conhecidos

### 1. Compatibilidade binária incompleta
Pequenas divergências em parsing ou encoding podem quebrar compatibilidade.

### 2. Complexidade do merge
`mergeUpdates` e as operações de slice/diff/interseção são significativamente mais complexos que extração de state vector.

### 3. Diferenças entre formatos/versionamento
O Yjs possui detalhes de encoding que exigem leitura fiel do código.

### 4. Camada de attribution
Essa parte é mais rica e menos trivial do que o núcleo binário.

### 5. Coordenação distribuída
Lease, `epoch`, fencing, replay de log e handoff de owner aumentam o risco de split-brain e inconsistência operacional.

## Mitigação dos riscos

- implementar incrementalmente
- manter testes pequenos e precisos
- validar contra comportamento observado no código-fonte
- documentar toda hipótese não confirmada
- evitar pular etapas do núcleo
- cercar transições de owner com invariantes explícitas de lease, `epoch` e replay

## Critério de sucesso do projeto

O projeto será bem-sucedido se conseguir, de forma incremental e testável:

- compreender updates Yjs em Go
- participar corretamente do protocolo básico de sync
- sustentar merge/diff/interseção mínimos e evoluir a compatibilidade de documento consolidado
- evoluir para operação distribuída com owner único por room/documento sem duplicar processamento
- preparar base sólida para features compatíveis com YHub

## Resumo executivo final

Este projeto constrói uma fundação em Go para compatibilidade com Yjs/YHub.

A ordem correta de construção é:

1. binário
2. updates
3. state vectors
4. content ids
5. protocolo sync
6. merge/diff/interseção mínimos
7. lazy writer / snapshots / V2
8. ownership distribuído / `snapshot + update log` / protocolo inter-node
9. attribution/content map
10. funcionalidades avançadas

O projeto deve evoluir por pequenas entregas, sempre testadas, sempre documentadas e sempre guiadas pelo comportamento real do código do Yjs/YHub.
