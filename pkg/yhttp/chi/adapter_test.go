package chi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chirouter "github.com/go-chi/chi/v5"
)

func TestMountDelegatesToChiRouter(t *testing.T) {
	t.Parallel()

	router := chirouter.NewRouter()
	called := false
	Mount(router, "/ws", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if !called {
		t.Fatal("mounted handler was not called")
	}
	if rec.Code != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSwitchingProtocols)
	}
}
