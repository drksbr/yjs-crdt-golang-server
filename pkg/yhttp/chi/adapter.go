package chi

import (
	"net/http"

	chirouter "github.com/go-chi/chi/v5"
)

const (
	nilRouterPanic  = "yhttp/chi: router nao pode ser nil"
	nilHandlerPanic = "yhttp/chi: handler nao pode ser nil"
)

// Mount registra o `http.Handler` em um path base do router Chi.
func Mount(router chirouter.Router, pattern string, handler http.Handler) {
	if router == nil {
		panic(nilRouterPanic)
	}
	if handler == nil {
		panic(nilHandlerPanic)
	}

	router.Mount(pattern, handler)
}
