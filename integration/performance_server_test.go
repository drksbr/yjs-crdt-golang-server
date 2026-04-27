package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gingonic "github.com/gin-gonic/gin"
	chirouter "github.com/go-chi/chi/v5"
	labstackecho "github.com/labstack/echo/v4"

	"yjs-go-bridge/pkg/storage"
	"yjs-go-bridge/pkg/storage/memory"
	pgstore "yjs-go-bridge/pkg/storage/postgres"
	yhttpchi "yjs-go-bridge/pkg/yhttp/chi"
	yhttpecho "yjs-go-bridge/pkg/yhttp/echo"
	yhttpgin "yjs-go-bridge/pkg/yhttp/gin"
)

type perfBackend struct {
	name     string
	newStore func(t *testing.T, caseName string) (storage.SnapshotStore, func())
}

func newPerfBackends(t *testing.T) []perfBackend {
	t.Helper()
	t.Setenv(runDockerSmokeEnv, "1")
	pg := startDockerPostgres(t)

	return []perfBackend{
		{
			name: "memory",
			newStore: func(t *testing.T, _ string) (storage.SnapshotStore, func()) {
				t.Helper()
				return memory.New(), func() {}
			},
		},
		{
			name: "postgres",
			newStore: func(t *testing.T, caseName string) (storage.SnapshotStore, func()) {
				t.Helper()

				store, err := pgstore.New(context.Background(), pgstore.Config{
					ConnectionString: pg.dsn,
					Schema:           newSmokeSchema(caseName),
				})
				if err != nil {
					t.Fatalf("pgstore.New() unexpected error: %v", err)
				}
				return store, func() {
					store.Close()
				}
			},
		},
	}
}

func newPerfServer(t *testing.T, framework string, store storage.SnapshotStore) (*httptest.Server, func()) {
	t.Helper()

	handler, err := newTransportHandler(store)
	if err != nil {
		t.Fatalf("newTransportHandler() unexpected error: %v", err)
	}

	switch framework {
	case "net/http":
		mux := http.NewServeMux()
		mux.Handle("/ws", handler)
		server := httptest.NewServer(mux)
		return server, server.Close
	case "gin":
		gingonic.SetMode(gingonic.ReleaseMode)
		router := gingonic.New()
		router.GET("/ws", yhttpgin.Handler(handler))
		server := httptest.NewServer(router)
		return server, server.Close
	case "echo":
		router := labstackecho.New()
		router.HideBanner = true
		router.HidePort = true
		router.GET("/ws", yhttpecho.Handler(handler))
		server := httptest.NewServer(router)
		return server, server.Close
	case "chi":
		router := chirouter.NewRouter()
		yhttpchi.Mount(router, "/ws", handler)
		server := httptest.NewServer(router)
		return server, server.Close
	default:
		t.Fatalf("framework %q nao suportado", framework)
	}

	return nil, nil
}

func perfDocumentKey(framework, backend string) storage.DocumentKey {
	return storage.DocumentKey{
		Namespace:  "integration",
		DocumentID: perfDocumentID(framework, backend),
	}
}

func perfDocumentID(framework, backend string) string {
	replacer := strings.NewReplacer("/", "-", " ", "-", ".", "-")
	return replacer.Replace(framework) + "-" + backend
}
