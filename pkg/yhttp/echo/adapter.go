package echo

import (
	"net/http"

	labstackecho "github.com/labstack/echo/v4"
)

const nilHandlerPanic = "yhttp/echo: handler nao pode ser nil"

// Handler adapta um `http.Handler` para `echo.HandlerFunc`.
func Handler(handler http.Handler) labstackecho.HandlerFunc {
	if handler == nil {
		panic(nilHandlerPanic)
	}

	return func(c labstackecho.Context) error {
		handler.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
