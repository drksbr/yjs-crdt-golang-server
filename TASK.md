# TASK.md

## Estado atual

Projeto com base documental consolidada e com o núcleo inicial já ultrapassado.

Neste momento o repositório já possui:
- módulo Go inicializado
- pacote `internal/binary` com leitura sequencial segura
- pacote `internal/varint` com encode/decode de `varuint`
- pacote `internal/ytypes` com `ID`, `Item`, `GC`, `Skip` e `DeleteSet` básico
- pacote `internal/yidset` com ranges normalizados por cliente
- pacote `internal/yupdate` com leitura lazy de update V1, decode de delete set, state vector, content ids, reencode V1, merge, diff e interseção por content ids
- pacote `pkg/yprotocol` com API pública de protocolo em V1 (`SyncStep1`, `SyncStep2`, envelope websocket, encoder público de `ProtocolMessage` e helpers de parse/encode), com limites de V2 explícitos
- pacote `internal/yprotocol` com wire format mínimo do sync protocol e suporte interno adicional
- envelope de mensagens websocket para `sync`, `awareness`, `auth` e `queryAwareness` (sem alegar implementação de provider completo)
- helpers de sync inicial a partir de múltiplos updates V1
- pacote `pkg/yawareness` com API pública de awareness runtime/wire em V1 (estado local, proteção do estado local, expiração e heartbeat)
- runtime in-process mínimo em `pkg/yprotocol` com `Session`, `HandleProtocolMessage` e `HandleEncodedMessages` para composição local de sync/awareness sem provider completo
- camada mínima de provider em `pkg/yprotocol` acima de `Session`, com `Provider`, `Open`, `Connection`, `DispatchResult`, `Persist` e `Close`, ainda sem provider completo, transporte distribuído ou V2
- pacote `pkg/yhttp` com handler `net/http` + WebSocket acima de `pkg/yprotocol.Provider`, com fanout local, persistência opcional no close e compatibilidade por adaptação com Gin/Echo/afins
- subpacotes `pkg/yhttp/gin`, `pkg/yhttp/echo` e `pkg/yhttp/chi` com adapters mínimos para acoplamento direto em frameworks HTTP
- hooks opcionais de observabilidade em `pkg/yhttp` via interface `Metrics`, com adapter Prometheus em `pkg/yhttp/prometheus`
- um runtime local por documento em `pkg/yprotocol.Provider` que passa a servir como embrião do futuro owner distribuído
- pacote `internal/yawareness` com wire format mínimo de awareness
- state manager de awareness alinhado em clocks, proteção do estado local e expiração/renovação básica
- cobertura ampliada para round-trip de content refs, metadata wire de `Item` e slicing estrutural de `JSON`/`Any`
- regressão adicional cobrindo fluxo `merge -> diff -> intersect` sobre cadeia de 3 updates fora de ordem
- lazy writer V1 endurecido para ordem encontrada no update, reuso pós-`finish` e preservação de metadata em slices
- classificação mínima de formato já distinguindo cabeçalho V2 e mistura de formatos incompatíveis
- API pública de formato/merge endurecida com validação agregada e erro indexado por update
- APIs públicas de inspeção por `single-update` alinhadas para despacho V1 e rejeição explícita de V2
- `MergeUpdatesV1`/`MergeUpdates` agora expõem variantes com `context.Context` e usam agregação paralela na etapa de fusão
- `DiffUpdate` e `IntersectUpdateWithContentIDs` agora expõem variantes com `context.Context`
- a pré-validação agregada de formato também respeita `context.Context` nas APIs multi-update
- novas regressões de merge cobrindo preenchimento de `Skip` sintético e `Skip` explícito vindo do input
- contratos públicos adicionais cobrindo equivalência/cancelamento das APIs com `context` e bordas V2/context
- APIs derivadas de `state vector` e `content ids` alinhadas à pré-validação agregada de formato
- variantes com `context.Context` para agregação de `state vector` e `content ids`
- erro indexado até o primeiro payload não vazio quando `state vector` e `content ids` detectam V2 ainda não suportado
- leitura de stream endurecida com contratos explícitos de cancelamento quando disponíveis
- limpeza de artefatos temporários de depuração introduzidos em rodadas anteriores
- documentação de roadmap alinhada ao estado real do código
- regressões novas para comutatividade de `merge`, gaps sintéticos, workflow multi-client de `merge -> diff -> intersect`, round-trip do `lazy writer` e contratos V2 das APIs derivadas
- guardas defensivas em helpers internos de merge para listas vazias, evitando `panic` fora do caminho feliz
- contratos adicionais do `lazy writer` cobrindo `finish` vazio e normalização de structs intercaladas em `EncodeV1`
- contratos agregados adicionais de V2 cobrindo rejeição após payloads vazios e precedência de mistura `V1/V2` em `MergeUpdates`
- `EncodeV1` agora normaliza structs por clock dentro de cada cliente antes de serializar
- `MergeUpdates` agora indexa o primeiro payload relevante ao rejeitar V2 detectado, alinhando o diagnóstico às outras APIs agregadas
- cobertura adicional para `sliceStructWindowV1`, refs não fatiáveis em `ParsedContent.SliceWindow` e continuação de merge após `Skip` explícito no overlap
- guards de slice endurecidos com aritmética segura contra overflow em `sliceStructWindowV1` e `ParsedContent.SliceWindow`
- `MergeUpdates` agora trata listas compostas apenas por payloads vazios como `no-op`, retornando update V1 vazio
- cobertura context-aware adicional para payload V1 malformado em `MergeUpdatesV1Context` e `ValidateUpdatesFormatWithReasonContext`
- primeiro corte funcional de `conversion/snapshots` em V1 com `ConvertUpdateToV1`, `ConvertUpdatesToV1`, `SnapshotFromUpdate(s)`, `PersistedSnapshotFromUpdate(s)` e codec explícito `Encode/DecodePersistedSnapshotV1` para estado em memória, bytes canônicos e restore V1
- ciclo de hidratação reversa de `PersistedSnapshot` em V1 com `EncodePersistedSnapshotV1` e `DecodePersistedSnapshotV1` (`context` incluído), sem suporte a V2
- testes cobrindo casos válidos e inválidos
- `pkg/yjsbridge` já expõe API pública mínima para `Snapshot`, `PersistedSnapshot` e operações de persistência V1 (`Convert`, `Encode`, `Decode`)
- `pkg/yjsbridge` também recebeu a promoção da API pública de `update` em V1, com superfície exposta para operações de merge/diff/intersect e utilitários de `state vector`/`content ids`, mantendo `V2` explicitamente não suportado
- `pkg/storage` já define `SnapshotStore`, `DocumentKey`, `SnapshotRecord`, erros de domínio e contratos de `SaveSnapshot`/`LoadSnapshot`
- `pkg/storage` agora também expõe o scaffolding distribuído inicial com `UpdateLogStore`, `PlacementStore`, `LeaseStore`, `DistributedStore`, além de `UpdateLogRecord`, `PlacementRecord`, `LeaseRecord`, `OwnerInfo`, `ShardID`, `NodeID` e `UpdateOffset`
- `pkg/storage` agora também já expõe helpers públicos de replay/recovery (`ReplaySnapshot`, `RecoverSnapshot`) para reconstrução via `snapshot + update log`
- `pkg/ycluster` agora expõe o scaffolding público de control plane com tipos estáveis de `NodeID`/`ShardID`/`Placement`/`Lease`, `DeterministicShardResolver`, `StaticLocalNode`, `PlacementOwnerLookup` e interfaces mínimas de `Runtime`
- `pkg/ycluster` agora também já expõe adapters storage-backed com `StorageOwnerLookup`, `StorageLeaseStore` e conversões `storage <-> cluster`
- `pkg/storage/memory` e `pkg/storage/postgres` agora cercam leases por geração persistida, com `OwnerInfo.Epoch` obrigatório, `ErrLeaseConflict`/`ErrLeaseStaleEpoch` e preservação da última geração após release
- `pkg/ycluster` agora só resolve ownership a partir de lease ativa e válida; placement sozinho não classifica mais owner local/remoto
- `pkg/ynodeproto` agora expõe o framing binário versionado do protocolo inter-node com `Header`, `Frame`, `MessageType`, `Encode/DecodeFrame` e decode por prefixo
- `pkg/ynodeproto` agora também carrega `clientID` em `handshake`/`handshake-ack` e mensagens roteadas para `query-awareness`, `disconnect` e `close`
- `pkg/storage/memory` e `pkg/storage/postgres` já implementam stores operacionais de snapshots com persistência canônica em V1
- `pkg/storage/memory` e `pkg/storage/postgres` agora também já implementam `DistributedStore` com update log, placement e lease
- `pkg/yhttp` agora expõe um forwarder remoto typed plugável em `OwnerAwareServerConfig.OnRemoteOwner`, baseado em `RemoteOwnerDialer`/`NodeMessageStream`
- `examples/memory` e `examples/postgres` foram adicionados com fluxos iniciais de persistência usando a API pública e stores referenciados
- `examples/provider-memory` agora cobre fluxo local de provider com late joiner, persistência explícita e restore em novo provider, servindo como referência do recovery por snapshot antes do replay distribuído
- `examples/http-memory` foi adicionado como exemplo de transporte `net/http` + WebSocket com `pkg/yhttp`
- `examples/gin-memory`, `examples/echo-memory` e `examples/chi-memory` foram adicionados como exemplos de acoplamento por adapters específicos de framework
- `examples/http-postgres`, `examples/gin-postgres`, `examples/echo-postgres` e `examples/chi-postgres` foram adicionados como variantes com persistência PostgreSQL para o fluxo WebSocket
- `examples/gin-react-tailwind-postgres` foi adicionado como demo full-stack com `vite` + `react` + `tailwindcss`, backend `gin`, persistência PostgreSQL e editor colaborativo com awareness
- `examples/owner-aware-http-edge` agora sobe um owner remoto real e demonstra relay de tráfego WebSocket entre edge e owner
- pacote `integration` adiciona smoke tests opt-in com Postgres efêmero via Docker para funcionalidade, persistência e performance básica do fluxo WebSocket
- pacote `integration` também adiciona matriz opt-in de performance entre `net/http`, `gin`, `echo` e `chi`, cobrindo memória/Postgres e medindo throughput, latência e restore
- `integration/owner_aware_remote_relay_test.go` agora cobre sync e awareness atravessando o relay owner-aware para um owner remoto

