package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/joho/godotenv"
)

// Options contains command-line overrides for backend configuration.
type Options struct {
	EnvFile        string
	Address        string
	DatabaseURL    string
	Schema         string
	Namespace      string
	DataDir        string
	AllowedOrigins string
	AuthSecret     string
	MasterPassword string
}

type Config struct {
	Address        string
	PostgresDSN    string
	Schema         string
	Namespace      string
	DataDir        string
	AllowedOrigins []string
	AuthSecret     string
	MasterPassword string
}

func LoadEnvFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) && filepath.Clean(path) == ".env" {
			return nil
		}
		return fmt.Errorf("carregar arquivo .env %q: %w", path, err)
	}
	values, err := godotenv.Read(path)
	if err != nil {
		return fmt.Errorf("carregar arquivo .env %q: %w", path, err)
	}
	for key, value := range values {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("aplicar variável %s de %q: %w", key, path, err)
		}
	}
	return nil
}

func Load(opts Options) (Config, error) {
	dsn := firstNonEmpty(opts.DatabaseURL, os.Getenv("DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("YJSBRIDGE_POSTGRES_DSN"))
	}
	if dsn == "" {
		return Config{}, errors.New("defina DATABASE_URL (ou YJSBRIDGE_POSTGRES_DSN)")
	}
	dsn = normalizePostgresDSN(dsn)

	address := firstNonEmpty(opts.Address, os.Getenv("DONTPAD_ADDR"))
	if address == "" {
		address = common.DefaultAddress
	}

	schema := firstNonEmpty(opts.Schema, os.Getenv("DONTPAD_SCHEMA"))
	if schema == "" {
		schema = common.DefaultSchema
	}
	if !common.SchemaIdentPattern.MatchString(schema) {
		return Config{}, fmt.Errorf("DONTPAD_SCHEMA invalido: %q", schema)
	}

	namespace := firstNonEmpty(opts.Namespace, os.Getenv("DONTPAD_NAMESPACE"))
	if namespace == "" {
		namespace = common.DefaultNamespace
	}

	dataDir := firstNonEmpty(opts.DataDir, os.Getenv("DONTPAD_DATA_DIR"))
	if dataDir == "" {
		dataDir = common.DefaultDataDir
	}

	authSecret := firstNonEmpty(opts.AuthSecret, os.Getenv("JWT_SECRET"))
	if authSecret == "" {
		authSecret = "dontpad-go-backend-dev-secret-change-me"
	}

	return Config{
		Address:        address,
		PostgresDSN:    dsn,
		Schema:         schema,
		Namespace:      namespace,
		DataDir:        dataDir,
		AllowedOrigins: loadAllowedOrigins(firstNonEmpty(opts.AllowedOrigins, os.Getenv("DONTPAD_ALLOWED_ORIGINS"))),
		AuthSecret:     authSecret,
		MasterPassword: firstNonEmpty(opts.MasterPassword, os.Getenv("MASTER_PASSWORD")),
	}, nil
}

func normalizePostgresDSN(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return raw
	}
	if parsed.Query().Get("sslmode") != "" {
		return raw
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "127.0.0.1" && host != "localhost" {
		return raw
	}
	query := parsed.Query()
	query.Set("sslmode", "disable")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func loadAllowedOrigins(raw string) []string {
	defaults := []string{
		"http://127.0.0.1:3000",
		"http://localhost:3000",
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaults
	}
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return defaults
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
