package main

import (
	"log"

	gingonic "github.com/gin-gonic/gin"

	"github.com/drksbr/yjs-crdt-golang-server/examples/internal/httpdemo"
	yhttpgin "github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp/gin"
)

func main() {
	handler, closeStore, err := httpdemo.NewPostgresHandler()
	if err != nil {
		log.Fatalf("criando handler websocket postgres: %v", err)
	}
	defer func() {
		if err := closeStore(); err != nil {
			log.Printf("fechando store postgres: %v", err)
		}
	}()

	router := gingonic.Default()
	router.GET("/", func(c *gingonic.Context) {
		c.String(200, httpdemo.RootMessage("gin-postgres"))
	})
	router.GET(httpdemo.WSPath, yhttpgin.Handler(handler))

	log.Printf("gin-postgres: ouvindo em http://%s\n", httpdemo.Address)
	log.Printf("gin-postgres: websocket em ws://%s%s?doc=notes&client=101&persist=1\n", httpdemo.Address, httpdemo.WSPath)

	if err := router.Run(":8080"); err != nil {
		log.Fatalf("servidor gin: %v", err)
	}
}
