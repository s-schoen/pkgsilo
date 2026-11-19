// Before any other import, as in main.tsx: zod reads the setting when it creates a schema.
import "@/lib/zod-jitless.ts";
import addonA11y from "@storybook/addon-a11y";
import addonDocs from "@storybook/addon-docs";
import { definePreview } from "@storybook/react-vite";
import { HttpResponse, http } from "msw";
import addonMsw from "msw-storybook-addon";
import { setupWorker } from "msw/browser";
import { useState } from "react";

import { AppProviders } from "@/app/app-providers.tsx";
import { createClients } from "@/app/clients.ts";
import { configureZodLocale } from "@/lib/zod-locale.ts";
import { baseLocale, locales, overwriteGetLocale, toLocale } from "@/paraglide/runtime.js";

import type { Decorator } from "@storybook/react-vite";
import type { SetupWorker } from "msw/browser";
import type { ReactNode } from "react";

// Renders the story in the locale of the toolbar: the messages (m.…()), zod's messages and <html lang>,
// as main.tsx sets them for the browser's language.
const withLocale: Decorator = (Story, { globals }) => {
  const locale = toLocale(globals["locale"]) ?? baseLocale;
  overwriteGetLocale(() => locale);
  configureZodLocale(locale);
  document.documentElement.lang = locale;
  return <Story />;
};

// Gives each story its own clients, as src/test/render.tsx gives each test, so that no cached
// response or collection leaks into another story. Queries and mutations don't retry, so that a
// story shows the first error response.
function StoryClients({ children }: { children: ReactNode }) {
  const [clients] = useState(() =>
    createClients({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }),
  );
  return <AppProviders clients={clients}>{children}</AppProviders>;
}

const withClients: Decorator = (Story) => (
  <StoryClients>
    <Story />
  </StoryClients>
);

// The one worker of MSW for all stories. The addon keeps the worker only once it has started, so
// the stories of a docs page, which render at once, would each start one, and the second fails:
// they share the promise instead.
let mswWorker: Promise<SetupWorker> | undefined;

async function startMswWorker() {
  const worker = setupWorker(
    http.all("/api/*", ({ request }) => {
      console.error(`No MSW handler of the story takes ${request.method} ${request.url}`);
      return HttpResponse.error();
    }),
  );
  await worker.start({ quiet: true, onUnhandledFrame: "bypass" });
  return worker;
}

export default definePreview({
  addons: [
    addonDocs(),
    addonA11y(),
    // Mocks the API: a story registers the handlers it needs in beforeEach, with
    // `({ msw }) => { msw.use(…) }`, and they're removed after it. A request to the API that no
    // story's handler takes fails, as in the tests (src/test/setup.ts), instead of reaching the
    // proxy of the dev server. The handler below survives the reset between stories and takes it
    // after theirs; Storybook's own requests pass.
    addonMsw(() => (mswWorker ??= startMswWorker())),
  ],
  // A docs page for every component, from its stories and its props.
  tags: ["autodocs"],
  parameters: {
    // An accessibility violation fails the story's test, not only the panel.
    a11y: { test: "error" },
  },
  globalTypes: {
    locale: {
      description: "The locale of the messages",
      toolbar: {
        title: "Locale",
        icon: "globe",
        items: [...locales],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { locale: baseLocale },
  // The last one wraps the others: the clients are outermost.
  decorators: [withLocale, withClients],
});
