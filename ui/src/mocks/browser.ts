import { setupWorker } from "msw/browser";

import { createHandlers } from "@/mocks/handlers.ts";

// Starts the mock API of `task ui:dev:mock`: the service worker that vite.config.ts serves answers the
// requests to /api/ in the browser. The data lives in the page, so a reload starts over. A request
// that no handler takes goes on to the dev server's proxy.
export async function startMocking(): Promise<void> {
  const worker = setupWorker(...createHandlers());
  await worker.start({ onUnhandledFrame: "bypass" });
}
