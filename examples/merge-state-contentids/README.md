# examples/merge-state-contentids

Este exemplo mostra como usar a API pública de `pkg/yjsbridge` para operações de updates:

- `MergeUpdates`
- `DiffUpdate`
- `StateVectorFromUpdate` / `StateVectorFromUpdates`
- `EncodeStateVectorFromUpdates` + `DecodeStateVector`
- `CreateContentIDsFromUpdate` / `ContentIDsFromUpdates`
- `EncodeContentIDs` / `DecodeContentIDs`
- `DiffContentIDs`
- `IntersectUpdateWithContentIDsContext`

## Como executar

```bash
cd examples/merge-state-contentids
go run .
```

## O que o exemplo imprime

- `left`, `right` e `merged` em hex.
- state vectors derivados do `merged` e do agregado `left+right`.
- diff do `merged` contra o state vector de `left`, simulando o delta faltante para um peer atrasado.
- content ids individuais e agregados.
- serialização de content ids em hex com round-trip local.
- interseção para recuperar somente os trechos derivados de `left` e `right`.
- verificação de reconvergência reaplicando `left + diff`.

## Observação

Os payloads `left` e `right` são atualizações V1 sintéticas embutidas em hex para manter o exemplo totalmente auto-contido dentro do diretório `examples/`.
