# MCP tools of a feature

The MCP endpoint at `/mcp` serves the tools that the features offer to agents: another transport for the same actor and policies as the REST API.
`feature/identity/tools` is the reference: read it alongside this document.
Only `core/mcpserver` imports the MCP SDK, and a tools package is never named `mcp`.

## Choosing tools

A feature offers a tool because an agent working with pkgsilo needs it, decided as part of the feature's design.
Rare, irreversible operations stay REST-only.
No tool returns a secret, such as a new API key, since it would pass through a model's context.
A tool is named `<type>_<verb>` in snake case, with the type as in the authz actions: `identity_get`, `group_add_member`.

## Files

- `types.go`: the inputs and outputs.
- `errors.go`: the tool errors and `translateError`.
- `tools.go`: the `Tools` type, a `mcpserver.Toolset` made by `New(service)`, which `app/app.go` passes to `mcpserver.NewHandler`.

`Register` adds each tool with `reg.Add(mcpserver.Tool{Name, Description, Kind}, handler)`.
The handler gets the actor as a parameter and passes it to the service method whose authorization checks the tool keeps.

## Kind and hints

`Kind` (`Read`, `Write`, `Delete`) sets the annotations by which clients approve reads and confirm what takes something away.

- A `Write` is destructive unless it sets `Additive`, which only a tool that overwrites nothing and can't take away access sets, such as `identity_create`.
  A policy, or a member of a group, can take away access through a forbid, so creating one is destructive.
- A `Write` that changes nothing more when it's called again sets `Idempotent`.

## Descriptions

An agent picks a tool and fills in its arguments from the descriptions alone.
A tool's description says what the tool does, when to use it, the domain terms it relies on (what an identity or a policy is), what each of its errors means, and how to page; `mcpserver.PagingDescription` and `mcpserver.WriteErrorsDescription` describe what `mcpserver` does.

Every input and output member has a `jsonschema:"…"` tag with its description.
A limit that a description names comes from the service's constant, such as `strconv.Itoa(service.MaxSourceBytes)`; a tag can't refer to one, so a test pins it with `mcptest.AssertDescriptionContains`.

## Input

- Members are named like the REST request members and the `authz` tags (`displayName`), so that a field action, a REST member and a tool member are the same word.
- Members are required unless tagged `omitempty`, and never null.
  `Add` answers a null, a missing or unknown member and a value that doesn't decode, such as a malformed UUID, with `invalid_arguments` before `Validate` runs.
- `Validate()` applies the service's field rules.
  A tool without arguments takes an empty struct whose `Validate` returns nil.
- An `<type>_update` tool calls the service's `Patch`, with every settable member a `*T` tagged `omitempty` and validated with `validation.When(member != nil, rules...)`, so that a member left out stays unchanged.
  Replacing is REST-only: there's no `<type>_replace` tool.

## Output and lists

- Every member but `id` is an `envelope.Field` tagged `json:",omitzero"`, filled with `envelope.NewField(field.Get())`, so that a hidden field is absent and a visible `null` stays `null`.
- The output of an object has `allowedActions` like the REST response, with the tools package's own `<Type>Action` type (see [openapi.md](openapi.md)), so that the output schema lists the actions as an enum.
  Its description says what it holds; the shared part of the tools' descriptions mentions it.
- A list tool takes `mcpserver.ListInput` and returns the `mcpserver.ListOutput` of `mcpserver.NewListOutput`, whose `nextOffset` an agent copies into its next call.
  Each item is the output of the matching get tool.

## Errors

Every domain error that an agent must tell apart gets a tool error, defined with `mcpserver.DefineError(code, message)` in `errors.go`, with a snake_case code such as `username_taken` and a message that says what happened and what to do.
`translateError` maps the service's errors to them.
`Add` answers denials (`fields_denied`, `forbidden`) and invalid arguments (`invalid_arguments`) itself, and logs any other error and answers it with `internal_error` and the request ID.

## Tests

`tools_test.go` mocks the service and calls the tools through `mcptest.New`, which serves them over HTTP with the production handler, authenticates as `apitest.Actor` and fails a tool, input member or output member without a description.
It covers:

- the annotations, with `Hints`;
- results, with `AssertOutput`;
- every tool error, with `AssertToolError` against the values in `errors.go`, and `AssertInternalError`;
- every validation error;
- pagination with and without `nextOffset`;
- the output, with `mcptest.AssertReadableMembers` and `mcptest.AssertAllowedActionsMember`, and the enum in the output schema of each tool that returns the object, with `OutputSchema`;
- the inputs of create and update tools, with `mcptest.AssertSettableMembers`.

Beyond `tools_test.go`:

- The feature's `e2e_test.go` adds one MCP flow across tool calls (see [e2e-tests.md](e2e-tests.md)).
- `TestMCP` in `app/e2e_test.go` lists the names of all tools.
- The compatibility suite with the TypeScript SDK (`app/mcp_compat_test.go`) covers core behaviour only; features leave it as it is.
