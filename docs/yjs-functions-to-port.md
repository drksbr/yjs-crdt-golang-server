# docs/yjs-functions-to-port.md

## Objetivo

Listar e organizar as funções, estruturas e áreas do Yjs que precisam ser estudadas, reproduzidas ou adaptadas no port para Go.

Este documento serve para:

- definir prioridades técnicas
- separar núcleo mínimo de funcionalidades avançadas
- orientar a ordem de implementação
- evitar tentar portar tudo ao mesmo tempo

---

## Resumo executivo

O port para Go não deve começar pelo Yjs inteiro.

A estratégia correta é dividir o problema em camadas:

1. utilitários binários
2. parsing de update
3. state vector
4. id sets
5. content ids
6. protocolo de sync
7. awareness
8. merge/diff/interseção mínimos
9. lazy writer e compatibilidade binária ampliada
10. arquitetura distribuída por owner + protocolo inter-node
11. attribution/content maps
12. recursos avançados do YHub

As funções listadas abaixo devem ser entendidas como prioridades progressivas, e não como uma lista para implementação imediata de uma só vez.

## Status atual

Projeto em **Meta técnica 9 / Fase 3 em consolidação**, com promoção da API pública de update em V1 em `pkg/yjsbridge`, exposição pública de sync/awareness em V1 em `pkg/yprotocol` e `pkg/yawareness`, runtime in-process mínimo em `pkg/yprotocol` e camada mínima de provider no mesmo pacote. A próxima etapa aberta no roadmap é a **Meta técnica 10 / Fase 4**, dedicada à arquitetura distribuída por owner.

