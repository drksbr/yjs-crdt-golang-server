```md
# AGENT.md

## Objetivo

Construir um pacote em Go que funcione como camada de comunicação e compatibilidade com Yjs, servindo como base para um servidor 100% compatível com o ecossistema Yjs/YHub.

Este projeto deve priorizar compatibilidade binária, modularidade, testabilidade e clareza arquitetural.

---

## Missão do agente

Você é um agente de engenharia responsável por projetar e implementar, em Go puro, componentes compatíveis com o protocolo e com as estruturas binárias utilizadas pelo Yjs no servidor.

Seu trabalho não é criar uma abstração genérica de colaboração em tempo real.  
Seu trabalho é construir uma base técnica compatível com Yjs, com foco inicial em parsing, encoding, sync e manipulação de updates.

---

## Fontes de verdade

Sempre trate como fonte principal:

1. Código-fonte do Yjs
2. Código-fonte do YHub
3. Protocolo oficial do Yjs / y-protocols
4. Especificações e documentos deste repositório

Nunca assuma comportamento interno sem confirmar no código-fonte ou na documentação oficial.

---

## Documentos obrigatórios antes de agir

Antes de qualquer implementação, sempre ler nesta ordem:

1. `AGENT.md`
2. `SPEC.md`
3. `TASK.md`
4. `docs/yjs-go-port-context.md`
5. `docs/yhub-code-findings.md`
6. `docs/yjs-functions-to-port.md`

Se houver contradição:
- `SPEC.md` define a direção arquitetural
- `TASK.md` define o escopo atual
- documentos em `docs/` fornecem contexto técnico e achados de pesquisa

---

## Regras de implementação

### Estrutura de código
- Nunca criar arquivos com mais de 300 linhas.
- Separar parsing binário, protocolo, tipos, compatibilidade e testes em pacotes distintos.
- Evitar arquivos “god object”.
- Preferir composição e interfaces pequenas.

### Compatibilidade
- Priorizar compatibilidade com o comportamento observado no Yjs/YHub.
- Não inventar simplificações sem registrar explicitamente no código e na documentação.
- Toda decisão que altere compatibilidade deve ser documentada.

### Testes
- Toda funcionalidade binária deve ter teste.
- Toda codificação/decodificação deve ter round-trip test quando aplicável.
- Toda lógica de merge/diff/intersection deve ter testes com casos de borda.
- Sempre adicionar testes antes ou junto da implementação.
- Testes devem cobrir casos mínimos, casos concorrentes e casos inválidos quando possível.

### Comentários
- Escrever comentários técnicos em português.
- Explicar especialmente:
  - formatos binários
  - invariantes
  - decisões de compatibilidade
  - diferenças entre comportamento observado e comportamento implementado

### Dependências
- Preferir Go puro.
- Não depender de runtime Node.js para a execução normal do projeto.
- Evitar dependências externas desnecessárias.
- Se alguma dependência for realmente necessária, justificar no código e na documentação.

### Segurança e robustez
- Nunca confiar em input binário arbitrário.
- Validar tamanhos, ranges, varints e offsets.
- Tratar parsing malformado como erro explícito.
- Evitar panics em dados externos.

---

## Escopo técnico prioritário

### Fase inicial
Implementar primeiro os blocos necessários para compatibilidade básica de comunicação:

1. utilitários binários
2. varint encode/decode
3. decoder básico de update Yjs
4. tipos estruturais mínimos
5. state vector a partir de update
6. content ids a partir de update
7. protocolo de sync básico
8. awareness wire format
9. merge/diff/interseção mínimos sobre updates V1

### Fase intermediária
Depois evoluir para:

1. endurecimento de compatibilidade de `merge/diff/intersect`
2. writers lazy / merge incremental
3. suporte melhor a snapshots binários
4. compatibilidade V2 e conversões de formato

### Fase avançada
Somente depois atacar:

1. content maps
2. attribution layer
3. rollback
4. activity
5. changeset
6. recursos avançados inspirados no YHub

---

## Funções e áreas prioritárias do Yjs

O agente deve priorizar estudo e eventual porte destas áreas:

- `LazyReaderV1` / decode de update V1
- `encodeStateVectorFromUpdate`
- `createContentIdsFromUpdate`
- `mergeUpdates`
- `diffUpdate`
- interseção/filtragem por content ids
- `IdSet`
- `mergeIdSets`
- `insertIntoIdSet`
- protocolo de sync
- awareness protocol

Em fases posteriores:

- `mergeContentIds`
- lazy writer / `finishLazyStructWriting`
- `mergeContentMaps`
- `excludeContentMap`
- `createContentMapFromContentIds`
- attribution manager e recursos correlatos

---

## Fluxo esperado de trabalho

Para cada tarefa, siga este fluxo:

1. Ler os documentos obrigatórios
2. Resumir entendimento do problema
3. Propor arquitetura ou alteração mínima
4. Listar riscos técnicos
5. Implementar de forma incremental
6. Criar ou atualizar testes
7. Atualizar documentação relevante
8. Atualizar `TASK.md` com status do que foi feito

---

## O que fazer antes de codar

Antes de começar qualquer implementação relevante, o agente deve informar:

1. o que entendeu do objetivo
2. quais arquivos pretende criar ou alterar
3. quais invariantes precisam ser preservados
4. quais riscos existem de incompatibilidade
5. qual será o critério de pronto

---

## O que evitar

- Não criar abstrações amplas cedo demais.
- Não misturar protocolo WebSocket com parser binário.
- Não misturar lógica de storage com lógica de update.
- Não reimplementar o projeto inteiro antes de validar os blocos mínimos.
- Não mudar escopo da tarefa atual por conta própria.
- Não assumir que “parece funcionar” é compatibilidade suficiente.

---

## Organização sugerida de pacotes

A organização exata pode evoluir, mas a tendência deve seguir separação semelhante a:

- `internal/binary`
- `internal/varint`
- `internal/yupdate`
- `internal/ytypes`
- `internal/yidset`
- `internal/yprotocol`
- `internal/yawareness`
- `internal/ycompat`
- `pkg/...` apenas quando houver API pública estável

---

## Critério de qualidade

Uma entrega só deve ser considerada pronta quando:

- compila
- tem testes
- está documentada o suficiente
- respeita limite de tamanho dos arquivos
- não introduz suposições não verificadas
- mantém coerência com `SPEC.md` e `TASK.md`

---

## Critério de compatibilidade

Compatibilidade significa, sempre que possível:

- interpretar corretamente updates produzidos por clientes Yjs
- gerar estruturas coerentes com o protocolo esperado
- reproduzir resultados equivalentes aos utilitários relevantes do Yjs
- manter comportamento determinístico em parsing e encoding

Compatibilidade não deve ser declarada sem teste.

---

## Entregas incrementais

O agente deve preferir pequenas entregas completas em vez de grandes blocos incompletos.

Cada entrega deve:
- fechar uma unidade técnica pequena
- incluir testes
- atualizar documentação
- deixar o repositório em estado íntegro

---

## Atualização de contexto

Sempre que concluir uma etapa:
- atualizar `TASK.md`
- registrar decisões relevantes
- apontar pendências e riscos remanescentes

Se uma descoberta importante mudar o entendimento do projeto, registrar também em `SPEC.md` ou nos documentos em `docs/`.

---

## Regra final

Na dúvida, priorize:
1. compatibilidade observável
2. simplicidade estrutural
3. testes
4. documentação
5. expansão incremental
```
