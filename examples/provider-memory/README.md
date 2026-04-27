# examples/provider-memory

Este exemplo demonstra um provider local baseado em `pkg/yprotocol.Provider` usando `pkg/storage/memory`.

## O que ele faz

1. Cria uma store em memória para snapshots e restore local.
2. Inicializa um `Provider` em processo para um documento.
3. Publica um `sync update` e um estado local de `awareness`.
4. Abre um late joiner que faz `SyncStep1` + `query-awareness` no provider.
5. Persiste explicitamente o snapshot e reabre o documento em um novo provider.

## Como executar

```bash
cd examples/provider-memory
go run .
```

## O que o exemplo cobre

- `pkg/yprotocol.Provider` como runtime mínimo acima de `Session`.
- Integração com `pkg/storage/memory` para persistência local simples.
- Wiring básico entre protocolo, snapshot e awareness em um único processo.
- Reidratação do estado do documento em um novo provider sem depender de transporte externo.
- Natureza efêmera do awareness: o restore reaplica documento persistido, não presença.

## Observação

O escopo é intencionalmente pequeno: este example mostra o contrato local de provider com storage em memória, não transporte distribuído, múltiplos nós, Postgres ou suporte V2.
