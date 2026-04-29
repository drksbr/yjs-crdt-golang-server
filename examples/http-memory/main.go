package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/drksbr/yjs-crdt-golang-server/examples/internal/httpdemo"
)

func main() {
	handler, err := httpdemo.NewMemoryHandler()
	if err != nil {
		log.Fatalf("criando handler websocket: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(httpdemo.RootMessage("http-memory")))
	})

	log.Printf("http-memory: ouvindo em http://%s\n", httpdemo.Address)
	log.Printf("http-memory: websocket em ws://%s%s?doc=notes&client=101&persist=1\n", httpdemo.Address, httpdemo.WSPath)

	if err := http.ListenAndServe(":8080", mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("servidor http: %v", err)
	}
}
