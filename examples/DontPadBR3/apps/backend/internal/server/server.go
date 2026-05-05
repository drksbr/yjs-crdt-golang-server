package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/coder/websocket"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/config"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/documents"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/media"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/security"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	pgstore "github.com/drksbr/yjs-crdt-golang-server/pkg/storage/postgres"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp"
	yhttpgin "github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp/gin"
	"github.com/drksbr/yjs-crdt-golang-server/pkg/yprotocol"
)

type Server struct {
	cfg       config.Config
	schemaSQL string

	store    *pgstore.Store
	provider *yprotocol.Provider
	metaDB   *pgxpool.Pool

	dataRoot  string
	security  *security.Service
	documents *documents.Service
	media     *media.Service
}

func New(cfg config.Config) (*Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	store, err := pgstore.New(ctx, pgstore.Config{
		ConnectionString: cfg.PostgresDSN,
		Schema:           cfg.Schema,
		ApplicationName:  "dontpadbr2-backend",
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres store: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}

	absDataDir, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		pool.Close()
		store.Close()
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	if err := os.MkdirAll(absDataDir, 0o755); err != nil {
		pool.Close()
		store.Close()
		return nil, fmt.Errorf("ensure data dir: %w", err)
	}

	paths := common.StoragePaths{Root: absDataDir}
	sec := security.New(security.Deps{
		DB:             pool,
		SchemaSQL:      common.QuoteIdentifier(cfg.Schema),
		Namespace:      cfg.Namespace,
		Address:        cfg.Address,
		MasterPassword: cfg.MasterPassword,
		Secret:         []byte(cfg.AuthSecret),
	})
	app := &Server{
		cfg:       cfg,
		schemaSQL: common.QuoteIdentifier(cfg.Schema),
		store:     store,
		provider:  yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		metaDB:    pool,
		dataRoot:  absDataDir,
		security:  sec,
	}
	app.documents = documents.New(documents.Deps{
		DB:        pool,
		SchemaSQL: app.schemaSQL,
		Namespace: cfg.Namespace,
		Store:     store,
		Provider:  app.provider,
		Paths:     paths,
		Security:  sec,
		Legacy:    documents.NewLegacyYSweetMigrator(absDataDir),
	})
	app.media = media.New(media.Deps{
		DB:        pool,
		SchemaSQL: app.schemaSQL,
		Namespace: cfg.Namespace,
		Paths:     paths,
		Security:  sec,
	})

	if err := app.ensureMetadataSchema(ctx); err != nil {
		app.Shutdown()
		return nil, err
	}
	return app, nil
}

func (a *Server) Shutdown() {
	if a.metaDB != nil {
		a.metaDB.Close()
	}
	if a.store != nil {
		a.store.Close()
	}
}

func (a *Server) Run() error {
	wsHandler, err := yhttp.NewServer(yhttp.ServerConfig{
		Provider:       a.provider,
		ReadLimitBytes: common.MaxDocumentUpdateBytes,
		ResolveRequest: func(r *http.Request) (yhttp.Request, error) {
			return a.resolveWSRequest(r)
		},
		AcceptOptions: &websocket.AcceptOptions{
			OriginPatterns: a.cfg.AllowedOrigins,
		},
	})
	if err != nil {
		return fmt.Errorf("build websocket server: %w", err)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/", a.handleRoot)
	router.GET("/healthz", a.handleHealth)
	router.GET("/ws", yhttpgin.Handler(wsHandler))

	api := router.Group("/api")
	{
		api.GET("/documents/:documentId/token", a.security.HandleGetToken)
		api.GET("/documents/:documentId/security", a.security.HandleGetSecurity)
		api.POST("/documents/:documentId/security", a.security.HandleSaveSecurity)
		api.POST("/documents/:documentId/verify-pin", a.security.HandleVerifyPIN)
		api.POST("/documents/:documentId/flush", a.documents.HandleFlush)
		api.DELETE("/documents/:documentId", a.documents.HandleDeleteDocument)
		api.GET("/documents/:documentId/subdocuments", a.documents.HandleListSubdocuments)
		api.POST("/documents/:documentId/subdocuments", a.documents.HandleCreateSubdocument)
		api.DELETE("/documents/:documentId/subdocuments/:slug", a.documents.HandleDeleteSubdocument)

		api.GET("/documents/:documentId/versions", a.documents.HandleListVersions)
		api.POST("/documents/:documentId/versions", a.documents.HandleCreateVersion)
		api.GET("/documents/:documentId/versions/:versionId", a.documents.HandleGetVersion)
		api.POST("/documents/:documentId/versions/:versionId", a.documents.HandleRestoreVersion)
		api.DELETE("/documents/:documentId/versions/:versionId", a.documents.HandleDeleteVersion)

		api.GET("/documents/:documentId/files", a.media.HandleListFiles)
		api.POST("/documents/:documentId/files", a.media.HandleUploadFile)
		api.DELETE("/documents/:documentId/files", a.media.HandleDeleteFile)
		api.GET("/documents/:documentId/files/download", a.media.HandleDownloadFile)

		api.GET("/audio-notes", a.media.HandleListAudioNotes)
		api.POST("/audio-notes", a.media.HandleCreateAudioNote)
		api.DELETE("/audio-notes", a.media.HandleDeleteAudioNote)
		api.GET("/audio-notes/:noteId", a.media.HandleGetAudioNote)
	}

	server := &http.Server{
		Addr:              a.cfg.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	display := common.NormalizeDisplayAddress(a.cfg.Address)
	log.Printf("dontpad backend: http://%s", display)
	log.Printf("dontpad backend: ws://%s/ws", display)
	log.Printf("dontpad backend: namespace=%s schema=%s", a.cfg.Namespace, a.cfg.Schema)
	log.Printf("dontpad backend: data dir=%s", a.dataRoot)
	return server.ListenAndServe()
}

func (a *Server) handleRoot(c *gin.Context) {
	display := common.NormalizeDisplayAddress(a.cfg.Address)
	c.String(http.StatusOK, "dontpad backend ok\nhealthz: http://%s/healthz\nws: ws://%s/ws?doc=welcome&client=101&persist=1\n", display, display)
}

func (a *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":             true,
		"namespace":      a.cfg.Namespace,
		"schema":         a.cfg.Schema,
		"dataDir":        a.dataRoot,
		"websocketPath":  "/ws",
		"allowedOrigins": a.cfg.AllowedOrigins,
	})
}
