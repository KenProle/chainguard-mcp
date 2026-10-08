package main

import (
	"embed"
	"io/fs"
)

// webDist holds the built React app. The folder always contains .gitkeep, so
// the Go build works before the UI has been built with npm.
//
//go:embed all:web/dist
var webDist embed.FS

func embeddedUI() fs.FS {
	ui, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		panic(err) // the path is a constant, so this can't fail at runtime
	}
	return ui
}
