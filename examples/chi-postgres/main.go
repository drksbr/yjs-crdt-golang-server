package main

import (
	"errors"
	"log"
	"net/http"

	chirouter "github.com/go-chi/chi/v5"

	"github.com/drksbr/yjs-crdt-golang-server/examples/internal/httpdemo"
	yhttpchi "github.com/drksbr/yjs-crdt-golang-server/pkg/yhttp/chi"
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

	router := chirouter.NewRouter()
	router.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(httpdemo.RootMessage("chi-postgres")))
	})
	yhttpchi.Mount(router, httpdemo.WSPath, handler)

	log.Printf("chi-postgres: ouvindo em http://%s\n", httpdemo.Address)
	log.Printf("chi-postgres: websocket em ws://%s%s?doc=notes&client=101&persist=1\n", httpdemo.Address, httpdemo.WSPath)

	if err := http.ListenAndServe(":8080", router); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("servidor chi: %v", err)
	}
}
