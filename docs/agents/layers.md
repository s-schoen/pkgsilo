# Layers of a feature

A feature in `feature/<feature>/` has three layer packages, `api` → `service` → `repository`, plus `tools` (see [mcp-tools.md](mcp-tools.md)) next to `api`, which calls the service too.
Each layer calls only the layer below and owns its types and errors.
`feature/identity/` is the reference slice: read it alongside this document.

## Translation

The layer above always translates, even when the types are identical today: record → domain model → API response, repository error → domain error → HTTP problem (or tool error).
The service builds a domain model from its record with an `authz.Mapper` when the model's members match the record's by name and type, and by hand with `ReadableFields.Reveal` otherwise (see [authorization.md](authorization.md)).

Translate only errors that have a meaning in the layer above.
Wrap every other error with `%w`, so it reaches the API's log intact and is answered with a 500 without details.

## Files of each layer package

- `types.go`: the layer's data types.
  The repository has `<Name>Record` with `db` tags, the service has the domain model and the field rules, and the API has explicit request and response bodies.
- `errors.go`: the layer's errors, plus `translateError` for the errors of the layer below.
  In the API, it holds the problem values and `writeError` instead.
- `actions.go` (service only): the feature's authorization actions (see [authorization.md](authorization.md)).
- `repository.go` / `service.go`: the interface and its single implementation.
  The API's `handler.go` has no interface.
- `authenticator.go` (API, only for an authentication method): implements `authn.Authenticator`, and `app/app.go` passes it to `authn.Require`.
  A rejected credential wraps the `authn` reason error that fits (`ErrMalformedCredentials`, `ErrUnknownCredentials`, `ErrExpiredCredentials`), which the metrics and logs name, or else `ErrInvalidCredentials`.
  A login, which authenticates outside `authn.Require`, runs in `authn.LoginRecorder.Record` instead, so that it's counted and traced like the other methods.

## Repository

- One interface, implemented for PostgreSQL with sqlx.
  A repository that also runs in a transaction, as in `app.Bootstrap`, takes a `database.Querier`; inside `database.InTx`, queries use the context that `InTx` passes to `fn`.
- `Patch` writes only the columns it sets, so that concurrent patches of other fields survive.
- Tests run on a real database: each test gets its own fresh one, with `migrations/` applied, from `databasetest.New(t)` and calls `t.Parallel()`.
  The database is never mocked.

## Service

- All logic lives here: IDs, timestamps, audit fields, domain rules, authorization.
- The actor is a parameter of every method, passed in by the handler or tool; the service never reads it from the context.
- **Field rules** are domain rules: the service exports them in `types.go` as ozzo rule sets, such as `UsernameRules`, for the API and the tools to validate their input with.
  Rules for free text come from `core/rules`, which depends on no transport.
  A limit that a description names is an exported constant, such as `MaxUsernameLength`.
- **Lists** of objects without per-row rules query the repository for `limit+1` rows and return a `pagination.Page` built with `pagination.FromLookahead`.
  Lists with per-row rules work differently: see [authorization.md](authorization.md#lists-with-per-row-rules).
- Tests mock the repository, check error translation with `servicetest.AssertTranslated`, capture a mock argument whole with `servicetest.Capture` and answer field reads with `servicetest.ExpectReadableFields`.

## API

- A chi handler with `Routes()`, mounted under `/api/v1` in `app/app.go`, in the group behind `authn.Require` unless the route is public.
  Handlers pass `authn.Actor(r)` to the service.
- **Docs**: `docs.go` describes every route of `Routes()` in the OpenAPI document (see [openapi.md](openapi.md)).
- **Input** is validated with ozzo-validation and the service's field rules, through `core/request` (`DecodeJSON`, `DecodeMergePatch`, `ParsePagination`).
- **PATCH** takes a JSON Merge Patch with the same members as the `PUT` body, each a `request.Optional` validated with `request.IfSet`.
  It reaches the service as pointers, nil for a member left out.
- **Responses** go through the `core/envelope` writers: `WriteResource`, `WriteCreated`, `WriteCollection`, `WriteProblem`, and `WriteJSON` only for bodies with a format of their own that no envelope wraps, such as health checks.
  A service page becomes a collection with `NewPagedCollection`.
- **Response bodies** hold every member but `id` as an `envelope.Field` tagged `omitzero`, filled with `envelope.NewField(field.Get())`, or `envelope.Visible` for a member that's never hidden.
  `Resource` and `Collection` leave a hidden field out and keep a visible `null` as `null`.
  They fail for a member that isn't an `envelope.Field`, which they would otherwise leave out when zero.
- **Problems**: every domain error a client must tell apart gets its own problem type, defined with `envelope.DefineProblemType` in `errors.go`.
  The generic `about:blank` problems (`envelope.NotFoundProblem`, …) are for problems that mean nothing beyond their status.
- Tests mock the service, mount the handler with `apitest.Mount`, which authenticates every request as `apitest.Actor`, and send requests through an `apitest.Client` that checks every response against the feature's `Docs()` (`.WithDocs`), whose `Assert*` methods check responses against the problem values in `errors.go`.
  `apitest.AssertReadableMembers` checks the response body.

## Logging

The handler logs, through its injected logger.
Shared `core` packages log nothing, except `authn.Require`, which logs rejected and failed authentications, and `core/mcpserver`, which logs the unexpected errors of tools and MCP methods.
Bugs such as an unencodable response or a misconfigured validation rule panic, and the router's recoverer logs them with the request ID.

## Test helpers and mocks

- Shared test helpers live in `test/<name>test`, one package each, so that the internal tests of any layer can import them without an import cycle; only `test/apptest` imports `app`.
  A client reports to the test it was made for: inside a subtest, use `api.For(t)`.
- Mocks are generated by mockery into `<pkg>/mocks` and committed.
  Register each new interface in `.mockery.yml`, then run `task generate`.
