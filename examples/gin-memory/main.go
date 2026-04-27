package main

import (
	"log"

	gingonic "github.com/gin-gonic/gin"

	"yjs-go-bridge/examples/internal/httpdemo"
	yhttpgin "yjs-go-bridge/pkg/yhttp/gin"
)

func main() {
	handler, err := httpdemo.NewMemoryHandler()
	if err != nil {
		log.Fatalf("criando handler websocket: %v", err)
	}

	router := gingonic.Default()
	router.GET("/", func(c *gingonic.Context) {
		c.String(200, httpdemo.RootMessage("gin-memory"))
	})
	router.GET(httpdemo.WSPath, yhttpgin.Handler(handler))

	log.Printf("gin-memory: ouvindo em http://%s\n", httpdemo.Address)
	log.Printf("gin-memory: websocket em ws://%s%s?doc=notes&client=101&persist=1\n", httpdemo.Address, httpdemo.WSPath)

	if err := router.Run(":8080"); err != nil {
		log.Fatalf("servidor gin: %v", err)
	}
}
