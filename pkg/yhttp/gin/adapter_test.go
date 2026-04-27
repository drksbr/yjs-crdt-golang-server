package gin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gingonic "github.com/gin-gonic/gin"
)

func TestHandlerDelegatesAndAborts(t *testing.T) {
	t.Parallel()

	gingonic.SetMode(gingonic.TestMode)

	var delegated, downstream bool
	router := gingonic.New()
	router.GET("/ws",
		Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			delegated = true
			w.WriteHeader(http.StatusNoContent)
		})),
		func(c *gingonic.Context) {
			downstream = true
			c.Status(http.StatusOK)
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if !delegated {
		t.Fatal("delegated handler was not called")
	}
	if downstream {
		t.Fatal("downstream gin handler ran after adapter abort")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
