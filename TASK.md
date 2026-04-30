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
- pacote `pkg/yprotocol` com API pública de protocolo em V1 (`SyncStep1`, `SyncStep2`, envelope websocket, encoder público de `ProtocolMessage` e helpers de parse/encode), aceitando payloads V2 válidos nos caminhos de sync por normalização canônica para V1
- pacote `internal/yprotocol` com wire format mínimo do sync protocol e suporte interno adicional
- envelope de mensagens websocket para `sync`, `awareness`, `auth` e `queryAwareness` (sem alegar implementação de provider completo)
- helpers de sync inicial a partir de múltiplos updates V1
- pacote `pkg/yawareness` com API pública de awareness runtime/wire em V1 (estado local, deltas, operadores de campo, proteção do estado local, expiração e heartbeat)
- runtime in-process mínimo em `pkg/yprotocol` com `Session`, `HandleProtocolMessage` e `HandleEncodedMessages` para composição local de sync/awareness sem provider completo
- camada mínima de provider em `pkg/yprotocol` acima de `Session`, com `Provider`, `Open`, `Connection`, `DispatchResult`, `Persist` e `Close`, ainda sem provider completo/transporte distribuído próprio, mas normalizando sync V2 válido para V1 antes de broadcast e persistência
- pacote `pkg/yhttp` com handler `net/http` + WebSocket acima de `pkg/yprotocol.Provider`, com fanout local, persistência opcional no close e compatibilidade por adaptação com Gin/Echo/afins
- subpacotes `pkg/yhttp/gin`, `pkg/yhttp/echo` e `pkg/yhttp/chi` com adapters mínimos para acoplamento direto em frameworks HTTP
- hooks opcionais de observabilidade em `pkg/yhttp` via interface `Metrics`, com adapter Prometheus em `pkg/yhttp/prometheus`
- um runtime local por documento em `pkg/yprotocol.Provider` que passa a servir como embrião do futuro owner distribuído
- pacote `internal/yawareness` com wire format mínimo de awareness
- state manager de awareness alinhado em clocks, deltas `Added/Updated/Removed`, operadores locais, proteção do estado local e expiração/renovação básica
- cobertura ampliada para round-trip de content refs, metadata wire de `Item` e slicing estrutural de `JSON`/`Any`
- regressão adicional cobrindo fluxo `merge -> diff -> intersect` sobre cadeia de 3 updates fora de ordem
- lazy writer V1 endurecido para ordem encontrada no update, reuso pós-`finish` e preservação de metadata em slices
- classificação mínima de formato já distinguindo cabeçalho V2 e mistura de formatos incompatíveis
- API pública de formato/merge endurecida com validação agregada e erro indexado por update
- APIs públicas de inspeção por `single-update` alinhadas para despacho V1 e conversão canônica de V2 válido para V1
- `MergeUpdatesV1`/`MergeUpdates` agora expõem variantes com `context.Context` e usam agregação paralela na etapa de fusão
- `DiffUpdate` e `IntersectUpdateWithContentIDs` agora expõem variantes com `context.Context`
- a pré-validação agregada de formato também respeita `context.Context` nas APIs multi-update
- novas regressões de merge cobrindo preenchimento de `Skip` sintético e `Skip` explícito vindo do input
- contratos públicos adicionais cobrindo equivalência/cancelamento das APIs com `context` e bordas V2/context
- APIs derivadas de `state vector` e `content ids` alinhadas à pré-validação agregada de formato
- variantes com `context.Context` para agregação de `state vector` e `content ids`
- conversão de V2 válido para V1 antes de extrair `state vector`, `content ids` e snapshots diretos
- primeiro corte V2 com reader interno e conversão canônica para V1, validado por fixtures upstream do Yjs
- cobertura V2 single-update ampliada com texto Unicode, `Any` aninhado em array/map e XML com atributo/texto usando fixtures upstream do Yjs
- cobertura V2 multi-update com fixtures upstream de `Y.mergeUpdatesV2` para append de texto entre clientes, deleção após insert, formatação sobre texto deletado, sets independentes/sobrescritos em map, escrita em map aninhado, XML, deleção em array e subdoc com update subsequente
- encoder V2 e APIs públicas opt-in `ConvertUpdateToV2`, `ConvertUpdatesToV2`, `MergeUpdatesV2`, `DiffUpdateV2` e `IntersectUpdateWithContentIDsV2`, mantendo as APIs sem sufixo em V1 canônico
- hardening V2 adicional rejeitando valores não consumidos em encoders laterais já cobertos pelo reader, como `client`, `info` e `keyClock` sem chaves consumidas, além de `parentInfo` inválido e coleções JSON/Any superdimensionadas
- `pkg/yprotocol.Session` e `pkg/yprotocol.Provider` agora aceitam `sync update`/`step2` com V2 válido e convertem para V1 canônico antes de atualizar estado, emitir broadcast, append no update log ou snapshot persistido
- segurança HTTP/WebSocket ampliada com `OriginPolicy`/`StaticOriginPolicy` para allowlist/preflight CORS e `RequestRedactor`/`HashingRequestRedactor` para sanitizar métricas e handlers de erro
- hardening público ampliado com `QuotaLimiter`/`LocalQuotaLimiter`, auth inter-node HMAC com `key_id`, timestamp/nonce/replay protection, rotação de segredo e validadores fail-closed de configuração de produção
- leitura de stream endurecida com contratos explícitos de cancelamento quando disponíveis
- limpeza de artefatos temporários de depuração introduzidos em rodadas anteriores
- documentação de roadmap alinhada ao estado real do código
- regressões novas para comutatividade de `merge`, gaps sintéticos, workflow multi-client de `merge -> diff -> intersect`, round-trip do `lazy writer` e contratos V2 das APIs derivadas/mutadoras com saída V1 canônica
- guardas defensivas em helpers internos de merge para listas vazias, evitando `panic` fora do caminho feliz
- contratos adicionais do `lazy writer` cobrindo `finish` vazio e normalização de structs intercaladas em `EncodeV1`
- contratos agregados adicionais de V2 cobrindo precedência de mistura `V1/V2` em `MergeUpdates`
- `EncodeV1` agora normaliza structs por clock dentro de cada cliente antes de serializar
- `MergeUpdates` agora aceita V2 válido por conversão canônica para V1, alinhando o comportamento às APIs derivadas
- cobertura adicional para `sliceStructWindowV1`, refs não fatiáveis em `ParsedContent.SliceWindow` e continuação de merge após `Skip` explícito no overlap
- guards de slice endurecidos com aritmética segura contra overflow em `sliceStructWindowV1` e `ParsedContent.SliceWindow`
- `MergeUpdates` agora trata listas compostas apenas por payloads vazios como `no-op`, retornando update V1 vazio
- cobertura context-aware adicional para payload V1 malformado em `MergeUpdatesV1Context` e `ValidateUpdatesFormatWithReasonContext`
- primeiro corte funcional de `conversion/snapshots` em V1 com `ConvertUpdateToV1`, `ConvertUpdatesToV1`, `SnapshotFromUpdate(s)`, `PersistedSnapshotFromUpdate(s)` e codec explícito `Encode/DecodePersistedSnapshotV1` para estado em memória, bytes canônicos e restore V1
- `ConvertUpdateToV1`, `ConvertUpdatesToV1` e construtores de `PersistedSnapshot` agora aceitam V2 válido via reader interno fixture-backed e emitem V1 canônico; restore `DecodePersistedSnapshotV1` continua V1-only
- ciclo de hidratação reversa de `PersistedSnapshot` em V1 com `EncodePersistedSnapshotV1` e `DecodePersistedSnapshotV1` (`context` incluído), mantendo restore V2 rejeitado
- testes cobrindo casos válidos e inválidos
- `pkg/yjsbridge` já expõe API pública mínima para `Snapshot`, `PersistedSnapshot` e operações de persistência V1 (`Convert`, `Encode`, `Decode`)
- `pkg/yjsbridge` também recebeu a promoção da API pública de `update` em V1, com superfície exposta para operações de merge/diff/intersect e utilitários de `state vector`/`content ids`; V2 válido alimenta essas operações por conversão canônica para V1
- `pkg/storage` já define `SnapshotStore`, `DocumentKey`, `SnapshotRecord`, erros de domínio e contratos de `SaveSnapshot`/`LoadSnapshot`
- `pkg/storage` agora também expõe o scaffolding distribuído inicial com `UpdateLogStore`, `PlacementStore`, `PlacementListStore`, `LeaseStore`, `DistributedStore`, além de `UpdateLogRecord`, `PlacementRecord`, `LeaseRecord`, `OwnerInfo`, `ShardID`, `NodeID` e `UpdateOffset`
- `pkg/storage` agora também já expõe helpers públicos de replay/recovery/compaction (`ReplaySnapshot`, `RecoverSnapshot`, `CompactUpdateLog`, `CompactUpdateLogAuthoritative`) para reconstrução via `snapshot + update log`
- `pkg/storage` agora também expõe a fronteira pública de fencing autoritativo com `AuthorityFence`, `ErrAuthorityLost`, `AuthoritativeUpdateLogStore` e `AuthoritativeSnapshotStore`
- `pkg/ycluster` agora expõe o scaffolding público de control plane com tipos estáveis de `NodeID`/`ShardID`/`Placement`/`Lease`, `DeterministicShardResolver`, `StaticLocalNode`, `PlacementOwnerLookup`, `LeaseManager`, `StorageOwnershipCoordinator`, `DocumentOwnershipRuntime` e interfaces mínimas de `Runtime`
- `pkg/ycluster` agora também já expõe adapters storage-backed com `StorageOwnerLookup`, `StorageLeaseStore`, `StorageOwnershipCoordinator`, `StoragePlacementDocumentSource`, promoção segura quando owner está ausente/expirado, rebalance explícito/planejado/periódico por documento, execução bloqueante/ref-counted de ownership por documento e conversões `storage <-> cluster`
- `pkg/storage/memory` e `pkg/storage/postgres` agora cercam leases por geração persistida, com `OwnerInfo.Epoch` obrigatório, `ErrLeaseConflict`/`ErrLeaseStaleEpoch` e preservação da última geração após release
- `pkg/storage/memory` e `pkg/storage/postgres` agora também validam placement + lease + token + expiração em operações autoritativas de append/persist/trim, retornando `ErrAuthorityLost` quando o fence esperado já não é válido
- `pkg/ycluster` agora só resolve ownership a partir de lease ativa e válida; placement sozinho não classifica mais owner local/remoto
- `pkg/ynodeproto` agora expõe o framing binário versionado do protocolo inter-node com `Header`, `Frame`, `MessageType`, `Encode/DecodeFrame` e decode por prefixo
- `pkg/ynodeproto` agora também carrega `clientID` em `handshake`/`handshake-ack` e mensagens roteadas para `query-awareness`, `disconnect` e `close`
- `pkg/storage/memory` e `pkg/storage/postgres` já implementam stores operacionais de snapshots com persistência canônica em V1
- `pkg/storage/memory` e `pkg/storage/postgres` agora também já implementam `DistributedStore` com update log, placement e lease
- `pkg/yhttp` agora expõe um forwarder remoto typed plugável em `OwnerAwareServerConfig.OnRemoteOwner`, baseado em `RemoteOwnerDialer`/`NodeMessageStream`, normalizando payloads sync V2 válidos para V1 canônico antes de preencher mensagens inter-node `UpdateV1`
- `pkg/yhttp.Server` agora pode receber `DocumentOwnershipRuntime` para assumir ownership local antes de abrir o provider e liberar a lease no fechamento de conexões locais, streams owner-side e takeover `remote -> local`
- `pkg/yhttp.OwnerAwareServer` agora pode promover localmente, via flag explícita, quando o lookup indica owner ausente/expirado e o `Server` local possui `DocumentOwnershipRuntime`
- `pkg/yhttp.RemoteOwnerEndpoint` agora valida o epoch autoritativo real do provider contra o epoch do handshake inter-node, e o edge reconhece `Close` retryable ainda durante o handshake, retornando `503`/`Retry-After` quando o upgrade precisa ser refeito
- `pkg/yhttp` agora sinaliza perda de autoridade detectada durante writes locais pelo mesmo caminho de handoff/rebind transparente, sem depender apenas da revalidação periódica, e cobre timeout de rebind quando o epoch remoto não avança
- `pkg/yhttp` agora possui primeira camada de segurança HTTP/WebSocket opt-in com `Authenticator`, `Authorizer`, `BearerTokenAuthenticator` e `TenantAuthorizer` para isolar `DocumentKey.Namespace`
- `pkg/yhttp` agora também expõe `RateLimiter`, `FixedWindowRateLimiter` e chaves de referência por principal/IP, tenant ou documento para limitar abertura HTTP/WebSocket antes de materializar provider local ou resolver owner remoto
- `examples/memory` e `examples/postgres` foram adicionados com fluxos iniciais de persistência usando a API pública e stores referenciados
- `examples/provider-memory` agora cobre fluxo local de provider com late joiner, persistência explícita e restore em novo provider, servindo como referência do recovery por snapshot antes do replay distribuído
- `examples/http-memory` foi adicionado como exemplo de transporte `net/http` + WebSocket com `pkg/yhttp`
- `examples/gin-memory`, `examples/echo-memory` e `examples/chi-memory` foram adicionados como exemplos de acoplamento por adapters específicos de framework
- `examples/http-postgres`, `examples/gin-postgres`, `examples/echo-postgres` e `examples/chi-postgres` foram adicionados como variantes com persistência PostgreSQL para o fluxo WebSocket
- `examples/gin-react-tailwind-postgres` foi adicionado como demo full-stack com `vite` + `react` + `tailwindcss`, backend `gin`, persistência PostgreSQL e editor colaborativo com awareness
- `examples/owner-aware-http-edge` agora sobe um owner remoto real, usa `DocumentOwnershipRuntime` nos owners, expõe `/metrics` com adapters Prometheus rotulados por node e demonstra relay de tráfego WebSocket entre edge e owner
- `examples/owner-aware-http-edge/observability` agora inclui regras Prometheus de SLO por `env`/`region`/`tenant`/`deployment_role` além dos alertas por node e dashboards Grafana
- pacote `integration` adiciona smoke tests opt-in com Postgres efêmero via Docker para funcionalidade, persistência e performance básica do fluxo WebSocket
- pacote `integration` também adiciona matriz opt-in de performance entre `net/http`, `gin`, `echo` e `chi`, cobrindo memória/Postgres e medindo throughput, latência e restore
- `integration/owner_aware_remote_relay_test.go` agora cobre sync e awareness atravessando o relay owner-aware para um owner remoto

