# Embedding and serving the web UI

How the Go binary embeds the web UI and serves it at `/`.
The app itself, in `ui/`, is described in `ui/AGENTS.md`.

## Build tag

- `ui/embed.go` (`//go:build webui`) embeds `ui/dist`, the output of `task ui:build`.
- Without the tag, `ui/embed_stub.go` embeds `ui/stub/index.html`, a page that says the UI isn't built, and the server logs a warning at startup.
  So `go build`, `go test` and the linters need no Node.
- `task build:prod` builds the UI first and passes `-tags webui`; the CI job `build` runs it.
- `go.mod` ignores `ui/node_modules`, since npm packages may ship `.go` files.

## Serving (`core/spa`)

`app.NewHandler` mounts `spa.Handler(ui.FS(), …)` at `/*` for GET and HEAD, after `/mcp` and `/api/v1`, so a path under `/api/v1` that no route takes still gets the 404 problem.
The router answers the other methods with its 405 problem.

- A path that names a file gets it.
  Any other path gets `index.html` if the request is a browser's navigation, which accepts `text/html` by name, such as to `/identities/42` or `/packages/lodash.merge`: a client-side route.
  Every other request gets the 404 problem: a stale tab gets no HTML for an asset of an older build, and an API or MCP client none for a path that no route takes, such as `/.well-known/oauth-protected-resource`.
- `assets/*` (names hashed by Vite) are cached forever; every other file gets `no-cache` and an ETag, so that a browser revalidates it with a 304.
- `spa.Handler` reads and compresses the files once, when the server starts, and `app.NewHandler` panics if it can't (a broken build) or if there is no `index.html`.
  A file is sent with gzip to a client that accepts it, if gzip makes it smaller.
- The UI's files get the Content-Security-Policy `default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'`, instead of the API's, which blocks every script.
  So the UI has no inline scripts or styles and loads nothing from other origins.
