# AGENTS.md

pkgsilo is a Go service with a REST API under `/api/v1` and an MCP endpoint at `/mcp`, both served by
the same features with the same authorization, and a web UI at `/`.

## Layout

- `feature/<feature>/`: one vertical slice per feature, with the layer packages `api` → `service` →
  `repository`, and `tools` (MCP) next to `api`. `feature/identity/` is the reference slice. Only
  features go here.
- `core/`: shared infrastructure; each package's doc comment describes it.
- `test/<name>test`: shared test helpers, one package each.
- `app/`: assembles the features (`app/app.go`); `migrations/`: the SQL migrations.
- `ui/`: the web UI, a Vite + React + TypeScript app (pnpm) that the binary embeds; `core/spa` serves it.

## Rules for every change

- Each layer calls only the layer below, owns its types and errors, and translates those of the layer
  below, even when they're identical today.
- All logic lives in the service. The actor is a parameter of every service method, passed in by the
  handler or tool.
- Authorization denies everything that no policy permits. An object the actor can't read answers like
  a missing one.
- Repository tests run on a real PostgreSQL database from `databasetest.New(t)`.
- Mocks come from mockery: register each new interface in `.mockery.yml`, then run `task generate`.

## Read before you work on

- **A new feature**: run the `new-feature` skill.
- **Layer code** (types, errors, repositories, services, handlers and their tests):
  `docs/agents/layers.md`.
- **Authorization** (`actions.go`, `authz` tags, read/write checks, lists, references):
  `docs/agents/authorization.md`.
- **MCP tools** (`feature/*/tools`): `docs/agents/mcp-tools.md`.
- **API routes, request or response types** (`feature/*/api`, its `docs.go`): `docs/agents/openapi.md`.
- **End-to-end tests** (`e2e_test.go`): `docs/agents/e2e-tests.md`.
- **Telemetry** (metrics, spans, or logging errors): `docs/agents/telemetry.md`.
- **The web UI** (`ui/`, `core/spa`): `docs/agents/ui.md`.

## Commands

`task generate` (mocks), `task lint`, `task test` (needs Docker for PostgreSQL), `task test:short`
(without), `task db:migrate:new -- <name>`, `task ui:dev`, `task ui:build` and `task ui:test` (the web
UI), `task build:prod` (the binary with the web UI). Run changed workflows locally with `task ci:push`
and `task ci:pr` (act) before pushing them.

## Git

- NEVER create git commits or branches on your own unless you're explicitly told so
- Every commit passes the prek hooks (`prek.toml`): when one fails, fix what it reports or stage the
  files it rewrote, then commit again.
- NEVER add a trailer like Co-Authored-By to commit messages
