// Package postgres implementa persistência de snapshots em PostgreSQL para o
// contrato público `storage.SnapshotStore`.
//
// O pacote usa `pgx/v5`, executa migrations SQL explícitas e mantém a
// superfície focada em salvar e restaurar `PersistedSnapshot` canônicos em V1.
package postgres