A fase atual é **Meta técnica 9 / Fase 3 em consolidação**, com API pública de snapshot e de update em V1 já em operação em `pkg/yjsbridge`, além da exposição pública de protocolo e awareness em `pkg/yprotocol` e `pkg/yawareness` em V1.
As metas técnicas 1, 2, 3, 4, 5, 6, 7 e 8 já possuem corte mínimo implementado.
Em Meta 9, a prioridade continua sendo reduzir lacunas de compatibilidade estrutural em merge/diff/intersect, estabilizar o writer incremental e manter V2 como trilha explícita/opt-in sem alterar defaults V1-first.
Paralelamente, a nova etapa registra a consolidação operacional de snapshots V1 por meio de API pública e stores, o runtime in-process público de protocolo em `pkg/yprotocol`, a camada mínima de provider acima de `Session` e a primeira borda pública de transporte HTTP/WebSocket em `pkg/yhttp`, ainda sem saída V2 em storage/protocolo e sem transporte distribuído próprio no provider local.
A mesma transição também já materializou um corte operacional mais amplo da Meta 10: os contratos de persistência distribuída, o framing inter-node, os backends concretos de update log/placement/lease, listagem de placements, os helpers públicos de replay, o bootstrap do provider por `snapshot + update log`, a borda owner-aware, a propagação de fencing autoritativo até o runtime local, o handoff atômico de lease/epoch no storage/control plane, a primitiva explícita de rebalance document-level, política determinística com execução, control loop periódico de rebalance, seleção dinâmica de targets por membership/health, revalidação imediata de autoridade na borda a partir de decisões do controller e um bundle inicial de observabilidade operacional já estão no branch.
A próxima fase aberta no roadmap é a **Meta técnica 10 / Fase 4**, que fecha owner único por documento/shard com `lease`/`epoch`/fencing propagados até `apply`/persist/handoff, protocolo inter-node próprio e aceite de HTTP/WS em qualquer nó com processamento do room restrito ao owner.

