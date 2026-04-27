package echo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	labstackecho "github.com/labstack/echo/v4"
)

func TestHandlerDelegatesToHTTPHandler(t *testing.T) {
	t.Parallel()

	e := labstackecho.New()
	delegated := false
	e.GET("/ws", Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		delegated = true
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	})))

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if !delegated {
		t.Fatal("delegated handler was not called")
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", rec.Body.String(), "ok")
	}
}