A fase atual é **Meta técnica 9 / Fase 3 em consolidação**, com API pública de snapshot e de update em V1 já em operação em `pkg/yjsbridge`, além da exposição pública de protocolo e awareness em `pkg/yprotocol` e `pkg/yawareness` em V1.
As metas técnicas 1, 2, 3, 4, 5, 6, 7 e 8 já possuem corte mínimo implementado.
Em Meta 9, a prioridade continua sendo reduzir lacunas de compatibilidade estrutural em merge/diff/intersect, estabilizar o writer incremental e abrir a entrada controlada para V2 quando a camada pública atual estiver madura.
Paralelamente, a nova etapa registra a consolidação operacional de snapshots V1 por meio de API pública e stores, o runtime in-process público de protocolo em `pkg/yprotocol`, a camada mínima de provider acima de `Session` e a primeira borda pública de transporte HTTP/WebSocket em `pkg/yhttp`, ainda sem transporte distribuído e sem V2.
A mesma transição também já materializou o primeiro corte operacional da Meta 10: os contratos de persistência distribuída, o framing inter-node, os backends concretos de update log/placement/lease, os helpers públicos de replay e os adapters storage-backed de control plane já estão no branch, mas handoff, cutover e coordenação multi-nó ainda permanecem fora do runtime funcional.
A próxima fase aberta no roadmap é a **Meta técnica 10 / Fase 4**, que introduz owner único por documento/shard, lease/epoch/fencing, `snapshot + update log`, protocolo inter-node próprio e aceite de HTTP/WS em qualquer nó com processamento do room restrito ao owner.

