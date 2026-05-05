package main

import (
	"fmt"
	"os"

	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/cmd"
)

func main() {
	if err := cmd.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
