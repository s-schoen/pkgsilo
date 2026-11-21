import { DbClient } from "@tanstack/react-db";
import { QueryClient } from "@tanstack/react-query";

import type { RouterContext } from "@/app/routes/__root.tsx";
import type { QueryClientConfig } from "@tanstack/react-query";

// The clients that main.tsx provides to the app and passes to the router's context, and the test
// helpers in src/test/render.tsx create for each test with their own config. The QueryClient
// caches the responses of the API's hooks (src/api/gen/client), and the DbClient holds the
// collections of TanStack DB, which load through it: a collection's descriptor takes it with
// client.requireDependency("queryClient").
//
// A query retries an error up to three times, but not an error response of 4xx, which a retry
// would only repeat: a 401 leads to the sign-in page at once, and a 404 shows at once.
export function createClients(config?: QueryClientConfig): RouterContext {
  const queryClient = new QueryClient({
    ...config,
    defaultOptions: {
      ...config?.defaultOptions,
      queries: { retry: retryUnlessClientError, ...config?.defaultOptions?.queries },
    },
  });
  return { queryClient, dbClient: new DbClient({ queryClient }) };
}

// The retry of TanStack Query's default, three times, for every error but a 4xx response.
function retryUnlessClientError(failureCount: number, error: Error): boolean {
  const status = "status" in error && typeof error.status === "number" ? error.status : undefined;
  if (status !== undefined && status >= 400 && status < 500) {
    return false;
  }
  return failureCount < 3;
}
