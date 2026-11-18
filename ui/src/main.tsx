import { DbClient, DbProvider } from "@tanstack/react-db";
import { TanStackDevtools } from "@tanstack/react-devtools";
import { formDevtoolsPlugin } from "@tanstack/react-form-devtools";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtoolsPanel } from "@tanstack/react-query-devtools";
import { RouterProvider } from "@tanstack/react-router";
import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { createAppRouter } from "@/app/router.tsx";
import { configureZodLocale } from "@/lib/zod-locale.ts";
import { getLocale } from "@/paraglide/runtime.js";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

// index.html says en, which is right only for English.
document.documentElement.lang = getLocale();
// The validation errors of the forms, in the same language.
configureZodLocale(getLocale());

// Caches the responses of the API's hooks (src/api/gen/client) for the whole app.
const queryClient = new QueryClient();
// Holds the collections of TanStack DB, which load through the QueryClient: a collection's
// descriptor takes it with client.requireDependency("queryClient").
const dbClient = new DbClient({ queryClient });

const router = createAppRouter({ queryClient, dbClient });

// The devtools show only on the dev server: the devtools plugin in vite.config.ts strips them from
// the build.
createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <DbProvider client={dbClient}>
        <RouterProvider router={router} />
      </DbProvider>
      <TanStackDevtools
        plugins={[
          { name: "TanStack Query", render: <ReactQueryDevtoolsPanel /> },
          { name: "TanStack Router", render: <TanStackRouterDevtoolsPanel router={router} /> },
          formDevtoolsPlugin(),
        ]}
      />
    </QueryClientProvider>
  </StrictMode>,
);