### Corte provável do próximo epoch

- materializar o endpoint owner-side concreto que consome `NodeMessageStream`/`pkg/ynodeproto` e compartilha fanout com o `Provider` local, em vez de parar no seam do edge;
- substituir o relay de exemplo/teste via WebSocket bruto por um transporte owner-side que use as mensagens tipadas já expostas em `pkg/ynodeproto`;
- propagar `epoch`/fencing para o caminho autoritativo de `apply`, append log, persistência e respostas de handoff/cutover, para além do lifecycle de lease já endurecido;
- fechar failover/handoff do owner em cima do bootstrap já implementado por `snapshot + update log`.

---

## Objetivo da fase atual

Consolidar o corte técnico em andamento da Meta 9, deixando o projeto pronto para avançar para V2 e operações derivadas com menos incerteza.

Nesta etapa, a aceitação é:
- consolidar contratos públicos atuais de `pkg/yjsbridge` e `pkg/storage` sem regressão de comportamento interno
- consolidar contratos públicos de `pkg/yprotocol` e `pkg/yawareness` em V1 com limites explícitos
- consolidar a camada mínima de runtime/provider de `pkg/yprotocol` (`Session`, `Provider`, `Open`, `Connection`, `DispatchResult`, `Persist`, `Close`, handlers e encoder público de `ProtocolMessage`) sem extrapolar para provider completo
- consolidar a primeira borda pública de transporte em `pkg/yhttp` sem acoplar o núcleo a um framework HTTP específico
- manter o mapeamento de lacunas de compatibilidade em merge/diff/intersect para reduzir risco de regressão
- consolidar persistência operacional V1 em memória e Postgres antes de expandir casos avançados
- documentar limites (suporte V1 somente, V2 rejeitado explicitamente) em qualquer novo exemplo/documentação funcional
- tratar a etapa atual como estabilidade operacional da API pública em `pkg/yjsbridge`, `pkg/yprotocol` e `pkg/yawareness` em V1, incluindo runtime/provider mínimo em processo, sem assumir provider completo
- congelar os invariantes locais que serão reutilizados como runtime do futuro owner distribuído
- preparar a transição para multi-nó sem reescrever a API pública já exposta

