import { createClients } from "@/app/clients.ts";

// The clients of one test (src/test/render.tsx) or story (.storybook/preview.tsx), as main.tsx creates
// them for the app, so that no cached response or collection leaks into another. Queries and
// mutations don't retry, so that they fail at the first error response.
export function createTestClients() {
  return createClients({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}
