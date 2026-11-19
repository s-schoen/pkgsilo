# API client

How the UI calls the API through the generated client.

Orval generates the API client into `ui/src/api/gen/` from `app/testdata/openapi.json`, the OpenAPI document that `app/openapi_test.go` pins (`ui/orval.config.ts`).
`src/api/gen/` is ignored by git, and `task ui:gen:api` generates it; `ui:dev`, `ui:build`, `ui:test` and `ui:lint` depend on that task.

- Call the API through the generated TanStack Query hooks in `src/api/gen/client/<tag>/<tag>.ts`, such as `useGetGroup` and `useCreateGroup`, not with `fetch`.
  `main.tsx` provides their `QueryClient`.
- A hook resolves with `{ data, status, headers }`, where `data` is the response body, so the resource is in `data.data`.
  An error response rejects: the hook's `error` has the HTTP status in `status` and the problem in `info`.
- The types of the bodies are in `src/api/gen/client/model/`.
  `src/api/gen/zod/` has zod schemas of the requests and responses, for validating forms.
  They check the types and the required members, not the rules that only a `jsonschema` description states.
  The client doesn't check the responses with them.
- After a change of the API, `go test ./app -run TestDocs -update` rewrites the document, and `task ui:lint` then reports the code that the change breaks.
  The prek hook `oxlint` runs on a commit that changes the document too.