---

## Fase atual

### Meta técnica 9 — Fase 2 — operações binárias de documento

#### Entregáveis desta fase
- [x] Cobertura ampliada de `JSON` e `Any` em `merge/diff/intersect`
- [x] Round-trips adicionais para `Binary`, `Embed`, `Format` e metadata wire de `Item`
- [x] `intersect` alinhado para `nextClock` conforme observado no Yjs
- [x] Correção de regressão em ranges filtrados por `Skip`
- [x] Slicing de `ContentString` alinhado para `U+FFFD` em fronteiras inválidas
- [x] Lazy writer V1 interno integrado em encode/diff/intersect
- [x] Lazy writer V1 endurecido para slices com metadata, ordem encontrada no update e reuso pós-`finish`
- [x] Classificação mínima de formato com detecção de cabeçalho V2 e erro explícito para V2 ainda não suportado
- [x] API pública de `MergeUpdates`/formato alinhada à validação agregada com cobertura para payload vazio, mistura de formatos e erro indexado
- [x] APIs públicas de `StateVectorFromUpdate`, `EncodeStateVectorFromUpdate`, `CreateContentIDsFromUpdate` e `IntersectUpdateWithContentIDs` alinhadas ao dispatch por formato
- [x] `MergeUpdatesV1Context` e `MergeUpdatesContext` adicionados com cobertura para cancelamento e despacho V1
- [x] `DiffUpdateContext` e `IntersectUpdateWithContentIDsContext`, além das variantes V1 com `context`, adicionados com cobertura para cancelamento e dispatch
- [x] Validação agregada de formato com `context` propagada para `MergeUpdatesContext`, `StateVectorFromUpdatesContext` e `ContentIDsFromUpdatesContext`
- [x] Cobertura adicional de `MergeUpdatesV1` para preenchimento parcial de gaps já representados por `Skip`, inclusive `Skip` vindo do input
- [x] Contratos públicos adicionais para APIs com `context` e bordas V2/context
- [x] APIs derivadas alinhadas à pré-validação agregada para mistura de formatos e V2 detectado, preservando no-op para payload vazio
- [x] Contratos públicos de `state vector` e `content ids` cobrindo V2 detectado, mistura de formatos e índice preservado após payloads vazios
- [x] Regressões estruturais adicionadas para comutatividade de `merge`, gap sintético e workflow multi-client de `merge -> diff -> intersect`
- [x] Round-trips adicionais do `lazy writer` cobrindo multi-client, delete set e slices em fronteira UTF-16
- [x] Guardas defensivas em helpers de merge para listas vazias com cobertura dedicada
- [x] Contratos adicionais do `lazy writer` para `finish` vazio e normalização de structs intercaladas
- [x] Contratos adicionais de V2 cobrindo rejeição agregada após payloads vazios e precedência de mistura em `MergeUpdates`
- [x] `EncodeV1` normalizando structs fora de ordem por clock dentro do mesmo cliente
- [x] `MergeUpdates` alinhado ao erro indexado de V2 detectado
- [x] Cobertura adicional para erros/guards de `sliceStructWindowV1`, refs não fatiáveis e `Skip` explícito no meio do overlap
- [x] Guards de slice endurecidos contra overflow/underflow em `uint32`
- [x] `MergeUpdates` alinhado para `no-op` quando todos os payloads agregados são vazios
- [x] Contratos adicionais cobrindo payload V1 malformado nos caminhos context-aware de merge e validação agregada
- [x] Primeiro corte funcional de `conversion/snapshots` em V1 com normalização pública de update (`ConvertUpdateToV1`/`ConvertUpdatesToV1`) e extração de snapshot em memória (`state vector` + `delete set`)
- [x] `PersistedSnapshotFromUpdate(s)` implementa corte funcional de snapshot binário persistido V1 (canônico V1), sem suporte a V2 por enquanto
- [x] `EncodePersistedSnapshotV1`/`DecodePersistedSnapshotV1` fecham o ciclo mínimo de persistência/restore em V1, já integrados ao primeiro corte operacional de storage
- [x] Documentação dos blocos núcleo (`AGENT.md`, `SPEC.md`, `docs/yjs-functions-to-port.md`) sincronizada com o estado do código
- [x] Helpers de `SyncStep1`/`SyncStep2` a partir de múltiplos updates V1
- [x] Cobertura adicional para `type/doc/binary/embed/format/any`


