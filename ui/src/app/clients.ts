import { DbClient } from "@tanstack/react-db";
import { QueryClient } from "@tanstack/react-query";

import type { RouterContext } from "@/app/routes/__root.tsx";
import type { QueryClientConfig } from "@tanstack/react-query";

// The clients that main.tsx provides to the app and passes to the router's context, and the test
// helpers in src/test/render.tsx create for each test with their own config. The QueryClient
// caches the responses of the API's hooks (src/api/gen/client), and the DbClient holds the
// collections of TanStack DB, which load through it: a collection's descriptor takes it with
// client.requireDependency("queryClient").
export function createClients(config?: QueryClientConfig): RouterContext {
  const queryClient = new QueryClient(config);
  return { queryClient, dbClient: new DbClient({ queryClient }) };
}