- Estruturas, parsing e protocolo de base já estão no plano mínimo operacional.
- O escopo em execução é consolidar compatibilidade estrutural de `merge/diff/intersect`, com lazy writer já endurecido no fluxo principal atual.
- A API pública de formato e merge já usa validação agregada com erro indexado por update.
- As superfícies públicas de sync (`pkg/yprotocol`) e awareness (`pkg/yawareness`) já estão promovidas para V1 com limites explícitos de suporte, sem provider completo e sem suporte a V2.
- `pkg/yprotocol` agora também cobre o primeiro runtime in-process mínimo de sessão/protocolo, com `Session`, `HandleProtocolMessage`, `HandleEncodedMessages` e encode público de `ProtocolMessage`, ainda sem transporte/provider completo.
- `pkg/yprotocol` agora também cobre a camada mínima de provider acima de `Session`, com `Provider`, `Open`, `Connection`, `DispatchResult`, `Persist` e `Close`, ainda sem provider completo, transporte distribuído ou V2.
- A API pública de persistência mínima já está disponível em `pkg/yjsbridge` para snapshots V1 (`PersistedSnapshot`, conversão, encode/decode de restore e persistência canônica).
- A promoção da API pública de update já está refletida em `pkg/yjsbridge` (operações de merge/diff/intersect e derivados como state vector/content ids), mantendo `V2` explicitamente fora de escopo.
- A camada de armazenamento operacional já está definida em `pkg/storage` (contratos: `SnapshotStore`, `DocumentKey`, `SnapshotRecord`, erros) com stores implementadas para memória e Postgres.
- Existem exemplos iniciais de integração em `examples/memory` e `examples/postgres` cobrindo save/load com a API pública.
- As APIs públicas de inspeção por `single-update` já despacham V1 e rejeitam V2 de forma explícita.
- O caminho de merge agora também possui variantes com `context` e agregação paralela na etapa de fusão.
- Os caminhos públicos de `diff` e `intersect` agora também possuem variantes com `context`.
- A pré-validação agregada de formato agora também respeita `context` nas APIs multi-update.
- A suíte de merge agora cobre preenchimento parcial de gaps já materializados como `Skip`, inclusive quando o `Skip` vem do input.
- Os helpers internos de merge agora também têm guardas defensivas para listas vazias, com cobertura dedicada para evitar `panic` fora do caminho feliz.
- `EncodeV1` agora normaliza structs fora de ordem por clock dentro do mesmo cliente antes de serializar, endurecendo o fluxo do lazy writer quando recebe `DecodedUpdate` materializado fora da ordem canônica.
- Os caminhos de slice em `merge` e `ParsedContent` agora usam aritmética segura para evitar wraparound em `uint32` antes de calcular a janela efetiva.
- As APIs derivadas de `state vector` e `content ids` já respeitam a mesma pré-validação agregada de formato e preservam o índice do primeiro payload relevante ao rejeitar V2.
- A suíte estrutural já cobre comutatividade de `merge`, gaps sintéticos, workflow multi-client de `merge -> diff -> intersect`, contratos adicionais do `lazy writer` e round-trip mais pesado do writer incremental.
- A borda pública atual de V2 também ganhou contratos agregados extras para rejeição após `nil`/vazios e precedência de mistura `V1/V2` em `MergeUpdates`.
- `MergeUpdates` agora também indexa o payload relevante quando rejeita V2 detectado, e a suíte passou a cobrir refs não fatiáveis em `ParsedContent.SliceWindow`, erros de `sliceStructWindowV1` e continuação de merge após `Skip` explícito no overlap.
- `MergeUpdates` também passou a tratar listas compostas apenas por payloads vazios como `no-op`, mantendo o retorno de update V1 vazio, e a suíte cobre payload V1 malformado nos caminhos context-aware de merge e validação agregada.
- Já existe um primeiro corte funcional de `conversion/snapshots` em V1: `ConvertUpdateToV1` e `ConvertUpdatesToV1` normalizam payloads suportados para a forma canônica, e `SnapshotFromUpdate(s)` extrai `state vector + delete set` em memória a partir de update(s) V1 agregados.
- Novo corte funcional em V1 de snapshot binário persistido já existe: `PersistedSnapshot` com `PersistedSnapshotFromUpdate(s)` gera, em um passo, `UpdateV1` canônico persistível e `Snapshot` materializado em memória.
- Há também corte funcional de hidratação reversa V1 (`restore`) para `PersistedSnapshot` com `EncodePersistedSnapshotV1`, `DecodePersistedSnapshotV1` e `DecodePersistedSnapshotV1Context`.
- O ciclo de persistência operacional já está em funcionamento via stores; `pkg/yprotocol.Provider` passa a servir como runtime local do futuro owner distribuído.
- O próximo bloco de esforço agora combina duas frentes: fechar lacunas de `merge/diff/intersect` e V2 no runtime local, e abrir a arquitetura distribuída com owner único por documento/shard, lease/epoch/fencing, `snapshot + update log`, protocolo inter-node próprio e aceite de HTTP/WS em qualquer nó.

---

## Grupo 1 — utilitários binários e base estrutural

Estas áreas são pré-requisitos para quase todo o restante.

### Componentes a portar ou reproduzir
- leitura segura de bytes
- leitura e escrita de varuint
- encoders/decoders binários básicos
- tipos de erro para parsing
- helpers de bounds check
- representação de IDs (`client`, `clock`)

### Motivação
Sem isso, não é possível ler updates, delete sets ou mensagens de protocolo.

### Prioridade
Máxima

### Status esperado
Implementar primeiro

---

## Grupo 2 — structs do update

Estas estruturas aparecem no fluxo interno do parsing de updates do Yjs.

### Estruturas principais
- `Item`
- `GC`
- `Skip`

### Áreas relacionadas
- `ID`
- leitura de info flags
- comprimento (`length`)
- slicing parcial de structs
- classificação do tipo de struct

### Motivação
Esses structs são a base para leitura de updates, state vectors, merges e content ids.

### Prioridade
Máxima

### Status esperado
Implementar logo após a base binária

---

## Grupo 3 — delete set e id set

Essas estruturas são fundamentais para deletes, content ids e merges.

### Estruturas/funções principais
- `IdSet`
- `createIdSet`
- `readIdSet`
- `writeIdSet`
- `insertIntoIdSet`
- `mergeIdSets`
- interseção de sets
- diferença de sets
- normalização de ranges
- iteração por client/range

### Motivação
O Yjs usa sets de ranges por client para representar partes importantes do estado binário.

### Prioridade
Muito alta