#### Em foco (imediato)
- [ ] Expandir cobertura estrutural de `merge/diff/intersect` em casos com composição mais complexa de structs
- [ ] Expandir o uso já endurecido do lazy writer para o restante do fluxo principal de fusão/inspeção
- [ ] Fechar estudo inicial de compatibilidade V2 além da classificação mínima atual
- [ ] Decidir o próximo bloco de V2 (regras de parsing e integração de content/formatos) e critérios de aceitação
- [x] Alinhar runtime de `awareness` com upstream em clocks, proteção do estado local e expiração/heartbeat
- [ ] Expor deltas/operadores restantes de runtime de `awareness` quando a integração de provider exigir
- [ ] Registrar lacunas conhecidas por função antes de escalar para a próxima sprint da Fase 3
- [ ] Manter testes de regressão para os cenários críticos já encontrados

### Meta técnica 9 — Fase 3 — API pública, stores e examples

#### Entregáveis desta etapa
- [x] Definir pacote público estável em `pkg/yjsbridge` para operações de snapshot persistido e normalização/codificação V1 já exigidas
- [x] Consolidar a promoção da API pública de update em `pkg/yjsbridge` para V1 (`MergeUpdates`, `DiffUpdate`, `IntersectUpdateWithContentIDs`, `StateVectorFromUpdate`, `CreateContentIDsFromUpdate`), com suporte V2 apenas explicitamente rejeitado
- [x] Consolidar a promoção da API pública de sync protocol em `pkg/yprotocol` (`SyncStep1`, `SyncStep2`, envelope websocket e codecs), com suporte V2 explicitamente não suportado e sem provider completo
- [x] Consolidar a promoção da API pública de awareness em `pkg/yawareness` (wire format mínimo, merge runtime e heartbeat), com limites V1 e sem provider completo
- [x] Introduzir runtime in-process mínimo em `pkg/yprotocol` com `Session`, `HandleProtocolMessage`, `HandleEncodedMessages` e encoder público tipado de `ProtocolMessage`, ainda sem provider completo e sem V2
- [x] Introduzir camada mínima de provider em `pkg/yprotocol` acima de `Session`, com `Provider`, `Open`, `Connection`, `DispatchResult`, `Persist` e `Close`, ainda sem provider completo, sem transporte/distribuição e sem V2
- [x] Introduzir borda pública de transporte HTTP/WebSocket em `pkg/yhttp`, apoiada em `net/http` e `pkg/yprotocol.Provider`, sem dependência direta de frameworks como Gin/Echo
- [x] Introduzir adapters mínimos em `pkg/yhttp/gin`, `pkg/yhttp/echo` e `pkg/yhttp/chi` para wiring direto em frameworks HTTP suportados
- [x] Documentar contrato público mínimo de erros, cancelamento com `context.Context` e limites de suporte V1/V2
- [x] Introduzir abstrações de `store` para persistir e recuperar `PersistedSnapshot` com encode/decode canônico em `pkg/storage`
- [x] Implementar stores de referência em `pkg/storage/memory` e `pkg/storage/postgres` com integração real de persistência e testes
- [x] Adicionar exemplos de integração iniciais em `examples/memory` e `examples/postgres`
- [x] Adicionar exemplo inicial de transporte `net/http` + WebSocket em `examples/http-memory`
- [x] Adicionar examples de integração com Gin, Echo e Chi em cima dos adapters específicos
- [x] Adicionar variantes dos examples HTTP/WebSocket com persistência PostgreSQL
- [x] Adicionar smoke tests opt-in em `integration/` com Postgres efêmero, duas conexões WebSocket, restart do servidor e medição básica de throughput
- [x] Adicionar hooks opcionais de observabilidade em `pkg/yhttp` e adapter Prometheus em `pkg/yhttp/prometheus`
- [x] Adicionar matriz opt-in de performance em `integration/` cobrindo `net/http`, `gin`, `echo`, `chi`, memória, Postgres e tempo de restore
- [x] Expandir exemplos para cobrir `merge/diff/intersect` e `content ids` com cenários de integração mais completos
- [x] Consolidar a camada mínima de provider com examples, contratos de erro e cenários de integração em cima de `pkg/yprotocol.Provider`/`Connection`
- [x] Decidir a primeira borda de adaptação de transporte em cima do provider mínimo, sem acoplar o núcleo a WebSocket/Redis ou abrir provider completo cedo demais
- [x] Validar a etapa com matriz de compatibilidade/performance por cenário e registrar lacunas remanescentes de V2 sem regressão

