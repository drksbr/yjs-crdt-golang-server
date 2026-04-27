package httpdemo

import (
	"context"
	"fmt"
	"os"
	"strings"

	pgstore "yjs-go-bridge/pkg/storage/postgres"
	"yjs-go-bridge/pkg/yhttp"
	"yjs-go-bridge/pkg/yprotocol"
)

const (
	PostgresDSNEnv = "YJSBRIDGE_POSTGRES_DSN"
	postgresSchema = "yjs_bridge_ws_example"
)

// NewPostgresHandler cria um handler WebSocket em cima de provider local e
// persistência PostgreSQL compartilhada.
func NewPostgresHandler() (*yhttp.Server, func() error, error) {
	dsn := strings.TrimSpace(os.Getenv(PostgresDSNEnv))
	if dsn == "" {
		return nil, nil, fmt.Errorf("defina %s para executar este exemplo", PostgresDSNEnv)
	}

	store, err := pgstore.New(context.Background(), pgstore.Config{
		ConnectionString: dsn,
		Schema:           postgresSchema,
	})
	if err != nil {
		return nil, nil, err
	}

	handler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: ResolveRequest,
	})
	if err != nil {
		store.Close()
		return nil, nil, err
	}

	return handler, func() error {
		store.Close()
		return nil
	}, nil
}
