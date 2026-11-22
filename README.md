# pkgsilo

Self-hosted APT and DNF repositories for Debian and RPM packages.
API-first, built for automation and AI agents, and simple enough for a small team.

[![CI](https://github.com/s-schoen/pkgsilo/actions/workflows/ci.yml/badge.svg)](https://github.com/s-schoen/pkgsilo/actions/workflows/ci.yml) [![codecov](https://codecov.io/gh/s-schoen/pkgsilo/branch/master/graph/badge.svg)](https://codecov.io/gh/s-schoen/pkgsilo) [![Go 1.27](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[Install](#install) · [Quickstart](#quickstart) · [Usage](#usage) · [Configuration](#configuration) · [Development](#development)

> [!WARNING]
> **Early development.**
> pkgsilo does not host packages yet.
> What exists today is the foundation: identities, groups, policies, API keys, passwords and sessions, exposed through a REST API, an MCP endpoint and a web UI.
> Debian and RPM repository hosting is still being built.

## What it is

Small and medium organizations often build their own `.deb` and `.rpm` packages and need a place to publish them.
pkgsilo is being built as that place: a single service that serves APT and DNF repositories, managed through an API, a web UI and MCP tools.

Instead of a general-purpose artifact platform, pkgsilo keeps to operating system packages and an API-first design, so that a script or an AI agent configures it as naturally as a person does.
A caching proxy for upstream repositories, as a modern alternative to apt-cacher-ng, is planned for later.

## Features

What works today:

- **Policy-based authorization** - every request is decided by a policy; objects the caller may not read answer like missing ones.
- **Identities, groups and policies** - users and agents, group membership, and reusable rules for actions and fields.
- **API keys and password sessions** - hashed API keys for automation, and a `__Host-` session cookie for the web UI.
- **REST API with an OpenAPI document** - mounted at `/api/v1`, with RFC 9457 Problem Details and a document at `/api/v1/openapi.json`.
- **MCP tools** - identities, groups, policies, API keys and passwords are also manageable through `/mcp`.
- **Embedded web UI** - a React interface in English and German, served from the same origin as the API.
- **PostgreSQL, metrics and traces** - tested against PostgreSQL 18, with Prometheus metrics and OTLP/HTTP spans.

## Requirements

- Go 1.27 or later.
- PostgreSQL 18.
- [Task](https://taskfile.dev) and the tools pinned in `mise.toml`, installed with `mise install`.
- Docker, for the development database that `task db:up` starts; unnecessary if you have PostgreSQL elsewhere.
- Node 26 and pnpm 12, only to build the web UI with `task build:prod`.

## Install

Build the server from source:

```sh
git clone https://github.com/s-schoen/pkgsilo.git
cd pkgsilo
task build
```

This writes `build/pkgsilo` without the web UI.

<details>
<summary>Build the binary with the web UI</summary>

`task build:prod` embeds the compiled UI, so it needs Node and pnpm:

```sh
task build:prod
```

</details>

## Quickstart

The shortest path uses the PostgreSQL container from `docker-compose.dev.yaml`; the commands assume Linux or WSL.

```sh
cp .env.example .env
sed -i "s/^API_KEY_PEPPER=.*/API_KEY_PEPPER=$(openssl rand -hex 32)/" .env
task db:up
task db:migrate
task run
```

On its first start, pkgsilo creates the `admin` identity, the `administrators` group with a policy that permits everything, and an API key that never expires:

```text
{"level":"WARN","msg":"created identity admin with an api key that never expires: create your own key and delete this one","api_key":"psk_…"}
{"level":"INFO","msg":"server started","server":"api","addr":":8080"}
```

Copy the `psk_…` key; it is logged only once.
Reset the `admin` password and delete the bootstrap key as described in the [operator guide](docs/operators.md).

Check that the server is up:

```sh
curl http://localhost:8080/api/v1/health
```

```json
{"status":"healthy","service":"pkgsilo","components":[{"name":"database","status":"healthy"}]}
```

`task run` builds without the web UI, so `/` shows a placeholder page.
To use the UI, run `task build:prod` and start `build/pkgsilo` instead.

Stop the database with `task db:down`.

## Usage

The API lives under `/api/v1`.
API keys go in the `X-API-Key` header; a browser uses the session cookie from `POST /api/v1/sessions`.
Errors are RFC 9457 Problem Details.
The full API is described by the OpenAPI document at `/api/v1/openapi.json`.

Create an identity with the bootstrap key:

```sh
export PKGSILO_KEY="psk_…"

curl -s -X POST http://localhost:8080/api/v1/identities \
  -H "X-API-Key: $PKGSILO_KEY" \
  -H "Content-Type: application/json" \
  -d '{"username":"deploy-bot","displayName":"Deploy Bot"}'
```

```json
{"data":{"id":"…","username":"deploy-bot","displayName":"Deploy Bot","createdBy":"…","updatedBy":"…","createdAt":"2026-11-22T17:38:04Z","updatedAt":"2026-11-22T17:38:04Z"}}
```

Download the OpenAPI document for a client generator:

```sh
curl -s http://localhost:8080/api/v1/openapi.json > openapi.json
```

## Configuration

pkgsilo reads its configuration from the environment; `task run` loads `.env`.
These are the settings most deployments touch:

| Variable                       | Default | Purpose                                                          |
| ------------------------------ | ------- | ---------------------------------------------------------------- |
| `ENV`                          | `prod`  | `dev` enables human-friendly, colored logs.                      |
| `HTTP_ADDR`                    | `:8080` | Address of the API and web UI.                                   |
| `DATABASE_URL`                 | -       | PostgreSQL connection string, required.                          |
| `API_KEY_PEPPER`               | -       | Secret for API key hashes, at least 32 characters, required.     |
| `LOG_LEVEL`                    | `info`  | Minimum log level.                                               |
| `METRICS_ADDR`                 | `:9090` | Address of `/metrics`; an empty value turns metrics off.         |
| `OTEL_EXPORTER_OTLP_ENDPOINT`  | -       | OTLP/HTTP endpoint for traces; unset turns tracing off.          |
| `SESSION_IDLE_TIMEOUT`         | `1h`    | How long a session survives without a request.                   |
| `SESSION_MAX_AGE`              | `24h`   | How long a session survives after its login.                     |

`.env.example` documents every variable, including `DATABASE_MAX_CONNS` and `TRUSTED_PROXIES`.
The [operator guide](docs/operators.md) covers first start, passwords and sessions.

## Development

- `task test` runs every test on real PostgreSQL databases; Docker is required.
- `task test:short` runs the tests that need no database.
- `task lint` runs golangci-lint, and `task generate` regenerates the mocks.
- `task ui:dev` serves the UI with a proxy to `task run`; `task ui:dev:mock` needs no backend.
- `task ui:test` and `task ui:lint` test and lint the web UI.
- `task build:prod` builds the release binary with the embedded UI.

Read [`docs/agents/`](docs/agents/) before changing layer code, authorization, MCP tools or the OpenAPI document.
[`ui/AGENTS.md`](ui/AGENTS.md) covers the web UI.

## Documentation

- [Operator guide](docs/operators.md) - first start, the bootstrap key, passwords and sessions.
- `GET /api/v1/openapi.json` - every endpoint of the API.

## License

MIT - see [LICENSE](LICENSE).
