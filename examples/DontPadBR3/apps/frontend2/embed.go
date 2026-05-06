package frontend2

import (
	"embed"
	"io/fs"
)

// Files contains the Vite production build generated in dist.
//
// Release flow:
//
//	cd examples/DontPadBR3/apps/frontend2 && npm run build
//	cd ../backend && go build .
//
// The committed dist/.gitkeep keeps the package buildable before the frontend
// has been generated; a production binary should be built after npm run build.
//
//go:embed dist
var Files embed.FS

// Dist returns the embedded frontend build root.
func Dist() (fs.FS, error) {
	return fs.Sub(Files, "dist")
}
