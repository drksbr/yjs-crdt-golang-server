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
10. attribution/content maps
11. recursos avançados do YHub

As funções listadas abaixo devem ser entendidas como prioridades progressivas, e não como uma lista para implementação imediata de uma só vez.

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
- framing das mensagens do protocolo
- compatibilidade com `y-protocols`

### Motivação
Permite que o backend Go participe corretamente do fluxo de sincronização com clientes Yjs.

### Prioridade
Alta

### Status esperado
Corte mínimo implementado para V1

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
Codec wire format mínimo implementado; state manager de awareness segue pendente

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
Corte mínimo implementado para V1, com lazy writer interno mínimo; faltam integração mais ampla, V2 e maior cobertura estrutural

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
Corte mínimo implementado para `diff` e interseção por content ids; faltam V2, conversion e maior cobertura estrutural

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

## Ordem prática de implementação

O núcleo mínimo já entregue cobre:

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

Os próximos passos práticos ficam em:

1. ampliar compatibilidade estrutural de `merge/diff/intersect`
2. ampliar o uso do lazy writer além do corte mínimo atual
3. estudar/update V2 e conversion de formato
4. avançar para content maps
5. avançar para attribution
6. só então atacar recursos avançados do YHub

---

## Funções prioritárias imediatas

O backlog imediato agora deve focar em:

1. compatibilidade estrutural de `mergeUpdates`
2. compatibilidade estrutural de `diffUpdate`
3. interseção/filtragem por content ids em mais content refs
4. ampliar e endurecer o lazy writer (`writeStructToLazyStructWriter` / `finishLazyStructWriting`)
5. compatibilidade V2
6. `mergeContentIds`
7. `content maps`
8. attribution manager

---

## Dependências entre os grupos

### Dependência estrutural
- state vector depende de leitura de update
- content ids depende de leitura de update + id set
- merge depende de leitura de update + escrita lazy + delete set
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

Esses blocos definem o núcleo mínimo compatível.

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
7. endurecer compatibilidade com lazy writer e V2
8. só depois entrar em attribution e recursos avançados

Este documento deve ser atualizado sempre que uma nova função relevante do Yjs for estudada ou quando uma prioridade mudar.
