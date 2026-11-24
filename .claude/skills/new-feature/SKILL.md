---
name: new-feature
description: Build a new feature slice in feature/<name>/ (repository, service, API, MCP tools, wiring, e2e test, web UI). Use when adding a new resource type or feature to pkgsilo.
---

# New feature

Build the slice layer by layer, bottom up, with `feature/identity/` open as the reference: copy its shape, not its domain.
Each step ends on a criterion; reach it before starting the next.

## 1. Design

Decide and write down, in the conversation or the ticket:

- the domain model's fields, which of them are settable on create and on update, and which are optional;
- the authz type name, which also prefixes the actions and tool names, and any further actions beyond read, create, update and delete (`docs/agents/authorization.md`);
- whether its list has per-row rules (every top-level list so far does; a child list such as a group's members uses `limit+1`), whether the resource can be a group member, and whether it's owned by an identity, and if so by at most one resource per identity (`OnePerOwner`);
- the REST routes, and which of them get MCP tools; rare, irreversible operations and anything that returns a secret stay REST-only (`docs/agents/mcp-tools.md`);
- whether the slice ships a web UI now (step 8).

Done when every field has a settable/optional decision and every operation has a route and a tool-or-not decision.

## 2. Migration and repository

Read `docs/agents/layers.md`.
Create the migration with `task db:migrate:new -- create_<things>` and write the `.up.sql` and `.down.sql`.
Write `repository/{types,errors,repository}.go` and the repository tests.

Done when `go test ./feature/<name>/repository/...` passes, with a test for every constraint the migration declares.

## 3. Service and actions

Read the service section of `docs/agents/layers.md` and all of `docs/agents/authorization.md`.
Write `service/{types,errors,actions,service}.go`.
The model has the member `AllowedActions authz.AllowedActions`, filled by the `to<Model>` mapper on `actions.Access(decider, resource)` (`AccessIf` for a create).
Register the repository interface in `.mockery.yml` and run `task generate`.

Done when `go test ./feature/<name>/service/...` passes, with `servicetest.AssertTranslated` covering every repository error, a denial test for every action the service checks, and `servicetest.ExpectAllowedActions` in every test that returns the object.

## 4. API

Read `docs/agents/openapi.md`.
Write `api/{types,errors,handler,docs}.go`, with the `<Type>Action` enum and the `allowedActions` member of every resource response, and `handler_test.go`, whose client checks every response against `Docs()`.
Register the service interface in `.mockery.yml` and run `task generate`.

Done when `go test ./feature/<name>/api/...` passes, with `apitest.AssertReadableMembers` and `apitest.AssertAllowedActionsMember` on every response type and a field action for every settable request member, and `Docs()` has an operation for every route of `Routes()` whose `Problems` hold every problem type its handler answers with: the types of `errors.go`, the not-found type of each `request.UUIDParam` or `request.IdentityParam`, and `envelope.ForbiddenProblem` and `request.FieldsDeniedType` where `writeError` answers with them.

## 5. MCP tools

Read `docs/agents/mcp-tools.md`.
Write `tools/{types,errors,tools}.go`, with the tools package's own `<Type>Action` enum and `allowedActions` in the output of the object, and `tools_test.go`.

Done when `go test ./feature/<name>/tools/...` passes and covers every item of its Tests section.

## 6. Wiring

Tick every item:

- [ ] `app/app.go`: the service built in `NewHandler`, its handler mounted under `/api/v1` (behind `authenticate` unless public), its toolset passed to `mcpserver.NewHandler`.
- [ ] `app/app.go`: `<name>service.Actions()` added in `typeActions()`, which feeds both the Cedar schema and the permissions endpoint.
- [ ] `app/app.go`: `<name>api.Docs()` mounted in `Docs()` at the same path as its routes.
- [ ] `go test ./app -run TestDocs -update`, and the diff of `app/testdata/openapi.json` shows exactly the new operations and schemas, each resource response with its `allowedActions` enum.
- [ ] `go test ./app -run TestSchema -update`, and the diff of `app/testdata/schema.cedarschema` shows exactly the new type and its actions.
- [ ] `ui/src/mocks/actions.ts`: the new actions in `allActions`, as that diff shows them, so that the mock API's permissions list holds them.
- [ ] `app/e2e_test.go`: the new tool names in `TestMCP`, in sorted order.
- [ ] `.github/codecov.yml`: a component for `feature/<name>/**`.
- [ ] `.mockery.yml`: every new interface, with `task generate` run.

## 7. End-to-end test

Read `docs/agents/e2e-tests.md`.
Write `feature/<name>/e2e_test.go` with at least one REST flow, one MCP flow, and a flow as an identity from `apptest.NewIdentity`, which the default policies don't make an administrator.

Done when `go test ./feature/<name>/` passes.

## 8. Web UI

Only for a slice that ships a UI now; otherwise skip to step 9.
Read `ui/AGENTS.md` and `ui/docs/agents/permissions.md`, and build the pages after `ui/src/features/identities/`.
Run `task ui:gen:api` for the new types, then:

- add the type to `ResourceType` in `ui/src/features/permissions/types/action.ts`, and its `allowedActions` item type and its create request's `CreateActions` to `Action`;
- write `use<Resource>Permissions` in the feature's `hooks/`, the only place with the type's action strings, with its create actions exported for the guard;
- give the navigation entry `action="<type>:read"` and guard the "new" page with `requirePermission`;
- pass the hook's booleans to the shared components' props (`isReadOnly`, `isVisible`, `canDelete`).

Done when every denied action of the type is hidden for the `reader` persona, the hook's unit tests cover each boolean and each business rule, and `task ui:lint` and `task ui:test` pass.

## 9. Verify

Run `task lint` and `task test`.
Done when both pass with no new warnings.
