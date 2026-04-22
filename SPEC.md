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
- preparar a evolução para lazy writer, snapshots binários e funcionalidades avançadas do YHub

---

## Objetivos secundários

- permitir construção futura de um servidor WebSocket compatível com Yjs
- permitir persistência e recuperação de snapshots binários
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

## Fase 1 — núcleo mínimo compatível

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

### Entregáveis
- endurecimento de compatibilidade de `merge/diff/intersect`
- lazy writer
- merge incremental de updates
- compatibilidade V2 e conversões de formato
- suporte melhor a snapshots binários

### Resultado esperado
Capacidade de:
- consolidar documento binário com menos gaps de compatibilidade
- responder syncs iniciais com consistência estrutural maior
- preparar backend para persistência e compaction

---

## Fase 3 — recursos compatíveis com YHub

### Entregáveis
- content ids avançado
- content maps
- attribution layer
- rollback
- activity
- changeset
- recursos auxiliares para auditoria e histórico

### Resultado esperado
Capacidade de:
- aproximar compatibilidade funcional com o YHub
- suportar recursos analíticos e operacionais avançados

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
- updates podem ser lidos no nível necessário
- state vector pode ser extraído de update
- content ids podem ser extraídos de update
- protocolo básico de sync está implementado
- awareness wire format está implementado
- merge/diff/interseção mínimos sobre update V1 existem com testes
- há testes cobrindo os principais casos

### Fase 2 pronta quando:
- múltiplos updates podem ser mesclados corretamente em casos estruturais mais amplos
- estado consolidado pode ser produzido com gaps de compatibilidade conhecidos documentados
- writer lazy e compatibilidade V2 têm corte funcional verificável

### Fase 3 pronta quando:
- attribution/content map funcionam de forma verificável
- endpoints e recursos equivalentes ao YHub podem ser sustentados por essa base

---

## Organização inicial sugerida

A estrutura de pacotes deve tender a algo próximo de:

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
```

Pacotes públicos em `pkg/` só devem surgir quando a API estiver estável.

## Decisões arquiteturais iniciais

### Decisão 1
Começar por compatibilidade de leitura e protocolo, não por servidor completo.

### Decisão 2
Implementar primeiro funções deriváveis diretamente de updates sem exigir materialização completa de `Y.Doc`.

### Decisão 3
Tratar content maps e attribution como etapa posterior.

### Decisão 4
Manter a implementação preparada para uso futuro em servidor estilo YHub, mas sem acoplar o núcleo a Redis, Postgres ou WebSocket logo de início.

## Riscos técnicos conhecidos

### 1. Compatibilidade binária incompleta
Pequenas divergências em parsing ou encoding podem quebrar compatibilidade.

### 2. Complexidade do merge
`mergeUpdates` e as operações de slice/diff/interseção são significativamente mais complexos que extração de state vector.

### 3. Diferenças entre formatos/versionamento
O Yjs possui detalhes de encoding que exigem leitura fiel do código.

### 4. Camada de attribution
Essa parte é mais rica e menos trivial do que o núcleo binário.

## Mitigação dos riscos

- implementar incrementalmente
- manter testes pequenos e precisos
- validar contra comportamento observado no código-fonte
- documentar toda hipótese não confirmada
- evitar pular etapas do núcleo

## Critério de sucesso do projeto

O projeto será bem-sucedido se conseguir, de forma incremental e testável:

- compreender updates Yjs em Go
- participar corretamente do protocolo básico de sync
- sustentar merge/diff/interseção mínimos e evoluir a compatibilidade de documento consolidado
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
8. attribution/content map
9. funcionalidades avançadas

O projeto deve evoluir por pequenas entregas, sempre testadas, sempre documentadas e sempre guiadas pelo comportamento real do código do Yjs/YHub.
