import { createRootRouteWithContext } from "@tanstack/react-router";

import { RootLayout } from "@/app/root-layout.tsx";
import { NotFound } from "@/components/errors/not-found.tsx";

import type { DbClient } from "@tanstack/react-db";
import type { QueryClient } from "@tanstack/react-query";

// What every route gets in its context, from createAppRouter: the loaders fill the cache of the
// API's hooks with queryClient.ensureQueryData(), and load a collection of TanStack DB with
// dbClient.collection(…).preload().
export type RouterContext = {
  queryClient: QueryClient;
  dbClient: DbClient;
};

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  notFoundComponent: NotFound,
});
