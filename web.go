// Package web provides the embedded frontend filesystem.
// Import as "github.com/Mi-Bee-Studio/mibeehive".
package web

import (
	"embed"
	"io/fs"
)

//go:embed web
var rawFS embed.FS

// FS returns the embedded web frontend filesystem.
func FS() (fs.FS, error) {
	return fs.Sub(rawFS, "web")
}