### Corte provável do próximo epoch

- [x] usar a primitiva de handoff atômico já exposta para fechar o corte document-level de failover/rebalance do owner em cima do bootstrap por `snapshot + update log`;
- [x] preparar política determinística de rebalance multi-nó acima da primitiva `RebalanceDocument`;
- [x] preparar daemon/control loop automático de rebalance acima de `PlanDocumentRebalance`/`ExecuteRebalancePlan`;
- [x] conectar fonte operacional de documentos por placement store ao `RebalanceController`;
- [x] conectar cutover/rebind automático às decisões do controller.

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
- documentar limites: APIs sem sufixo seguem com saída canônica V1, APIs `*V2` são opt-in, restore V2 rejeitado e payloads V2 válidos de sync normalizados para V1 antes de broadcast/persistência
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
- [x] Classificação mínima de formato com detecção de cabeçalho V2 e erro explícito nos caminhos V2 ainda não suportados
- [x] API pública de `MergeUpdates`/formato alinhada à validação agregada com cobertura para payload vazio, mistura de formatos e erro indexado
- [x] APIs públicas de `StateVectorFromUpdate`, `EncodeStateVectorFromUpdate`, `CreateContentIDsFromUpdate` e `IntersectUpdateWithContentIDs` alinhadas ao dispatch por formato
- [x] `MergeUpdatesV1Context` e `MergeUpdatesContext` adicionados com cobertura para cancelamento e despacho V1
- [x] `DiffUpdateContext` e `IntersectUpdateWithContentIDsContext`, além das variantes V1 com `context`, adicionados com cobertura para cancelamento e dispatch
- [x] Validação agregada de formato com `context` propagada para `MergeUpdatesContext`, `StateVectorFromUpdatesContext` e `ContentIDsFromUpdatesContext`
- [x] Cobertura adicional de `MergeUpdatesV1` para preenchimento parcial de gaps já representados por `Skip`, inclusive `Skip` vindo do input
- [x] Contratos públicos adicionais para APIs com `context` e bordas V2/context
- [x] APIs derivadas alinhadas à pré-validação agregada para mistura de formatos e V2 detectado, preservando no-op para payload vazio
- [x] Contratos públicos de `state vector` e `content ids` cobrindo V2 válido por conversão, mistura de formatos e erros indexados após payloads vazios
- [x] Regressões estruturais adicionadas para comutatividade de `merge`, gap sintético e workflow multi-client de `merge -> diff -> intersect`
- [x] Round-trips adicionais do `lazy writer` cobrindo multi-client, delete set e slices em fronteira UTF-16
- [x] Guardas defensivas em helpers de merge para listas vazias com cobertura dedicada
- [x] Contratos adicionais do `lazy writer` para `finish` vazio e normalização de structs intercaladas
- [x] Contratos adicionais de V2 cobrindo conversão canônica e precedência de mistura em `MergeUpdates`
- [x] `EncodeV1` normalizando structs fora de ordem por clock dentro do mesmo cliente
- [x] `MergeUpdates` alinhado à conversão canônica de V2 válido para V1
- [x] Cobertura adicional para erros/guards de `sliceStructWindowV1`, refs não fatiáveis e `Skip` explícito no meio do overlap
- [x] Guards de slice endurecidos contra overflow/underflow em `uint32`
- [x] `MergeUpdates` alinhado para `no-op` quando todos os payloads agregados são vazios
- [x] Contratos adicionais cobrindo payload V1 malformado nos caminhos context-aware de merge e validação agregada
- [x] Primeiro corte funcional de `conversion/snapshots` em V1 com normalização pública de update (`ConvertUpdateToV1`/`ConvertUpdatesToV1`) e extração de snapshot em memória (`state vector` + `delete set`)
- [x] Primeiro corte V2 fixture-backed: reader interno, `DecodeUpdate`, `ConvertUpdateToV1`, `ConvertUpdatesToV1` e construtores de `PersistedSnapshot` convertendo V2 válido para V1 canônico
- [x] `StateVectorFromUpdate(s)`, `EncodeStateVectorFromUpdate(s)`, `CreateContentIDsFromUpdate(s)` e `SnapshotFromUpdate(s)` aceitam V2 válido por conversão canônica para V1
- [x] Cobertura V2 ampliada com fixtures upstream para `binary`, `embed`, `format`, texto Unicode, `Any` aninhado, XML com atributo/texto, tipo aninhado e subdoc, além de hardening contra truncamento, tabela de strings inconsistente, overflow em delete set, `parentInfo` inválido, coleções superdimensionadas e valores não consumidos em encoders laterais consumidos
- [x] Cobertura V2 multi-update com fixtures upstream de `Y.mergeUpdatesV2` validando `MergeUpdates`, `ConvertUpdatesToV1`, `DiffUpdate` e `IntersectUpdateWithContentIDs` em texto/format, map, XML, array, subdoc e tipo aninhado
- [x] `MergeUpdates`, `DiffUpdate` e `IntersectUpdateWithContentIDs` aceitam V2 válido por conversão canônica para V1
- [x] `PersistedSnapshotFromUpdate(s)` implementa corte funcional de snapshot binário persistido V1 (canônico V1), aceitando V2 válido por conversão para V1
- [x] `EncodePersistedSnapshotV1`/`DecodePersistedSnapshotV1` fecham o ciclo mínimo de persistência/restore em V1, já integrados ao primeiro corte operacional de storage
- [x] Documentação dos blocos núcleo (`AGENT.md`, `SPEC.md`, `docs/yjs-functions-to-port.md`) sincronizada com o estado do código
- [x] Helpers de `SyncStep1`/`SyncStep2` a partir de múltiplos updates V1
- [x] Cobertura adicional para `type/doc/binary/embed/format/any`


