//go:build !webui

package ui

import (
	"embed"
	"io/fs"
)

// Built reports whether FS serves the UI's build rather than the placeholder page.
const Built = false

// stub is the placeholder page of a binary built without the UI.
//
//go:embed stub
var stub embed.FS

// FS returns the placeholder page, as index.html at its root.
func FS() fs.FS {
	sub, err := fs.Sub(stub, "stub")
	if err != nil {
		// fs.Sub fails only for an invalid path, and "stub" is valid.
		panic(err)
	}

	return sub
}
