# TASK.md

## Estado atual

Projeto inicializado com base documental consolidada e primeira entrega técnica em Go já implementada.

Neste momento o repositório já possui:
- módulo Go inicializado
- pacote `internal/binary` com leitura sequencial segura
- pacote `internal/varint` com encode/decode de `varuint`
- pacote `internal/ytypes` com `ID`, `Item`, `GC`, `Skip` e `DeleteSet` básico
- pacote `internal/yidset` com ranges normalizados por cliente
- pacote `internal/yupdate` com leitura lazy de update V1, decode de delete set, state vector, content ids, reencode V1, merge, diff e interseção por content ids
- pacote `internal/yprotocol` com wire format mínimo do sync protocol
- pacote `internal/yawareness` com wire format mínimo de awareness
- cobertura ampliada para round-trip de content refs, metadata wire de `Item` e slicing estrutural de `JSON`/`Any`
- documentação de roadmap alinhada ao estado real do código
- testes cobrindo casos válidos e inválidos

A fase atual passa de alinhamento puro para implementação incremental do núcleo binário.
As metas técnicas 1, 2, 3, 4, 5, 6, 7 e 8 já possuem corte mínimo implementado.

---

## Objetivo da fase atual

Preparar o projeto para começar a implementação do núcleo mínimo compatível com Yjs em Go.

Essa fase deve terminar com:

- documentação-base pronta
- escopo inicial fechado
- ordem de implementação definida
- primeira unidade técnica pronta para desenvolvimento

---

## Fase atual

### Fase 0 — preparação e alinhamento técnico

#### Entregáveis desta fase
- [x] Criar estrutura documental do projeto
- [x] Criar `AGENT.md`
- [x] Criar `SPEC.md`
- [x] Criar `docs/yjs-go-port-context.md`
- [x] Criar `docs/yhub-code-findings.md`
- [x] Criar `docs/yjs-functions-to-port.md`
- [x] Consolidar plano inicial de pacotes Go
- [x] Definir primeira entrega técnica implementável
- [x] Inicializar módulo Go
- [x] Criar estrutura inicial de diretórios `internal/`

---

## Primeira meta técnica

A primeira meta técnica deve ser pequena, útil e altamente verificável.

### Meta técnica 1
Implementar a infraestrutura mínima para leitura binária necessária para futuras funções do Yjs.

#### Escopo da meta técnica 1
- helper de leitura de bytes
- helper de leitura segura com checagem de bounds
- encode/decode canônico de varuint compatível com a faixa efetiva usada pelo Yjs/lib0
- tipos básicos de erro de parsing
- testes cobrindo casos válidos e inválidos

#### Fora do escopo desta meta
- parser completo de update
- `IdSet`
- state vector
- sync protocol
- awareness
- merge de updates
- infraestrutura de servidor/storage

#### Critério de pronto
- existe pacote binário mínimo
- existe pacote de varint mínimo
- há round-trip test para `varuint`
- todos os testes passam
- não há panic em input malformado
- bounds, offsets e tamanho de leitura são validados
- arquivos respeitam limite de tamanho
- documentação não contradiz a implementação

---

## Próximas metas sugeridas

### Meta técnica 9
Ampliar merge/diff binário e reduzir gaps de compatibilidade estrutural

Situação atual da meta 9:
- cobertura ampliada para `JSON` e `Any` em `merge/diff/intersect`
- round-trip adicional para `Binary`, `Embed`, `Format` e metadata wire de `Item`
- `intersect` alinhado à semântica observada no upstream do Yjs para seleção por `nextClock`
- slicing de `ContentString` alinhado ao comportamento do Yjs em fronteiras inválidas de surrogate pair (`U+FFFD` em vez de erro)
- lazy writer V1 interno introduzido e integrado no caminho de `EncodeV1`, `DiffUpdateV1` e `IntersectUpdateWithContentIDsV1`
- `AGENT.md`, `SPEC.md` e `docs/yjs-functions-to-port.md` sincronizados com o estado real do projeto

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
