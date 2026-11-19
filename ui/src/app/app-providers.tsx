import { DbProvider } from "@tanstack/react-db";
import { QueryClientProvider } from "@tanstack/react-query";

import type { RouterContext } from "@/app/routes/__root.tsx";
import type { ReactNode } from "react";

// Provides the clients of createClients() to children: the QueryClient to the API's hooks, and the
// DbClient to the collections of TanStack DB. main.tsx wraps the app in it, the test helpers in
// src/test/render.tsx each test, and .storybook/preview.tsx each story.
export function AppProviders({
  clients: { queryClient, dbClient },
  children,
}: {
  clients: RouterContext;
  children: ReactNode;
}) {
  return (
    <QueryClientProvider client={queryClient}>
      <DbProvider client={dbClient}>{children}</DbProvider>
    </QueryClientProvider>
  );
}
