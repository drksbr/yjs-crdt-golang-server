package postgres

import (
	"fmt"
	"strings"
)

const defaultSchema = "yjs_bridge"

// Config define a configuração mínima do store Postgres.
type Config struct {
	// ConnectionString é a string de conexão no formato aceito pelo pgx.
	ConnectionString string
	// Schema define o schema do banco onde ficam as tabelas do store.
	Schema string
	// ApplicationName é aplicado em `runtime.application_name` da conexão.
	ApplicationName string
	// MinConns limita o mínimo de conexões no pool.
	MinConns int32
	// MaxConns limita o máximo de conexões no pool.
	MaxConns int32
	// SkipMigrations evita execução automática de migrations no boot.
	SkipMigrations bool
}

func (c Config) normalized() Config {
	if strings.TrimSpace(c.Schema) == "" {
		c.Schema = defaultSchema
	}
	if strings.TrimSpace(c.ApplicationName) == "" {
		c.ApplicationName = "yjs-crdt-golang-server"
	}
	return c
}

func (c Config) validate() error {
	if strings.TrimSpace(c.ConnectionString) == "" {
		return fmt.Errorf("postgres: connection string obrigatoria")
	}
	if c.MinConns < 0 {
		return fmt.Errorf("postgres: MinConns deve ser >= 0")
	}
	if c.MaxConns < 0 {
		return fmt.Errorf("postgres: MaxConns deve ser >= 0")
	}
	if c.MaxConns > 0 && c.MinConns > c.MaxConns {
		return fmt.Errorf("postgres: MinConns nao pode ser maior que MaxConns")
	}
	if strings.ContainsRune(c.Schema, 0) {
		return fmt.Errorf("postgres: schema contem byte nulo")
	}
	return nil
}

func quoteIdentifier(id string) string {
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}
