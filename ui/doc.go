// Package ui holds the web UI, a Vite + React app, and embeds its build into the binary.
//
// The build in dist/ is embedded only with the build tag webui, which `task build:prod` sets after
// building the UI. Without the tag, FS serves a placeholder page instead, so that Go builds and tests
// need no Node toolchain.
package ui