#### Em foco (imediato)
- [x] Expandir cobertura estrutural de `merge/diff/intersect` em casos com composição mais complexa de structs
- [x] Expandir o uso já endurecido do lazy writer para o restante do fluxo principal de fusão/inspeção
- [x] Fechar estudo inicial de compatibilidade V2 além da classificação mínima atual em `docs/v2-compatibility-map.md`
- [x] Decidir o próximo bloco de V2: reader/conversão limitada com fixtures upstream antes de liberar APIs públicas
- [x] Alinhar runtime de `awareness` com upstream em clocks, proteção do estado local e expiração/heartbeat
- [x] Expor deltas/operadores restantes de runtime de `awareness` quando a integração de provider exigir
- [x] Registrar lacunas conhecidas por função antes de escalar para a próxima sprint da Fase 3
- [x] Manter testes de regressão para os cenários críticos já encontrados

### Meta técnica 9 — Fase 3 — API pública, stores e examples

#### Entregáveis desta etapa
- [x] Definir pacote público estável em `pkg/yjsbridge` para operações de snapshot persistido e normalização/codificação V1 já exigidas
- [x] Consolidar a promoção da API pública de update em `pkg/yjsbridge` para V1 (`MergeUpdates`, `DiffUpdate`, `IntersectUpdateWithContentIDs`, `StateVectorFromUpdate`, `CreateContentIDsFromUpdate`), aceitando V2 válido por conversão canônica para V1
- [x] Consolidar a promoção da API pública de sync protocol em `pkg/yprotocol` (`SyncStep1`, `SyncStep2`, envelope websocket e codecs), com V2 válido normalizado para V1 nos payloads de sync e sem provider completo
- [x] Consolidar a promoção da API pública de awareness em `pkg/yawareness` (wire format mínimo, merge runtime e heartbeat), com limites V1 e sem provider completo
- [x] Introduzir runtime in-process mínimo em `pkg/yprotocol` com `Session`, `HandleProtocolMessage`, `HandleEncodedMessages` e encoder público tipado de `ProtocolMessage`, ainda sem provider completo e com V2 válido convertido para V1 nos caminhos mutáveis de sync
- [x] Introduzir camada mínima de provider em `pkg/yprotocol` acima de `Session`, com `Provider`, `Open`, `Connection`, `DispatchResult`, `Persist` e `Close`, ainda sem provider completo/transporte distribuído próprio e com broadcast/persistência sempre em V1 canônico
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
- [x] Implementar `PlacementListStore` concreto em `pkg/storage/memory` e `pkg/storage/postgres` para alimentar control loops por documentos conhecidos
- [x] Expor helpers públicos em `pkg/storage` para replay/recovery por `snapshot + update log`
- [x] Expor scaffolding público de control plane em `pkg/ycluster` com `ShardResolver`, `OwnerLookup`, `Runtime`, `DeterministicShardResolver`, `StaticLocalNode` e `PlacementOwnerLookup`
- [x] Expor adapters storage-backed em `pkg/ycluster` para lookup de owner e lease em cima de `pkg/storage`
- [x] Expor framing binário inicial do protocolo inter-node em `pkg/ynodeproto`, separado do wire `y-protocols` de cliente
- [x] Endurecer o lifecycle básico de lease em `pkg/ycluster` com `epoch` monotônico no acquire/renew/takeover e propagação desse epoch na resolução de owner
- [x] Expor um coordenador local mínimo de lease em `pkg/ycluster` para acquire/renew/reacquire/release do shard sem script manual no caminho quente
- [x] Adicionar loop autônomo bloqueante ao `LeaseManager` para manter a lease renovada enquanto o contexto do owner estiver ativo
- [x] Expor um coordenador storage-backed por documento em `pkg/ycluster` para gravar placement, materializar lease, resolver owner e derivar `AuthorityFence`/`LeaseManager` a partir de `storage.DocumentKey`
- [x] Expor promoção explícita por documento em `pkg/ycluster` que só toma ownership quando não há owner remoto ativo ou quando a lease atual expirou
- [x] Expor rebalance explícito por documento em `pkg/ycluster`, usando no-op seguro, promoção opt-in e handoff atômico de lease/epoch
- [x] Expor planejamento e execução sequencial de rebalance em `pkg/ycluster`, calculando moves por carga document-level e executando via `RebalanceDocument`
- [x] Expor `RebalanceController` em `pkg/ycluster` para rodar planejamento/execução uma vez ou em loop periódico com `context.Context`
- [x] Expor `StoragePlacementDocumentSource` em `pkg/ycluster` para alimentar o controller a partir de placements persistidos
- [x] Expor membership/health em `pkg/ycluster` com `HealthyRebalanceTargetSource` para seleção dinâmica de targets saudáveis no `RebalanceController`
- [x] Expor callback em `pkg/yhttp` para revalidar autoridade local imediatamente após resultados de rebalance do controller, acionando cutover/rebind sem aguardar o ticker
- [x] Expor execução bloqueante de ownership por documento em `pkg/ycluster`, combinando claim, loop de lease e release controlado no shutdown
- [x] Expor runtime ref-counted de ownership por documento em `pkg/ycluster`, compartilhando uma única lease local entre múltiplos callers do mesmo documento
- [x] Formalizar `DocumentKey`/room/shard como unidade de ownership, lease e roteamento
- [x] Garantir owner único por documento/shard com lease renovável, expiração detectável e revogação observável
- [x] Expor `AuthorityFence`, `ErrAuthorityLost` e contratos autoritativos de snapshot/update log em `pkg/storage`
- [x] Implementar validação de fencing autoritativo em `pkg/storage/memory` e `pkg/storage/postgres` para `append`, `persist` e `trim`, cruzando placement + lease + token + expiração
- [x] Propagar `epoch`/fencing para o caminho autoritativo do runtime (`apply`, `Persist`, recovery, handoff e respostas de cutover`)
- [x] Materializar `snapshot + update log` como fonte de hidratação, replay e recuperação do runtime local do owner sobre os contratos já expostos em `pkg/storage`, com bootstrap do provider a partir de snapshot base + tail do log
- [x] Fechar persistência de snapshot base + update log append-only por epoch, conectando checkpoint/compaction ao handoff e recovery distribuídos
- [x] Materializar payloads inter-node tipados e versionados sobre o framing já exposto em `pkg/ynodeproto`, pelo menos para handshake, sync request/response, document update, awareness update e ping/pong
- [x] Expor uma borda HTTP/WS owner-aware em `pkg/yhttp` para resolver owner antes do provider local e só materializar `Session`/`Provider` quando o owner resolvido é local
- [x] Expor autenticação/autorização HTTP/WS plugável em `pkg/yhttp`, com bearer token estático de referência e boundary multi-tenant por namespace
- [x] Expor rate limit HTTP/WS plugável em `pkg/yhttp`, com implementação fixed-window local de referência e chaves por principal/IP, tenant ou documento
- [x] Conectar `DocumentOwnershipRuntime` opcional ao `pkg/yhttp.Server`, garantindo claim/release de lease ao lifecycle de conexões WebSocket locais, endpoint owner-side e takeover `remote -> local`
- [x] Permitir promoção local opt-in na borda owner-aware quando não existe owner ativo, preservando o fallback retryable como comportamento padrão
- [x] Expor um seam typed de forwarding remoto em `pkg/yhttp` via `OwnerAwareServerConfig.OnRemoteOwner`, `RemoteOwnerDialer` e `NodeMessageStream`
- [x] Validar epoch do handshake inter-node contra a autoridade real do provider owner-side e propagar `Close` retryable/`503` quando o edge usa epoch obsoleto
- [x] Sinalizar perda de autoridade no write local para acionar handoff/rebind imediatamente e cobrir rebind remoto com epoch estagnado até timeout
- [x] Fechar o comportamento do nó não-owner em cima da borda owner-aware já exposta: autenticar, resolver owner, encaminhar pelo wire inter-node tipado e encerrar/cutover em caso de fencing ou handoff
- [x] Definir handoff seguro com bootstrap por snapshot, replay do tail do log e troca atômica de epoch
- [x] Expor gauges Prometheus de replay/recovery/compaction e persistência local para offsets, lag de tail e epoch observado
- [x] Expandir a observabilidade já existente para dashboards/alertas de lease, roteamento, forwarding, replay, lag e troca de owner

Nota de progresso atual:
- `snapshot + update log` já persistem checkpoint/high-water mark e metadata observável de `epoch` em memória/Postgres; replay público rejeita regressão de epoch no tail e `CompactUpdateLogAuthoritative` salva checkpoint + trim sob o mesmo fence;
- `pkg/yprotocol.Connection.HandleEncodedMessagesContext` agora propaga o contexto dos handlers HTTP/inter-node para o append autoritativo do apply path, preservando `HandleEncodedMessages` como wrapper compatível;
- `examples/owner-aware-http-edge/observability` agora inclui alertas Prometheus e dashboards Grafana operacional/oráculo para conexões, rotas, handoff/rebind, leases, offsets, epochs, erros e latências p95 por node;
- a borda owner-aware já cobre relay inter-node tipado com handoff transparente do browser entre `remote -> remote`, `remote -> local` e `local -> remote`, além de revalidação periódica/forçada e métricas de transição de ownership;
- `pkg/storage` e `pkg/yprotocol` agora também já expõem hooks/adapters Prometheus para replay/recovery/compaction, offsets, lag de tail, epoch observado e lifecycle local de autoridade/persistência do provider;
- `pkg/ycluster` agora também já expõe `LeaseManager`, `StorageOwnershipCoordinator`, `DocumentOwnershipRuntime`, execução bloqueante/ref-counted de ownership por documento, rebalance explícito/planejado/periódico por documento e adapter Prometheus para owner lookup e operações de lease do control plane;
- `pkg/ycluster.StorageOwnershipCoordinator.PromoteDocument` formaliza a regra inicial de recuperação: owner remoto ativo bloqueia promoção, owner ausente/expirado permite takeover com epoch monotônico;
- `pkg/storage.LeaseHandoffStore` e `pkg/ycluster.StorageOwnershipCoordinator.HandoffDocument` definem a troca atômica de owner: o novo holder só assume quando token/lease atuais ainda batem, o epoch avança exatamente `+1` e o `LeaseManager` do novo owner já nasce com o lease materializado;
- `pkg/ycluster.StorageOwnershipCoordinator.RebalanceDocument` fecha a primitiva document-level acima do handoff: no-op quando o alvo já é owner, erro em modo estrito sem owner ativo, promoção opt-in quando o owner está ausente/expirado e handoff atômico quando há owner ativo diferente do alvo;
- `pkg/ycluster.StorageOwnershipCoordinator.PlanDocumentRebalance` e `ExecuteRebalancePlan` adicionam a primeira política determinística acima da primitiva: moves por desbalanceamento, owner fora do target set e promoção planejada de documentos sem owner ativo;
- `pkg/storage.PlacementListStore` agora lista placements conhecidos em memória/Postgres e `pkg/ycluster.StoragePlacementDocumentSource` alimenta o `RebalanceController` a partir dessa fonte;
- `pkg/ycluster.RebalanceController` transforma planner/executor em control loop periódico com source plugável de documentos, targets estáticos ou dinâmicos por membership/health e callbacks de plano/resultado;
- `pkg/yhttp.NewRebalanceAuthorityRevalidationCallback` conecta resultados do `RebalanceController` à revalidação imediata de conexões locais afetadas, usando o mesmo caminho de close retryable/handoff transparente já existente;
- `pkg/yhttp.OwnerAwareServerConfig.PromoteLocalOnOwnerUnavailable` aplica essa regra na borda HTTP/WS de forma opt-in e somente quando o servidor local tem `DocumentOwnershipRuntime`;
- `pkg/yhttp.RemoteOwnerEndpoint` agora expõe hook de autenticação do handshake inter-node antes de abrir conexão no provider local do owner;
- `pkg/yhttp.Server` agora pode acoplar `DocumentOwnershipRuntime` para claim/release automático de lease por documento enquanto houver conexões locais, streams owner-side ou takeover local ativos;
- o handshake inter-node owner-side agora rejeita epoch obsoleto antes do ack e força relookup/cutover via `Close` retryable, com mapeamento HTTP `503` no edge quando isso ocorre antes do upgrade;
- a camada de segurança de `pkg/yhttp` já cobre hooks opt-in para autenticação, autorização, rate limit, quotas por conexão/frame, Origin/CORS, redaction, autenticação do handshake inter-node e validadores fail-closed explícitos para produção pública;
- o que segue em aberto fora do corte distribuído é versionar V2 em storage/protocolo se necessário, ajustar SLOs com dados reais de tráfego/topologia/tenant e hardening público multi-tenant.

#### Próximas frentes de segurança
- [ ] Evoluir `QuotaLimiter`/`LocalQuotaLimiter` para enforcement distribuído por tenant/documento/conexão, incluindo forwarding, owner lookup e custo de replay/storage
- [ ] Operacionalizar política inter-node obrigatória para produção: identidade de nó, HMAC/bearer/mTLS, escopos por operação, expiração curta e distribuição segura de chaves
- [ ] Definir redaction para logs, labels de métricas, payloads de erro HTTP/WS e dashboards, evitando expor namespaces, document ids, principals, tokens e connection ids crus

#### Em foco (abertura da fase)
- [x] Tratar `pkg/yprotocol.Provider` atual como runtime local do owner, sem fanout multi-process ad hoc e com bootstrap por `snapshot + update log`
- [x] Definir a fronteira entre a borda `pkg/yhttp` e a futura camada inter-node, explicitando o modo edge owner-aware
- [x] Promover `pkg/ynodeproto` de framing puro para camada de mensagens tipadas sem quebrar `Header`/`Frame` já expostos
- [x] Materializar backend/schema concretos para snapshot base, update log, placement e lease acima dos contratos públicos já definidos
- [x] Materializar fencing autoritativo na fronteira de storage (`snapshot`/`update log`) sem quebrar o runtime local atual
- [x] Ligar o fencing autoritativo ao `apply`/`Persist` do `pkg/yprotocol.Provider` e às respostas retryable de cutover na borda owner-aware
- [x] Definir regra inicial de recuperação após queda do owner: promoção só quando a lease atual está ausente/expirada, preservando bloqueio para owner remoto ativo
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