### Meta técnica 10 — Fase 4 — arquitetura distribuída por ownership

#### Entregáveis desta etapa
- [x] Expor contratos públicos de persistência distribuída em `pkg/storage` para snapshot base, update log, placement e lease, mantendo `SnapshotStore` como base compatível
- [x] Implementar `DistributedStore` concreto em `pkg/storage/memory` e `pkg/storage/postgres` para snapshot, update log, placement e lease
- [x] Expor helpers públicos em `pkg/storage` para replay/recovery por `snapshot + update log`
- [x] Expor scaffolding público de control plane em `pkg/ycluster` com `ShardResolver`, `OwnerLookup`, `Runtime`, `DeterministicShardResolver`, `StaticLocalNode` e `PlacementOwnerLookup`
- [x] Expor adapters storage-backed em `pkg/ycluster` para lookup de owner e lease em cima de `pkg/storage`
- [x] Expor framing binário inicial do protocolo inter-node em `pkg/ynodeproto`, separado do wire `y-protocols` de cliente
- [x] Endurecer o lifecycle básico de lease em `pkg/ycluster` com `epoch` monotônico no acquire/renew/takeover e propagação desse epoch na resolução de owner
- [x] Formalizar `DocumentKey`/room/shard como unidade de ownership, lease e roteamento
- [x] Garantir owner único por documento/shard com lease renovável, expiração detectável e revogação observável
- [ ] Introduzir `epoch` monotônico e fencing token em toda operação autoritativa (`apply`, persistência, append log, handoff e recovery)
- [x] Materializar `snapshot + update log` como fonte de hidratação, replay e recuperação do runtime local do owner sobre os contratos já expostos em `pkg/storage`, com bootstrap do provider a partir de snapshot base + tail do log
- [ ] Persistir snapshot base e update log append-only por epoch, com replay determinístico e checkpoint/compaction planejados
- [x] Materializar payloads inter-node tipados e versionados sobre o framing já exposto em `pkg/ynodeproto`, pelo menos para handshake, sync request/response, document update, awareness update e ping/pong
- [x] Expor uma borda HTTP/WS owner-aware em `pkg/yhttp` para resolver owner antes do provider local e só materializar `Session`/`Provider` quando o owner resolvido é local
- [x] Expor um seam typed de forwarding remoto em `pkg/yhttp` via `OwnerAwareServerConfig.OnRemoteOwner`, `RemoteOwnerDialer` e `NodeMessageStream`
- [ ] Definir comportamento do nó não-owner para requests HTTP e frames WebSocket: autenticar, resolver owner, encaminhar pelo wire inter-node tipado e encerrar/cutover em caso de fencing ou handoff
- [ ] Definir handoff seguro com bootstrap por snapshot, replay do tail do log e troca atômica de epoch
- [ ] Introduzir observabilidade e diagnósticos para lease, roteamento, forwarding, replay, lag e troca de owner

