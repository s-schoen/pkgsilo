import { RouterProvider, createMemoryHistory } from "@tanstack/react-router";
import { render, renderHook } from "@testing-library/react";

import { AppProviders } from "@/app/app-providers.tsx";
import { createAppRouter } from "@/app/router.tsx";
import { createTestClients } from "@/test/clients.ts";

import type { RouterContext } from "@/app/routes/__root.tsx";
import type { ReactElement, ReactNode } from "react";

function clientsWrapper() {
  const clients = createTestClients();
  return function ClientsWrapper({ children }: { children: ReactNode }) {
    return <AppProviders clients={clients}>{children}</AppProviders>;
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
// route's component loads in its own chunk, so find it with a findBy query. A test of how the app's
// own clients behave, such as how they retry, passes clients from createClients.
export function renderRoute(path: string, clients: RouterContext = createTestClients()) {
  const router = createAppRouter({
    ...clients,
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  const result = render(
    <AppProviders clients={clients}>
      <RouterProvider router={router} />
    </AppProviders>,
  );
  return { ...result, router };
}
