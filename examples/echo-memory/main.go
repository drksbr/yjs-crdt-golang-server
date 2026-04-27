package main

import (
	"log"

	labstackecho "github.com/labstack/echo/v4"

	"yjs-go-bridge/examples/internal/httpdemo"
	yhttpecho "yjs-go-bridge/pkg/yhttp/echo"
)

func main() {
	handler, err := httpdemo.NewMemoryHandler()
	if err != nil {
		log.Fatalf("criando handler websocket: %v", err)
	}

	e := labstackecho.New()
	e.GET("/", func(c labstackecho.Context) error {
		return c.String(200, httpdemo.RootMessage("echo-memory"))
	})
	e.GET(httpdemo.WSPath, yhttpecho.Handler(handler))

	log.Printf("echo-memory: ouvindo em http://%s\n", httpdemo.Address)
	log.Printf("echo-memory: websocket em ws://%s%s?doc=notes&client=101&persist=1\n", httpdemo.Address, httpdemo.WSPath)

	if err := e.Start(":8080"); err != nil {
		log.Fatalf("servidor echo: %v", err)
	}
}