### Status esperado
Implementar cedo, antes de content ids e merge

---

## Grupo 4 — leitura de update

Aqui entram as estruturas que percorrem updates de forma lazy.

### Estruturas/funções principais
- `LazyReaderV1`
- `NewLazyReaderV1`
- `DecodeV1`
- `ReadDeleteSet`
- decoder de update V1
- eventualmente decoder V2
- leitura do bloco de structs
- leitura do delete set ao final do update

### Motivação
Essa camada é necessária para:
- `encodeStateVectorFromUpdate`
- `createContentIdsFromUpdate`
- `mergeUpdates`

### Prioridade
Muito alta

### Status esperado
Corte mínimo implementado em V1; writer lazy e V2 seguem pendentes

---

## Grupo 5 — state vector

Primeiro grande recurso derivado do parsing de update.

### Funções principais
- `encodeStateVectorFromUpdate`
- eventualmente `encodeStateVectorFromUpdateV2`

### O que faz
Percorre os structs do update e computa, por `client`, o maior clock contínuo conhecido.

### Motivação
É usado pelo YHub no sync inicial e é parte essencial do protocolo de sync do Yjs.

### Prioridade
Muito alta

### Status esperado
Uma das primeiras funções de alto valor a serem implementadas

---

## Grupo 6 — content ids

Camada importante para metadados estruturais e futura attribution layer.

### Funções principais
- `createContentIdsFromUpdate`
- `createContentIdsFromUpdateV2`
- `mergeContentIds`
- `encodeContentIds`
- `decodeContentIds`

### O que faz
Extrai, representa e combina ranges de conteúdo inserido e deletado.

### Motivação
O YHub usa content ids diretamente ao montar documento e ao gerar metadata.

### Prioridade
Alta

### Status esperado
Extração básica implementada; merge/encode/decode de content ids seguem pendentes

---

## Grupo 7 — protocolo de sync

Essencial para comunicação compatível com clientes Yjs.

### Funções/áreas principais
- `SyncStep1`
- `SyncStep2`
- mensagem de update do sync (`SyncUpdate`, `EncodeSyncUpdate`, `EncodeProtocolSyncUpdate`)
- encode/decode de mensagens de sync
- encode público tipado de `ProtocolMessage`
- framing das mensagens do protocolo
- envelope externo do provider websocket (`sync`, `awareness`, `auth`, `queryAwareness`)
- runtime in-process mínimo (`Session`, `HandleProtocolMessage`, `HandleEncodedMessages`)
- camada mínima de provider (`Provider`, `Open`, `Connection`, `DispatchResult`, `Persist`, `Close`)
- compatibilidade com `y-protocols`

### Motivação
Permite que o backend Go participe corretamente do fluxo de sincronização com clientes Yjs.

### Prioridade
Alta

### Status esperado
Corte mínimo implementado para V1 com superfície pública em `pkg/yprotocol` (`SyncStep1`/`SyncStep2`, envelope websocket, encode público de `ProtocolMessage`, runtime in-process mínimo com `Session`/handlers e camada mínima de provider), sem provider completo

---

## Grupo 8 — awareness

Camada efêmera de presença.

### Funções/áreas principais
- encode/decode de mensagem awareness
- merge de awareness updates
- remoção/desconexão de usuário
- atualização de clock de awareness

### Motivação
O YHub trata awareness separadamente do documento principal e isso deve ser preservado.

### Prioridade
Alta

### Status esperado
Codec wire format, state manager básico e superfície pública em `pkg/yawareness` implementados para V1; deltas/eventos e operadores auxiliares seguem pendentes, sem provider completo

---

## Grupo 9 — merge de updates

Uma das áreas mais importantes e mais complexas do port.

### Funções principais
- `mergeUpdates`
- `mergeUpdatesV2`
- lazy writer
- `writeStructToLazyStructWriter`
- `finishLazyStructWriting`
- slicing de structs
- ordenação e compactação por client/clock

### O que faz
Mescla updates binários removendo redundâncias e produzindo um update consolidado.

### Motivação
É indispensável para:
- documento consolidado server-side
- snapshots
- sync inicial robusto
- comportamento semelhante ao YHub

