// Before any other import: zod reads the setting when it creates a schema.
import "@/lib/zod-jitless.ts";
// The tokens and the base of the page, before any component's styles.
import "@/styles/global.css";
import { TanStackDevtools } from "@tanstack/react-devtools";
import { formDevtoolsPlugin } from "@tanstack/react-form-devtools";
import { ReactQueryDevtoolsPanel } from "@tanstack/react-query-devtools";
import { RouterProvider } from "@tanstack/react-router";
import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { tableDevtoolsPlugin } from "@tanstack/react-table-devtools";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { AppProviders } from "@/app/app-providers.tsx";
import { createClients } from "@/app/clients.ts";
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

// `task ui:dev:mock` (`vite --mode mock`): the browser answers the API. It starts before the first
// render, so that no request escapes it. Vite replaces the mode of any other build with a constant,
// so this branch and the import leave the production build.
if (import.meta.env.MODE === "mock") {
  const { startMocking } = await import("@/mocks/browser.ts");
  await startMocking();
}

const clients = createClients();

const router = createAppRouter(clients);

// The devtools show only on the dev server: the devtools plugin in vite.config.ts strips them from
// the build.
createRoot(root).render(
  <StrictMode>
    <AppProviders clients={clients}>
      <RouterProvider router={router} />
      <TanStackDevtools
        plugins={[
          { name: "TanStack Query", render: <ReactQueryDevtoolsPanel /> },
          { name: "TanStack Router", render: <TanStackRouterDevtoolsPanel router={router} /> },
          formDevtoolsPlugin(),
          // Shows the tables that call useTanStackTableDevtools with a key.
          tableDevtoolsPlugin(),
        ]}
      />
    </AppProviders>
  </StrictMode>,
);
