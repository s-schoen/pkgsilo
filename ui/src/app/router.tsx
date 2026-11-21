import { createRouter } from "@tanstack/react-router";

import { routeTree } from "@/app/routeTree.gen.ts";
import { redirectOnUnauthorized } from "@/features/session/utils/unauthorized.ts";

import type { RouterContext } from "@/app/routes/__root.tsx";
import type { RouterHistory } from "@tanstack/react-router";

type AppRouterOptions = RouterContext & {
  // The browser's history by default; the tests pass a memory history.
  history?: RouterHistory;
};

// The router of the UI, over the routes in src/app/routes (routeTree.gen.ts is generated from
// them). main.tsx creates one, and renderRoute in src/test/render.tsx one per test. Once a request
// fails with 401, as when the session ends, it leads to the sign-in page.
export function createAppRouter({ queryClient, dbClient, history }: AppRouterOptions) {
  const router = createRouter({
    routeTree,
    context: { queryClient, dbClient },
    history,
    // Load a route's code and data when the pointer rests on a link to it. A preload always calls
    // the loader, and TanStack Query decides whether the cached data is fresh.
    defaultPreload: "intent",
    defaultPreloadStaleTime: 0,
    scrollRestoration: true,
  });
  redirectOnUnauthorized(queryClient, router);
  return router;
}

// Types Link, useNavigate and the other APIs with the routes of this router.
declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
