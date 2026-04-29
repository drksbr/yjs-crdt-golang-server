package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	gingonic "github.com/gin-gonic/gin"

	"github.com/drksbr/yjs-crdt-golang-server/pkg/storage"
	pgstore "github.com/drksbr/yjs-crdt-golang-server/pkg/storage/postgres"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp"
	yhttpgin "github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp/gin"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

const (
	defaultAddress = ":8080"
	defaultSchema  = "yjs_bridge_gin_react_demo"
	defaultRoom    = "writers-room"
)

type appConfig struct {
	Address        string
	PostgresDSN    string
	Schema         string
	AllowedOrigins []string
	DistDir        string
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		panic(fmt.Errorf("carregando configuracao: %w", err))
	}

	store, err := pgstore.New(context.Background(), pgstore.Config{
		ConnectionString: cfg.PostgresDSN,
		Schema:           cfg.Schema,
	})
	if err != nil {
		panic(fmt.Errorf("abrindo store postgres: %w", err))
	}
	defer store.Close()

	handler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: resolveRequest,
		AcceptOptions: &websocket.AcceptOptions{
			OriginPatterns: cfg.AllowedOrigins,
		},
	})
	if err != nil {
		panic(fmt.Errorf("criando handler websocket: %w", err))
	}

	router := gingonic.New()
	router.Use(gingonic.Logger(), gingonic.Recovery())

	router.GET("/healthz", func(c *gingonic.Context) {
		c.JSON(http.StatusOK, gingonic.H{
			"ok":               true,
			"defaultRoom":      defaultRoom,
			"frontendEmbedded": cfg.DistDir != "",
			"origins":          cfg.AllowedOrigins,
		})
	})
	router.GET("/ws", yhttpgin.Handler(handler))

	if cfg.DistDir != "" {
		mountBuiltFrontend(router, cfg.DistDir)
	} else {
		router.GET("/", func(c *gingonic.Context) {
			c.String(http.StatusOK, buildDevInstructions(cfg))
		})
	}

	displayAddress := normalizeDisplayAddress(cfg.Address)
	fmt.Printf("gin-react-tailwind-postgres: ouvindo em http://%s\n", displayAddress)
	fmt.Printf("gin-react-tailwind-postgres: websocket em ws://%s/ws?doc=%s&client=101&persist=1\n", displayAddress, defaultRoom)
	if cfg.DistDir != "" {
		fmt.Printf("gin-react-tailwind-postgres: servindo frontend buildado de %s\n", cfg.DistDir)
	} else {
		fmt.Println("gin-react-tailwind-postgres: frontend buildado nao encontrado, use `npm run dev` em examples/gin-react-tailwind-postgres/frontend")
	}

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(fmt.Errorf("executando servidor gin: %w", err))
	}
}

func loadConfig() (appConfig, error) {
	dsn := strings.TrimSpace(os.Getenv("YJSBRIDGE_POSTGRES_DSN"))
	if dsn == "" {
		return appConfig{}, errors.New("defina YJSBRIDGE_POSTGRES_DSN")
	}

	address := strings.TrimSpace(os.Getenv("YJSBRIDGE_DEMO_ADDR"))
	if address == "" {
		address = defaultAddress
	}

	schema := strings.TrimSpace(os.Getenv("YJSBRIDGE_DEMO_SCHEMA"))
	if schema == "" {
		schema = defaultSchema
	}

	return appConfig{
		Address:        address,
		PostgresDSN:    dsn,
		Schema:         schema,
		AllowedOrigins: loadAllowedOrigins(),
		DistDir:        detectFrontendDist(),
	}, nil
}

func loadAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("YJSBRIDGE_ALLOWED_ORIGINS"))
	if raw == "" {
		return []string{
			"http://127.0.0.1:5173",
			"http://localhost:5173",
		}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func detectFrontendDist() string {
	candidates := []string{
		filepath.Join("frontend", "dist"),
		filepath.Join("examples", "gin-react-tailwind-postgres", "frontend", "dist"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			return candidate
		}
	}
	return ""
}

func mountBuiltFrontend(router *gingonic.Engine, distDir string) {
	assetsDir := filepath.Join(distDir, "assets")
	if _, err := os.Stat(assetsDir); err == nil {
		router.StaticFS("/assets", http.Dir(assetsDir))
	}

	faviconPath := filepath.Join(distDir, "favicon.svg")
	if _, err := os.Stat(faviconPath); err == nil {
		router.StaticFile("/favicon.svg", faviconPath)
	}

	indexPath := filepath.Join(distDir, "index.html")
	router.GET("/", func(c *gingonic.Context) {
		c.File(indexPath)
	})
}

func resolveRequest(r *http.Request) (yhttp.Request, error) {
	query := r.URL.Query()

	documentID := strings.TrimSpace(query.Get("doc"))
	if documentID == "" {
		return yhttp.Request{}, errors.New("doc obrigatorio")
	}

	clientRaw := strings.TrimSpace(query.Get("client"))
	if clientRaw == "" {
		return yhttp.Request{}, errors.New("client obrigatorio")
	}
	clientValue, err := strconv.ParseUint(clientRaw, 10, 32)
	if err != nil {
		return yhttp.Request{}, fmt.Errorf("client invalido: %w", err)
	}

	persistOnClose := true
	persistRaw := strings.TrimSpace(query.Get("persist"))
	if persistRaw != "" {
		persistOnClose = persistRaw == "1" || strings.EqualFold(persistRaw, "true")
	}

	return yhttp.Request{
		DocumentKey: storage.DocumentKey{
			Namespace:  "examples",
			DocumentID: documentID,
		},
		ConnectionID:   strings.TrimSpace(query.Get("conn")),
		ClientID:       uint32(clientValue),
		PersistOnClose: persistOnClose,
	}, nil
}

func buildDevInstructions(cfg appConfig) string {
	displayAddress := normalizeDisplayAddress(cfg.Address)
	return fmt.Sprintf(
		"backend pronto em http://%s\nfrontend de desenvolvimento: cd examples/gin-react-tailwind-postgres/frontend && npm install && npm run dev\nwebsocket: ws://%s/ws?doc=%s&client=101&persist=1\norigins permitidos: %s\n",
		displayAddress,
		displayAddress,
		defaultRoom,
		strings.Join(cfg.AllowedOrigins, ", "),
	)
}

func normalizeDisplayAddress(address string) string {
	if strings.HasPrefix(address, ":") {
		return "127.0.0.1" + address
	}
	return address
}
