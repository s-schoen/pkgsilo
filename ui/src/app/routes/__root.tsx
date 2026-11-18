import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";

import { NotFound } from "@/components/errors/not-found.tsx";
import { AppLayout } from "@/components/layouts/app-layout.tsx";

import type { QueryClient } from "@tanstack/react-query";

// What every route gets in its context, from createAppRouter: the loaders fill the cache of the
// API's hooks with queryClient.ensureQueryData().
export type RouterContext = {
  queryClient: QueryClient;
};

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <AppLayout>
      <Outlet />
    </AppLayout>
  ),
  notFoundComponent: NotFound,
});
