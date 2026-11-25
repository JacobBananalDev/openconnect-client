package gui

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var assets embed.FS

// DistAssets returns the built frontend assets embedded in the application.
func DistAssets() (fs.FS, error) {
	return fs.Sub(assets, "web/dist")
}
