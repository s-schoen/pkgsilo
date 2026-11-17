# End-to-end tests of a feature

Every feature has `feature/<feature>/e2e_test.go` (package `<feature>_test`), which drives the assembled
app through `/api/v1` and `/mcp` on a real database. It covers what only the whole app shows: flows
across requests, database constraints, `Location` headers.

## Clients

- `apptest.New(t)` serves the app; its client authenticates as the `admin` identity of the first start.
- `api.Anonymous()` sends no credential.
- `apptest.NewIdentity` creates an identity outside the `administrators` group, with a client that
  authenticates as it.
- `apptest.AdministratorsGroup` finds the `administrators` group, which the first start creates with a
  random ID.
- `apptest.MCP(t, api)` calls tools; each feature adds one MCP flow across tool calls.
- Inside a subtest, use `api.For(t)`, so that the client reports to the subtest.
- `apptest.NewWithMetrics` scrapes the app's metrics and `apptest.NewWithTracing` collects its spans
  (see [telemetry.md](telemetry.md)).

## Shape

Each test's doc comment lists its steps in order, each with the result it expects, and numbered
comments (`// 1. Create.`) mark where each step starts in the code.

## Expected errors are literals

Write each expected problem type out as a literal `envelope.DefineProblemType(…)` and each expected
tool error as a literal `mcpserver.DefineError(…)`, so that the test pins the public contract (slug,
title, status; code, message) independently of `errors.go`.
