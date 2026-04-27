package postgres

import "testing"

func TestConfigNormalizationAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("normalized_defaults", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			ConnectionString: "postgres://localhost/db",
		}.normalized()

		if cfg.Schema != defaultSchema {
			t.Fatalf("normalized().Schema = %q, want %q", cfg.Schema, defaultSchema)
		}
		if cfg.ApplicationName != "yjs-go-bridge" {
			t.Fatalf("normalized().ApplicationName = %q, want yjs-go-bridge", cfg.ApplicationName)
		}
	})

	t.Run("normalized_preserves_custom_values", func(t *testing.T) {
		t.Parallel()

		cfg := Config{
			ConnectionString: "postgres://localhost/db",
			Schema:           "tenant_a",
			ApplicationName:  "my-service",
			MinConns:         2,
			MaxConns:         6,
		}.normalized()

		if cfg.Schema != "tenant_a" {
			t.Fatalf("normalized().Schema = %q, want %q", cfg.Schema, "tenant_a")
		}
		if cfg.ApplicationName != "my-service" {
			t.Fatalf("normalized().ApplicationName = %q, want my-service", cfg.ApplicationName)
		}
	})

	t.Run("valid_connections_and_limits", func(t *testing.T) {
		t.Parallel()

		tests := []Config{
			{ConnectionString: "postgres://localhost/db"},
			{ConnectionString: "  postgres://localhost/db  "},
			{ConnectionString: "postgres://localhost/db", MinConns: 2, MaxConns: 8},
			{ConnectionString: "postgres://localhost/db", MinConns: 2, MaxConns: 0},
		}

		for _, cfg := range tests {
			cfg := cfg
			t.Run(cfg.ConnectionString, func(t *testing.T) {
				t.Parallel()
				if err := cfg.validate(); err != nil {
					t.Fatalf("validate() unexpected error: %v", err)
				}
			})
		}
	})

	t.Run("invalid_configs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			cfg  Config
		}{
			{
				name: "empty_connection",
				cfg:  Config{},
			},
			{
				name: "blank_connection",
				cfg:  Config{ConnectionString: "   "},
			},
			{
				name: "negative_min_conns",
				cfg:  Config{ConnectionString: "postgres://localhost/db", MinConns: -1},
			},
			{
				name: "negative_max_conns",
				cfg:  Config{ConnectionString: "postgres://localhost/db", MaxConns: -1},
			},
			{
				name: "min_greater_than_max",
				cfg:  Config{ConnectionString: "postgres://localhost/db", MinConns: 10, MaxConns: 2},
			},
			{
				name: "schema_with_null_byte",
				cfg:  Config{ConnectionString: "postgres://localhost/db", Schema: "tenant\x00schema"},
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				if err := tt.cfg.validate(); err == nil {
					t.Fatalf("validate() error = nil, want error")
				}
			})
		}
	})
}

func TestQuoteIdentifierEscapesDoubleQuotes(t *testing.T) {
	t.Parallel()

	if got := quoteIdentifier(`tenant"schema`); got != `"tenant""schema"` {
		t.Fatalf("quoteIdentifier(%q) = %q, want %q", `tenant"schema`, got, `"tenant""schema"`)
	}
}
