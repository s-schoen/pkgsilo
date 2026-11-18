import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/App.tsx";
import { getLocale } from "@/paraglide/runtime.js";

const root = document.getElementById("root");
if (!root) {
  throw new Error("index.html has no #root element");
}

// index.html says en, which is right only for English.
document.documentElement.lang = getLocale();

// Caches the responses of the API's hooks (src/api/gen/client) for the whole app.
const queryClient = new QueryClient();

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
);
