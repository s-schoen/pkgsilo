import { TanStackDevtools } from "@tanstack/react-devtools";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtoolsPanel } from "@tanstack/react-query-devtools";
import { RouterProvider } from "@tanstack/react-router";
import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { createAppRouter } from "@/app/router.tsx";
import { getLocale } from "@/paraglide/runtime.js";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

// index.html says en, which is right only for English.
document.documentElement.lang = getLocale();

// Caches the responses of the API's hooks (src/api/gen/client) for the whole app.
const queryClient = new QueryClient();

const router = createAppRouter({ queryClient });

// The devtools show only on the dev server: the devtools plugin in vite.config.ts strips them from
// the build.
createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
      <TanStackDevtools
        plugins={[
          { name: "TanStack Query", render: <ReactQueryDevtoolsPanel /> },
          { name: "TanStack Router", render: <TanStackRouterDevtoolsPanel router={router} /> },
        ]}
      />
    </QueryClientProvider>
  </StrictMode>,
);
