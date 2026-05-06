package server

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/config"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/documents"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/media"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/objectstore"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/security"
	frontend2 "github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/frontend2"
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

	objectStore objectstore.Store
	dataRoot    string
	frontend    fs.FS
	security    *security.Service
	documents   *documents.Service
	media       *media.Service
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

	objects, dataRoot, err := buildObjectStore(ctx, cfg)
	if err != nil {
		pool.Close()
		store.Close()
		return nil, err
	}
	frontendFS, err := frontend2.Dist()
	if err != nil {
		pool.Close()
		store.Close()
		return nil, fmt.Errorf("load embedded frontend: %w", err)
	}

	paths := common.StoragePaths{Root: dataRoot}
	sec := security.New(security.Deps{
		DB:             pool,
		SchemaSQL:      common.QuoteIdentifier(cfg.Schema),
		Namespace:      cfg.Namespace,
		Address:        cfg.Address,
		MasterPassword: cfg.MasterPassword,
		Secret:         []byte(cfg.AuthSecret),
	})
	app := &Server{
		cfg:         cfg,
		schemaSQL:   common.QuoteIdentifier(cfg.Schema),
		store:       store,
		provider:    yprotocol.NewProvider(yprotocol.ProviderConfig{Store: store}),
		metaDB:      pool,
		objectStore: objects,
		dataRoot:    dataRoot,
		frontend:    frontendFS,
		security:    sec,
	}
	app.documents = documents.New(documents.Deps{
		DB:        pool,
		SchemaSQL: app.schemaSQL,
		Namespace: cfg.Namespace,
		Store:     store,
		Provider:  app.provider,
		Paths:     paths,
		Security:  sec,
		Objects:   objects,
		Legacy:    documents.NewLegacyYSweetMigrator(objects, paths),
	})
	app.media = media.New(media.Deps{
		DB:        pool,
		SchemaSQL: app.schemaSQL,
		Namespace: cfg.Namespace,
		Paths:     paths,
		Security:  sec,
		Objects:   objects,
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
	router.GET("/assets/*filepath", a.handleFrontendAsset)
	router.GET("/favicon.svg", a.handleFrontendAsset)
	router.GET("/tip.svg", a.handleFrontendAsset)
	router.GET("/next.svg", a.handleFrontendAsset)
	router.GET("/y-sweet.svg", a.handleFrontendAsset)

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
	router.NoRoute(a.handleFrontendApp)

	server := &http.Server{
		Addr:              a.cfg.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	display := common.NormalizeDisplayAddress(a.cfg.Address)
	log.Printf("dontpad backend: http://%s", display)
	log.Printf("dontpad backend: ws://%s/ws", display)
	log.Printf("dontpad backend: namespace=%s schema=%s", a.cfg.Namespace, a.cfg.Schema)
	log.Printf("dontpad backend: object store=%s", a.objectStore)
	return server.ListenAndServe()
}

func buildObjectStore(ctx context.Context, cfg config.Config) (objectstore.Store, string, error) {
	switch cfg.StorageBackend {
	case "local":
		absDataDir, err := filepath.Abs(cfg.DataDir)
		if err != nil {
			return nil, "", fmt.Errorf("resolve data dir: %w", err)
		}
		store, err := objectstore.NewLocal(absDataDir)
		if err != nil {
			return nil, "", err
		}
		return store, absDataDir, nil
	case "s3":
		store, err := objectstore.NewS3(ctx, objectstore.S3Config{
			Bucket:    cfg.S3Bucket,
			Prefix:    cfg.S3Prefix,
			Region:    cfg.S3Region,
			Endpoint:  cfg.S3Endpoint,
			Profile:   cfg.S3Profile,
			PathStyle: cfg.S3PathStyle,
		})
		if err != nil {
			return nil, "", err
		}
		return store, store.String(), nil
	default:
		return nil, "", fmt.Errorf("storage backend invalido: %q", cfg.StorageBackend)
	}
}

func (a *Server) handleRoot(c *gin.Context) {
	if a.hasEmbeddedFrontend() {
		a.serveFrontendIndex(c)
		return
	}
	display := common.NormalizeDisplayAddress(a.cfg.Address)
	c.String(http.StatusOK, "dontpad backend ok\nhealthz: http://%s/healthz\nws: ws://%s/ws?doc=welcome&client=101&persist=1\nfrontend: nao gerado; rode npm run build em apps/frontend2 antes do go build\n", display, display)
}

func (a *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":             true,
		"namespace":      a.cfg.Namespace,
		"schema":         a.cfg.Schema,
		"storageBackend": a.cfg.StorageBackend,
		"objectStore":    a.objectStore.String(),
		"websocketPath":  "/ws",
		"allowedOrigins": a.cfg.AllowedOrigins,
		"frontend":       a.hasEmbeddedFrontend(),
	})
}

func (a *Server) handleFrontendAsset(c *gin.Context) {
	name := strings.TrimPrefix(c.Request.URL.Path, "/")
	name = path.Clean("/" + name)
	name = strings.TrimPrefix(name, "/")
	if name == "." || name == "" {
		a.serveFrontendIndex(c)
		return
	}
	if !a.frontendFileExists(name) {
		c.Status(http.StatusNotFound)
		return
	}
	if strings.HasPrefix(name, "assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "public, max-age=3600")
	}
	c.FileFromFS(name, http.FS(a.frontend))
}

func (a *Server) handleFrontendApp(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "not found"})
		return
	}
	a.serveFrontendIndex(c)
}

func (a *Server) serveFrontendIndex(c *gin.Context) {
	if !a.hasEmbeddedFrontend() {
		c.Status(http.StatusNotFound)
		return
	}
	index, err := fs.ReadFile(a.frontend, "index.html")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", index)
}

func (a *Server) hasEmbeddedFrontend() bool {
	return a.frontendFileExists("index.html")
}

func (a *Server) frontendFileExists(name string) bool {
	if a.frontend == nil {
		return false
	}
	info, err := fs.Stat(a.frontend, name)
	return err == nil && !info.IsDir()
}
