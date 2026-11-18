//go:build webui

package ui

import (
	"embed"
	"io/fs"
)

// Built reports whether FS serves the UI's build rather than the placeholder page.
const Built = true

// dist is the UI's build. all: embeds the files whose names start with _ or ., which Rollup may emit.
//
//go:embed all:dist
var dist embed.FS

// FS returns the UI's build, with index.html at its root.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// fs.Sub fails only for an invalid path, and "dist" is valid.
		panic(err)
	}

	return sub
}
