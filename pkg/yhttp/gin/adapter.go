package gin

import (
	"net/http"

	gingonic "github.com/gin-gonic/gin"
)

const nilHandlerPanic = "yhttp/gin: handler nao pode ser nil"

// Handler adapta um `http.Handler` para `gin.HandlerFunc`.
//
// O wrapper executa o handler HTTP e aborta a cadeia do Gin em seguida para
// evitar middleware/handlers adicionais depois do upgrade WebSocket.
func Handler(handler http.Handler) gingonic.HandlerFunc {
	if handler == nil {
		panic(nilHandlerPanic)
	}

	return func(c *gingonic.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}
