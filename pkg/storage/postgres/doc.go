// Package postgres implementa persistência em PostgreSQL para os contratos
// públicos de `storage`, incluindo snapshots e stores auxiliares do runtime
// distribuído.
//
// O pacote usa `pgx/v5`, executa migrations SQL explícitas e mantém a
// superfície focada em salvar e restaurar `PersistedSnapshot` com payload V2
// preferencial e V1 como compatibilidade, além de update log, placement e
// leases.
package postgres