### Prioridade
Alta, mas posterior ao núcleo de leitura

### Status esperado
Corte mínimo implementado para V1, com lazy writer interno já endurecido no corte atual; faltam ampliar integração remanescente, V2 e maior cobertura estrutural

---

## Grupo 10 — diff e operações binárias correlatas

Essas funções costumam ser úteis depois do merge.

### Funções/áreas principais
- diff baseado em state vector
- filtragem/interseção por content ids
- update format conversion
- update obfuscation (baixa prioridade)
- helpers de subset/intersection

### Motivação
Ajudam a evoluir do núcleo de parsing para uma engine binária mais completa.

### Prioridade
Média

### Status esperado
Corte mínimo implementado para `diff` e interseção por content ids; classificação mínima de V2 já existe, mas faltam suporte V2 real, conversion e maior cobertura estrutural

---

## Grupo 11 — content maps

Camada avançada usada pelo YHub.

### Funções principais
- `encodeContentMap`
- `decodeContentMap`
- `mergeContentMaps`
- `excludeContentMap`
- `createContentMapFromContentIds`

### O que faz
Representa metadata atribuída a conteúdo inserido/deletado.

### Motivação
Base para:
- attribution
- rollback
- activity
- changeset

### Prioridade
Posterior

### Status esperado
Somente após núcleo de sync/merge estar validado

---

## Grupo 12 — attribution layer

Camada analítica/operacional avançada.

### Áreas principais
- attribution manager
- metadata de autor
- metadata temporal
- filtros por attribution
- lookup de activity
- suporte a rollback baseado em ranges atribuídos

### Motivação
Necessária para compatibilidade funcional avançada com o YHub.

### Prioridade
Posterior

### Status esperado
Fase avançada do projeto

---

## Grupo 13 — operações avançadas inspiradas no YHub

Essas operações dependem das camadas anteriores.

### Operações principais
- patch de documento
- rollback
- activity
- changeset
- compactação com ou sem GC
- reconstrução de documento consolidado

### Motivação
Representam o nível de compatibilidade operacional necessário para chegar perto do YHub completo.

### Prioridade
Posterior

### Status esperado
Só depois do núcleo binário e de attribution estar funcional

---

## Grupo 14 — coordenação distribuída e ownership

Camada que transforma o runtime local atual em execução multi-nó sem duplicar rooms.

### Funções/áreas principais
- resolução de owner por documento/shard
- aquisição, renovação, revogação e expiração de lease
- `epoch` monotônico e fencing token
- roteamento de HTTP/WS de qualquer nó para o owner
- forwarding de frames do cliente e de respostas do owner
- handoff de room e cutover entre owners

### Motivação
Permite que todos os nós aceitem tráfego de cliente, mas só um owner processe cada room por vez.

### Prioridade
Alta na próxima fase

### Status esperado
Primeiro grande bloco da Meta 10; deve reutilizar `pkg/yprotocol.Provider` como runtime local do owner, sem criar fanout multi-process ad hoc

---

## Grupo 15 — `snapshot + update log` e replay autoritativo

Camada de durabilidade incremental para recovery, restart e troca de owner.

### Funções/áreas principais
- snapshot base persistido
- append-only update log por room/epoch
- replay determinístico de log sobre snapshot
- checkpoint/compaction
- bootstrap do owner a partir de snapshot + tail do log
- envio/catch-up de snapshot/log no handoff

### Motivação
Sem `snapshot + update log`, mudança de owner e recuperação pós-falha ficam frágeis ou exigem reconstrução cara e pouco previsível.

### Prioridade
Alta na próxima fase

### Status esperado
Planejado para Meta 10 em conjunto com lease/`epoch`/fencing e protocolo inter-node próprio

---

## Ordem prática de implementação

O núcleo consolidado para Meta 9 já cobre:

1. base binária
2. varuint
3. structs mínimos (`Item`, `GC`, `Skip`)
4. `IdSet` e delete set
5. `LazyReaderV1` / leitura mínima de update
6. `encodeStateVectorFromUpdate`
7. `createContentIdsFromUpdate`
8. protocolo de sync
9. awareness wire format
10. `mergeUpdates`, `diffUpdate` e interseção mínima por content ids
11. corte funcional de persistência e hidratação reversa (`PersistedSnapshot`) em V1

