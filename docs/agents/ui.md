# The web UI

`ui/` is a Vite + React + TypeScript app, managed with pnpm (Node and pnpm are pinned in `mise.toml`).
The Go binary embeds its build and serves it at `/`.

## Build tag

- `ui/embed.go` (`//go:build webui`) embeds `ui/dist`, the output of `task ui:build`.
- Without the tag, `ui/embed_stub.go` embeds `ui/stub/index.html`, a page that says the UI isn't built,
  and the server logs a warning at startup. So `go build`, `go test` and the linters need no Node.
- `task build:prod` builds the UI first and passes `-tags webui`; the CI job `build` runs it.
- `go.mod` ignores `ui/node_modules`, since npm packages may ship `.go` files.

## Development

`task run` serves the API on `:8080`, and `task ui:dev` runs the Vite dev server, which proxies `/api`
and `/mcp` to it. Open the URL that Vite prints.

## Tests

`task ui:test` runs the tests once, with coverage; `pnpm run test:watch` in `ui/` reruns them on change.
The CI job `ui-test` runs them on every push and uploads the coverage and the JUnit report from
`ui/build/` to Codecov.

- Vitest, configured in the `test` section of `vite.config.ts`, runs the tests in jsdom. Its globals
  are off: import `describe`, `it` and `expect` from `vitest`.
- A test sits next to the code it tests, as `<name>.test.tsx`.
- Render with React Testing Library and query by role or label, as a user finds things; interact with
  `userEvent.setup()`. The jest-dom matchers, such as `toBeInTheDocument`, are loaded for every test.
- MSW mocks the API: register the handlers a test needs with `server.use(http.get("/api/v1/…", …))`,
  `server` from `@/test/server`. They're removed after each test. A request that no handler takes
  rejects with an MSW error and never reaches the network.
- Vitest undoes `vi.spyOn`, `vi.stubEnv` and `vi.stubGlobal` before each test, so a test doesn't
  restore them itself.
- `src/test/setup.ts` runs before every test file. Coverage leaves out `src/test/` and `src/main.tsx`.

## Serving (`core/spa`)

`app.NewHandler` mounts `spa.Handler(ui.FS(), …)` at `/*`, after `/mcp` and `/api/v1`, so a path under
`/api/v1` that no route takes still gets the 404 problem.

- A path that names a file gets it. A path without an extension that names none, such as
  `/identities/42`, is a client-side route and gets `index.html`. A missing path with an extension gets
  the 404 problem, so that a stale tab gets no HTML for an asset of an older build.
- `assets/*` (names hashed by Vite) are cached forever; every other file gets `no-cache`.
- The UI's files get the Content-Security-Policy `default-src 'self'; img-src 'self' data:;
  frame-ancestors 'none'; base-uri 'none'; form-action 'self'`, instead of the API's, which blocks every
  script. So the UI has no inline scripts or styles and loads nothing from other origins.
