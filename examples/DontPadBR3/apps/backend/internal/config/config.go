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
	StorageBackend string
	S3Bucket       string
	S3Prefix       string
	S3Region       string
	S3Endpoint     string
	S3Profile      string
	S3PathStyle    string
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
	StorageBackend string
	S3Bucket       string
	S3Prefix       string
	S3Region       string
	S3Endpoint     string
	S3Profile      string
	S3PathStyle    bool
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

	storageBackend := strings.ToLower(strings.TrimSpace(firstNonEmpty(opts.StorageBackend, os.Getenv("DONTPAD_STORAGE_BACKEND"))))
	if storageBackend == "" {
		storageBackend = "local"
	}
	if storageBackend != "local" && storageBackend != "s3" {
		return Config{}, fmt.Errorf("DONTPAD_STORAGE_BACKEND invalido: %q", storageBackend)
	}

	s3Bucket := firstNonEmpty(opts.S3Bucket, os.Getenv("DONTPAD_S3_BUCKET"))
	s3Region := firstNonEmpty(opts.S3Region, os.Getenv("DONTPAD_S3_REGION"), os.Getenv("AWS_REGION"), os.Getenv("AWS_DEFAULT_REGION"))
	s3PathStyle, err := parseOptionalBool(firstNonEmpty(opts.S3PathStyle, os.Getenv("DONTPAD_S3_PATH_STYLE")))
	if err != nil {
		return Config{}, fmt.Errorf("DONTPAD_S3_PATH_STYLE invalido: %w", err)
	}
	if storageBackend == "s3" && strings.TrimSpace(s3Bucket) == "" {
		return Config{}, errors.New("defina DONTPAD_S3_BUCKET quando DONTPAD_STORAGE_BACKEND=s3")
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
		StorageBackend: storageBackend,
		S3Bucket:       s3Bucket,
		S3Prefix:       firstNonEmpty(opts.S3Prefix, os.Getenv("DONTPAD_S3_PREFIX")),
		S3Region:       s3Region,
		S3Endpoint:     firstNonEmpty(opts.S3Endpoint, os.Getenv("DONTPAD_S3_ENDPOINT")),
		S3Profile:      firstNonEmpty(opts.S3Profile, os.Getenv("DONTPAD_S3_PROFILE"), os.Getenv("AWS_PROFILE")),
		S3PathStyle:    s3PathStyle,
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
		"http://127.0.0.1:5173",
		"http://localhost:5173",
		"http://127.0.0.1:5174",
		"http://localhost:5174",
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

func parseOptionalBool(raw string) (bool, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return false, nil
	}
	switch raw {
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	case "0", "false", "f", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%q", raw)
	}
}
