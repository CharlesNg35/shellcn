package web

import (
	"embed"
	"io/fs"
)

// StaticFiles embeds the frontend build output (web/dist) into the Go binary.
// This allows the application to serve the frontend without external dependencies.
//
//go:embed all:dist
var staticFS embed.FS

// FS returns the embedded filesystem containing the frontend static files.
// The files are located under the "dist" directory in the embedded FS.
func FS() (fs.FS, error) {
	// Strip the "dist" prefix to serve files from root
	return fs.Sub(staticFS, "dist")
}
