package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

func TestLoadUsesDefaultsFromEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://postgres@127.0.0.1:5432/dontpadbr3")

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Address != common.DefaultAddress {
		t.Fatalf("Address = %q, want %q", cfg.Address, common.DefaultAddress)
	}
	if cfg.Schema != common.DefaultSchema {
		t.Fatalf("Schema = %q, want %q", cfg.Schema, common.DefaultSchema)
	}
	if cfg.Namespace != common.DefaultNamespace {
		t.Fatalf("Namespace = %q, want %q", cfg.Namespace, common.DefaultNamespace)
	}
	if cfg.DataDir != common.DefaultDataDir {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, common.DefaultDataDir)
	}
	if cfg.StorageBackend != "local" {
		t.Fatalf("StorageBackend = %q, want local", cfg.StorageBackend)
	}
	if !strings.Contains(cfg.PostgresDSN, "sslmode=disable") {
		t.Fatalf("PostgresDSN = %q, want local DSN with sslmode=disable", cfg.PostgresDSN)
	}
	wantOrigins := []string{
		"http://127.0.0.1:3000",
		"http://localhost:3000",
		"http://127.0.0.1:5173",
		"http://localhost:5173",
		"http://127.0.0.1:5174",
		"http://localhost:5174",
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, wantOrigins) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", cfg.AllowedOrigins, wantOrigins)
	}
}

func TestLoadFlagOverridesEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://env@remotehost:5432/envdb")
	t.Setenv("DONTPAD_ADDR", ":9000")
	t.Setenv("DONTPAD_SCHEMA", "envschema")
	t.Setenv("DONTPAD_ALLOWED_ORIGINS", "http://env.local")

	cfg, err := Load(Options{
		DatabaseURL:    "postgres://flag@remotehost:5432/flagdb",
		Address:        ":7070",
		Schema:         "flagschema",
		Namespace:      "flagns",
		DataDir:        "/tmp/dontpad-data",
		StorageBackend: "s3",
		S3Bucket:       "flag-bucket",
		S3Prefix:       "flag-prefix",
		S3Region:       "us-east-1",
		S3Endpoint:     "http://minio.local:9000",
		S3Profile:      "flag-profile",
		S3PathStyle:    "true",
		AllowedOrigins: "http://one.local, http://one.local, http://two.local",
		AuthSecret:     "flag-secret",
		MasterPassword: "flag-master",
	})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Address != ":7070" || cfg.Schema != "flagschema" || cfg.Namespace != "flagns" {
		t.Fatalf("config flags not applied: %#v", cfg)
	}
	if cfg.PostgresDSN != "postgres://flag@remotehost:5432/flagdb" {
		t.Fatalf("PostgresDSN = %q, want flag DSN", cfg.PostgresDSN)
	}
	if cfg.DataDir != "/tmp/dontpad-data" || cfg.AuthSecret != "flag-secret" || cfg.MasterPassword != "flag-master" {
		t.Fatalf("config secret/storage flags not applied: %#v", cfg)
	}
	if cfg.StorageBackend != "s3" || cfg.S3Bucket != "flag-bucket" || cfg.S3Prefix != "flag-prefix" ||
		cfg.S3Region != "us-east-1" || cfg.S3Endpoint != "http://minio.local:9000" ||
		cfg.S3Profile != "flag-profile" || !cfg.S3PathStyle {
		t.Fatalf("S3 config flags not applied: %#v", cfg)
	}
	wantOrigins := []string{"http://one.local", "http://two.local"}
	if !reflect.DeepEqual(cfg.AllowedOrigins, wantOrigins) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", cfg.AllowedOrigins, wantOrigins)
	}
}

func TestLoadS3RequiresBucket(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://env@remotehost:5432/envdb")
	t.Setenv("DONTPAD_STORAGE_BACKEND", "s3")

	_, err := Load(Options{})
	if err == nil || !strings.Contains(err.Error(), "DONTPAD_S3_BUCKET") {
		t.Fatalf("Load() err = %v, want missing S3 bucket error", err)
	}
}

func TestLoadEnvFileLoadsValuesWithoutOverridingEnvironment(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_URL", "postgres://env@remotehost:5432/envdb")

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte("DATABASE_URL=postgres://file@remotehost:5432/filedb\nDONTPAD_SCHEMA=file_schema\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() unexpected error: %v", err)
	}
	if err := LoadEnvFile(envPath); err != nil {
		t.Fatalf("LoadEnvFile() unexpected error: %v", err)
	}
	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.PostgresDSN != "postgres://env@remotehost:5432/envdb" {
		t.Fatalf("PostgresDSN = %q, want existing environment to win", cfg.PostgresDSN)
	}
	if cfg.Schema != "file_schema" {
		t.Fatalf("Schema = %q, want value loaded from .env", cfg.Schema)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATABASE_URL",
		"YJSBRIDGE_POSTGRES_DSN",
		"DONTPAD_ADDR",
		"DONTPAD_SCHEMA",
		"DONTPAD_NAMESPACE",
		"DONTPAD_DATA_DIR",
		"DONTPAD_STORAGE_BACKEND",
		"DONTPAD_S3_BUCKET",
		"DONTPAD_S3_PREFIX",
		"DONTPAD_S3_REGION",
		"DONTPAD_S3_ENDPOINT",
		"DONTPAD_S3_PROFILE",
		"DONTPAD_S3_PATH_STYLE",
		"DONTPAD_ALLOWED_ORIGINS",
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"AWS_PROFILE",
		"JWT_SECRET",
		"MASTER_PASSWORD",
	} {
		t.Setenv(key, "")
	}
}