Os próximos passos práticos (Meta 9 -> Meta 10) ficam em:

1. congelar `pkg/yprotocol.Provider` como runtime local do futuro owner distribuído
2. definir owner único por documento/shard com lease, `epoch` e fencing
3. definir `snapshot + update log` para replay, recovery e handoff
4. definir protocolo inter-node próprio separado do `y-protocols`
5. adaptar a borda HTTP/WS para aceitar conexões em qualquer nó e encaminhar o room ao owner
6. ampliar compatibilidade estrutural de `merge/diff/intersect`
7. ampliar o uso do lazy writer além do corte já endurecido
8. estudar/update V2 e conversion de formato além da classificação mínima atual
9. avançar para content maps
10. avançar para attribution
11. só então atacar recursos avançados do YHub

---

## Funções prioritárias imediatas

O backlog imediato agora deve focar em:

1. definir a unidade de ownership (`DocumentKey`/room/shard) e o contrato de lease/`epoch`/fencing
2. definir o modelo `snapshot + update log` para hidratação, replay, restart e troca de owner
3. definir o protocolo inter-node próprio para resolve/open/forward/handoff/recovery
4. tratar `pkg/yprotocol.Provider` como runtime local do owner e `pkg/yhttp` como borda pública em qualquer nó
5. compatibilidade estrutural de `mergeUpdates`
6. compatibilidade estrutural de `diffUpdate`
7. interseção/filtragem por content ids em mais content refs
8. ampliar o endurecimento do lazy writer para os fluxos restantes (`writeStructToLazyStructWriter` / `finishLazyStructWriting`)
9. compatibilidade V2 além da classificação mínima
10. `mergeContentIds`
11. `content maps`
12. attribution manager

---

## Dependências entre os grupos

### Dependência estrutural
- state vector depende de leitura de update
- content ids depende de leitura de update + id set
- merge depende de leitura de update + escrita lazy + delete set
- ownership distribuído depende do runtime local de sync/awareness já estabilizado
- `snapshot + update log` depende de merge/persistência canônica e de invariantes de replay
- handoff entre owners depende de lease, `epoch`/fencing e replay determinístico
- content map depende de content ids
- attribution depende de content map
- rollback/activity/changeset dependem de attribution e documento consolidado

---

## O que pode ser adiado sem travar o projeto

Pode ficar para depois:
- V2 completo se V1 bastar no começo
- content maps
- attribution
- patch/rollback/activity/changeset
- obfuscation e conversões menos centrais

---

## O que não pode ser adiado

Não deve ser adiado:
- binário seguro
- varuint
- parsing mínimo de update
- state vector
- id sets
- content ids
- sync protocol
- awareness wire format
- merge/diff/interseção mínimos em V1

Ao entrar na fase distribuída, também não deve ser adiado:
- owner único por room/documento/shard
- lease, `epoch` e fencing
- `snapshot + update log` com replay determinístico
- protocolo inter-node próprio separado do wire de cliente

Esses blocos definem o núcleo compatível já estabelecido para a Fase 2.

---

## Critério de sucesso por grupo

Cada grupo só deve ser considerado concluído quando:

- houver implementação
- houver testes
- houver documentação mínima
- o comportamento não contradisser os achados do código do Yjs/YHub
- a compatibilidade alegada for demonstrável

---

## Resumo final

As funções e áreas a portar devem ser tratadas como camadas dependentes.

O caminho correto é:

1. ler binário
2. entender structs
3. extrair state vector
4. extrair content ids
5. falar o protocolo de sync
6. consolidar merge/diff/interseção mínimos
7. ampliar compatibilidade remanescente com lazy writer e V2
8. introduzir ownership distribuído com `snapshot + update log` e protocolo inter-node
9. só depois entrar em attribution e recursos avançados

Este documento deve ser atualizado sempre que uma nova função relevante do Yjs for estudada ou quando uma prioridade mudar.
