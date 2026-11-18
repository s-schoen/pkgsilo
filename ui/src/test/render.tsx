import { DbClient, DbProvider } from "@tanstack/react-db";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createMemoryHistory } from "@tanstack/react-router";
import { render, renderHook } from "@testing-library/react";

import { createAppRouter } from "@/app/router.tsx";

import type { ReactElement, ReactNode } from "react";

// The clients of one test, as main.tsx creates them for the app, so that no cached response or
// collection leaks into another test. Queries and mutations don't retry, so that they fail at the
// first error response.
function testClients() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const dbClient = new DbClient({ queryClient });
  return { queryClient, dbClient };
}

// Provides the clients to children, as main.tsx does.
function withClients(
  { queryClient, dbClient }: ReturnType<typeof testClients>,
  children: ReactNode,
) {
  return (
    <QueryClientProvider client={queryClient}>
      <DbProvider client={dbClient}>{children}</DbProvider>
    </QueryClientProvider>
  );
}

function clientsWrapper() {
  const clients = testClients();
  return function ClientsWrapper({ children }: { children: ReactNode }) {
    return withClients(clients, children);
  };
}

// render() for a component that uses the API's hooks or the collections of TanStack DB.
export function renderWithClient(ui: ReactElement) {
  return render(ui, { wrapper: clientsWrapper() });
}

// renderHook() for a hook that uses the API's hooks or the collections of TanStack DB.
export function renderHookWithClient<Result>(hook: () => Result) {
  return renderHook(hook, { wrapper: clientsWrapper() });
}

// Renders the whole app at path, with the routes of src/app/routes, as the browser would. The
// route's component loads in its own chunk, so find it with a findBy query.
export function renderRoute(path: string) {
  const clients = testClients();
  const router = createAppRouter({
    ...clients,
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  const result = render(withClients(clients, <RouterProvider router={router} />));
  return { ...result, router };
}
