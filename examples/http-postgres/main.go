package main

import (
	"errors"
	"log"
	"net/http"

	"yjs-go-bridge/examples/internal/httpdemo"
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

	mux := http.NewServeMux()
	mux.Handle(httpdemo.WSPath, handler)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(httpdemo.RootMessage("http-postgres")))
	})

	log.Printf("http-postgres: ouvindo em http://%s\n", httpdemo.Address)
	log.Printf("http-postgres: websocket em ws://%s%s?doc=notes&client=101&persist=1\n", httpdemo.Address, httpdemo.WSPath)

	if err := http.ListenAndServe(":8080", mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("servidor http: %v", err)
	}
}
