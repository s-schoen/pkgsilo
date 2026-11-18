import { defineConfig } from "orval";

// Generates the API client into src/api/gen/, which git ignores, from the OpenAPI document that
// app/openapi_test.go pins (`go test ./app -run TestDocs -update` rewrites it).
// `task ui:gen:api` runs it, and the tasks that read src/ depend on that task.
const input = { target: "../app/testdata/openapi.json" };

export default defineConfig({
  // TanStack Query hooks over fetch, a file per tag, and MSW handlers with faker data for the
  // tests. No baseUrl: the paths of the document start with /api/v1, and the UI is served from
  // the same origin as the API, so fetch sends the session cookie and passes the cross-origin
  // protection.
  api: {
    input,
    output: {
      mode: "tags-split",
      target: "src/api/gen/client",
      schemas: "src/api/gen/client/model",
      client: "react-query",
      httpClient: "fetch",
      clean: true,
      // An error response rejects, with its status and problem in the error's status and info, so
      // that the hooks report it as an error instead of as data.
      override: { fetch: { forceSuccessResponse: true } },
      mock: {
        // No delay, so that the tests don't wait for the responses.
        generators: [{ type: "msw", delay: false }, { type: "faker" }],
      },
    },
  },
  // zod schemas of the requests and responses, to validate forms. The client doesn't check the
  // responses with them.
  zod: {
    input,
    output: {
      mode: "tags-split",
      target: "src/api/gen/zod",
      client: "zod",
      fileExtension: ".zod.ts",
      clean: true,
    },
  },
});
