# OpenAPI document

`app.Docs()` builds the OpenAPI 3.1 document of the HTTP API, served publicly at `/api/v1/openapi.json`.
Each feature describes its own routes in `api/docs.go`; `core/openapi` adds everything the shared packages
do. `feature/identity/api/docs.go` is the reference.

## A feature's `Docs()`

`func Docs() openapi.Section` sits next to `Routes()` and has one `openapi.Operation` per route:

- `Pattern` is the chi pattern of `Routes()`, relative to the mount path. `Parameters` declares every path
  parameter of the full path with `openapi.PathID`, the mount path's ones included (see
  `feature/apikey/api/docs.go`).
- A public route in a section whose other routes need authentication, such as a login, sets `Public`; a
  section whose routes are all public is mounted with `spec.MountPublic` instead.
- `Body` and `Responses` name the api package's own types: `JSONBody`/`MergePatchBody` for what
  `request.DecodeJSON`/`DecodeMergePatch` read, and `Resource`, `Created`, `Collection`, `NoContent` or
  `JSON` for what the `envelope` writer writes.
- `Problems` lists every problem type that the handler answers with: the types of `errors.go`,
  `envelope.ForbiddenProblem`, `request.FieldsDeniedType`, and the not-found type of each `UUIDParam`.
  Build adds the rest itself: the body's 400/413/415/422, the pagination's 400 for a `Collection`, the 401
  of `authn.Require`, the 403 of `middleware.CrossOriginProtection` for a POST, PUT, PATCH or DELETE, and the
  500.

Schemas come from `core/typeschema`, as for the MCP tools: a `jsonschema` tag is a member's description, and
a member is required unless it's tagged `omitzero` or `omitempty`. Every `envelope.Field` and
`request.Optional` member is tagged `omitzero`; Build fails otherwise. A request member that may be left
out is tagged `omitzero` too. Describe the rules of a request member in its `jsonschema` tag, in the words of
the matching MCP tool.

## How it stays true

- `handler_test.go` builds its client with `.WithDocs("<mount path>", Docs())`: every response must match
  the operation of its request, its status and problem type included, or the test fails. The e2e tests of
  `test/apptest` check against the full document in the same way.
- `app/openapi_test.go` fails for a route that `Docs()` doesn't describe, or the reverse, and pins the
  document in `app/testdata/openapi.json`. After a change of the API, run `go test ./app -run TestDocs
  -update` and check that the diff of the golden file shows exactly that change.

## Wiring

`app.Docs()` mounts each section at the path that `NewHandler` mounts its routes at, with the same path
constants: `spec.Mount` for routes behind `authenticate`, `spec.MountPublic` for the others.