#### Em foco (abertura da fase)
- [x] Tratar `pkg/yprotocol.Provider` atual como runtime local do owner, sem fanout multi-process ad hoc e com bootstrap por `snapshot + update log`
- [x] Definir a fronteira entre a borda `pkg/yhttp` e a futura camada inter-node, explicitando o modo edge owner-aware
- [x] Promover `pkg/ynodeproto` de framing puro para camada de mensagens tipadas sem quebrar `Header`/`Frame` já expostos
- [x] Materializar backend/schema concretos para snapshot base, update log, placement e lease acima dos contratos públicos já definidos
- [ ] Definir regras de recuperação após queda do owner e de promoção segura de novo owner
- [x] Adicionar testes de integração das fundações distribuídas (`snapshot + update log` + owner lookup storage-backed)
- [x] Registrar explicitamente que o modo single-process atual continua suportado como modo de referência

## Progresso de metas anteriores

As metas técnicas anteriores ao escopo de Meta 9 estão completadas no corte mínimo:

- [x] Meta técnica 1 concluída
- [x] Meta técnica 2 concluída
- [x] Meta técnica 3 concluída
- [x] Meta técnica 4 concluída
- [x] Meta técnica 5 concluída
- [x] Meta técnica 6 concluída
- [x] Meta técnica 7 concluída
- [x] Meta técnica 8 concluída
- [ ] Meta técnica 9 em execução
- [ ] Meta técnica 10 em execução

---

## Estrutura inicial desejada

Quando a implementação começar, a tendência inicial é criar algo próximo de:

```text
internal/
  binary/
  varint/
  ytypes/
  yupdate/
  yidset/
  yprotocol/
  yawareness/
pkg/
  yjsbridge/
  yprotocol/
  yawareness/
  storage/
    memory/
    postgres/
```
