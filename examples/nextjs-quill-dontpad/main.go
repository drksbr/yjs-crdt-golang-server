package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
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
	defaultAddress   = ":8080"
	defaultSchema    = "yjs_bridge_nextjs_quill_dontpad"
	defaultNamespace = "nextjs-quill-dontpad"
	defaultDocSlug   = "welcome"
)

type appConfig struct {
	Address        string
	PostgresDSN    string
	Schema         string
	Namespace      string
	AllowedOrigins []string
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("carregando configuracao: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := pgstore.New(ctx, pgstore.Config{
		ConnectionString: cfg.PostgresDSN,
		Schema:           cfg.Schema,
		ApplicationName:  defaultNamespace,
	})
	if err != nil {
		log.Fatalf("abrindo store postgres: %v", err)
	}
	defer store.Close()

	handler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		ResolveRequest: cfg.resolveRequest,
		AcceptOptions: &websocket.AcceptOptions{
			OriginPatterns: cfg.AllowedOrigins,
		},
	})
	if err != nil {
		log.Fatalf("criando handler websocket: %v", err)
	}

	router := gingonic.New()
	router.Use(gingonic.Logger(), gingonic.Recovery())

	router.GET("/", func(c *gingonic.Context) {
		c.String(http.StatusOK, buildRootMessage(cfg))
	})
	router.GET("/healthz", func(c *gingonic.Context) {
		c.JSON(http.StatusOK, gingonic.H{
			"ok":             true,
			"namespace":      cfg.Namespace,
			"schema":         cfg.Schema,
			"allowedOrigins": cfg.AllowedOrigins,
			"websocketPath":  "/ws",
			"slugQueryParam": "doc",
		})
	})
	router.GET("/ws", yhttpgin.Handler(handler))

	displayAddress := normalizeDisplayAddress(cfg.Address)
	log.Printf("nextjs-quill-dontpad: ouvindo em http://%s", displayAddress)
	log.Printf("nextjs-quill-dontpad: healthz em http://%s/healthz", displayAddress)
	log.Printf("nextjs-quill-dontpad: websocket em ws://%s/ws?doc=%s&client=101&persist=1", displayAddress, defaultDocSlug)
	log.Printf("nextjs-quill-dontpad: frontend Next esperado em http://127.0.0.1:3000")

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("executando servidor gin: %v", err)
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
		Namespace:      defaultNamespace,
		AllowedOrigins: loadAllowedOrigins(),
	}, nil
}

func loadAllowedOrigins() []string {
	defaultOrigins := []string{
		"http://127.0.0.1:3000",
		"http://localhost:3000",
	}

	raw := strings.TrimSpace(os.Getenv("YJSBRIDGE_ALLOWED_ORIGINS"))
	if raw == "" {
		return defaultOrigins
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	if len(origins) == 0 {
		return defaultOrigins
	}
	return origins
}

func (cfg appConfig) resolveRequest(r *http.Request) (yhttp.Request, error) {
	query := r.URL.Query()

	slug := strings.TrimSpace(query.Get("doc"))
	if slug == "" {
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
			Namespace:  cfg.Namespace,
			DocumentID: slug,
		},
		ConnectionID:   strings.TrimSpace(query.Get("conn")),
		ClientID:       uint32(clientValue),
		PersistOnClose: persistOnClose,
	}, nil
}

func buildRootMessage(cfg appConfig) string {
	displayAddress := normalizeDisplayAddress(cfg.Address)
	return fmt.Sprintf(
		"backend pronto em http://%s\nhealthz: http://%s/healthz\nfrontend Next: cd examples/nextjs-quill-dontpad/frontend && npm install && npm run dev\nwebsocket: ws://%s/ws?doc=%s&client=101&persist=1\nslug do documento: query `doc`\norigins permitidos: %s\n",
		displayAddress,
		displayAddress,
		displayAddress,
		defaultDocSlug,
		strings.Join(cfg.AllowedOrigins, ", "),
	)
}

func normalizeDisplayAddress(address string) string {
	if strings.HasPrefix(address, ":") {
		return "127.0.0.1" + address
	}
	return address
}
