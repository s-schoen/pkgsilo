import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, renderHook } from "@testing-library/react";

import type { ReactElement, ReactNode } from "react";

// Each render gets its own QueryClient, so that no cached response leaks into another test, and
// queries don't retry, so that a query fails at the first error response.
function queryWrapper() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
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
