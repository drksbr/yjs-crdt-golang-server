# examples/memory

Este exemplo demonstra como usar a `memory` store de snapshots.

## O que ele faz

1. Cria uma instância `memory.New()`.
2. Gera um snapshot inicial vazio com `yjsbridge.PersistedSnapshotFromUpdates()`.
3. Salva o snapshot no store com uma chave de documento.
4. Lê o snapshot salvo e imprime:
   - chave do documento,
   - data de armazenamento,
   - tamanho do `UpdateV1`,
   - se o snapshot está vazio.

## Como executar

```bash
cd examples/memory
go run .
```

## Saída esperada (exemplo)

```
mem: notes/document-1
mem: salvo em 2006-01-02 15:04:05 +0000 UTC
mem: update_v1=2 bytes
mem: snapshot vazio=true
```

## Observação

Não há dependências externas adicionais; o objetivo é validar o contrato comum (`SnapshotStore`) com uma implementação local.
