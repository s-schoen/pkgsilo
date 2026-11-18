import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createMemoryHistory } from "@tanstack/react-router";
import { render, renderHook } from "@testing-library/react";

import { createAppRouter } from "@/app/router.tsx";

import type { ReactElement, ReactNode } from "react";

// A QueryClient for one test, so that no cached response leaks into another test. Queries don't
// retry, so that a query fails at the first error response.
function testQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function queryWrapper() {
  const client = testQueryClient();
  return function QueryWrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

// render() for a component that uses the API's hooks.
export function renderWithClient(ui: ReactElement) {
  return render(ui, { wrapper: queryWrapper() });
}

// renderHook() for a hook that uses the API's hooks.
export function renderHookWithClient<Result>(hook: () => Result) {
  return renderHook(hook, { wrapper: queryWrapper() });
}

// Renders the whole app at path, with the routes of src/app/routes, as the browser would. The
// route's component loads in its own chunk, so find it with a findBy query.
export function renderRoute(path: string) {
  const queryClient = testQueryClient();
  const router = createAppRouter({
    queryClient,
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  const result = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { ...result, router };
}
