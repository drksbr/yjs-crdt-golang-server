package app

import (
	"errors"
	"log"
	"net/http"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/config"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/server"
)

type Options = config.Options

// RunServer loads configuration and starts the DontPad HTTP/WebSocket backend.
func RunServer(opts Options) error {
	if err := config.LoadEnvFile(opts.EnvFile); err != nil {
		return err
	}

	cfg, err := config.Load(opts)
	if err != nil {
		return err
	}

	appServer, err := server.New(cfg)
	if err != nil {
		return err
	}
	defer appServer.Shutdown()

	if err := appServer.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Printf("dontpad backend stopped")
	return nil
}
