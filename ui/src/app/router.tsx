import { createRouter } from "@tanstack/react-router";

import { routeTree } from "@/app/routeTree.gen.ts";

import type { QueryClient } from "@tanstack/react-query";
import type { RouterHistory } from "@tanstack/react-router";

type AppRouterOptions = {
  queryClient: QueryClient;
  // The browser's history by default; the tests pass a memory history.
  history?: RouterHistory;
};

// The router of the UI, over the routes in src/app/routes (routeTree.gen.ts is generated from
// them). main.tsx creates one, and renderRoute in src/test/render.tsx one per test.
export function createAppRouter({ queryClient, history }: AppRouterOptions) {
  return createRouter({
    routeTree,
    context: { queryClient },
    history,
    // Load a route's code and data when the pointer rests on a link to it. A preload always calls
    // the loader, and TanStack Query decides whether the cached data is fresh.
    defaultPreload: "intent",
    defaultPreloadStaleTime: 0,
    scrollRestoration: true,
  });
}

// Types Link, useNavigate and the other APIs with the routes of this router.
declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
