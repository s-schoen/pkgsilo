// Before any other import, as in main.tsx: zod reads the setting when it creates a schema.
import "@/lib/zod-jitless.ts";
// The tokens and the base of the page, as in main.tsx.
import "@/styles/global.css";
// The preview of addon-mcp, which sizes a story that the MCP tools embed (stories-preview). Storybook
// loads an addon's preview by itself only for a preview without definePreview, and addon-mcp exports
// it to no one, so it's imported by its path.
import "../node_modules/@storybook/addon-mcp/dist/preview.js";
import addonA11y from "@storybook/addon-a11y";
import addonDocs from "@storybook/addon-docs";
import { Controls, Description, Primary, Subtitle, Title } from "@storybook/addon-docs/blocks";
import { definePreview } from "@storybook/react-vite";
import { HttpResponse, http } from "msw";
import addonMsw from "msw-storybook-addon";
import { setupWorker } from "msw/browser";
import { useState } from "react";

import { AppProviders } from "@/app/app-providers.tsx";
import { configureZodLocale } from "@/lib/zod-locale.ts";
import { baseLocale, locales, overwriteGetLocale, toLocale } from "@/paraglide/runtime.js";
import { createTestClients } from "@/test/clients.ts";

import type { Decorator } from "@storybook/react-vite";
import type { ReactNode } from "react";

// Renders the story in the locale of the toolbar: the messages (m.…()), zod's messages and <html lang>,
// as main.tsx sets them for the browser's language. The settings are global, which holds since a
// page renders one story (DocsPage below).
const withLocale: Decorator = (Story, { globals }) => {
  const locale = toLocale(globals["locale"]) ?? baseLocale;
  overwriteGetLocale(() => locale);
  configureZodLocale(locale);
  document.documentElement.lang = locale;
  return <Story />;
};

// Renders the story in the color scheme of the toolbar, whatever the operating system prefers:
// src/styles/tokens.css forces it through the attribute.
const withColorScheme: Decorator = (Story, { globals }) => {
  document.documentElement.dataset["colorScheme"] =
    globals["colorScheme"] === "dark" ? "dark" : "light";
  return <Story />;
};

// Gives each story its own clients, as src/test/render.tsx gives each test.
function StoryClients({ children }: { children: ReactNode }) {
  const [clients] = useState(createTestClients);
  return <AppProviders clients={clients}>{children}</AppProviders>;
}

const withClients: Decorator = (Story) => (
  <StoryClients>
    <Story />
  </StoryClients>
);

// The worker of MSW, which the addon starts for the first story and keeps once it has started. A
// failed start fails that story only: the next one starts it again.
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

// The docs page of a component: Storybook's, without the stories after the first. The stories of a
// page would render at once, and share the handlers of MSW and the locale, which are global.
function DocsPage() {
  return (
    <>
      <Title />
      <Subtitle />
      <Description of="meta" />
      <Description of="story" />
      <Primary />
      <Controls />
    </>
  );
}

export default definePreview({
  addons: [
    addonDocs(),
    addonA11y(),
    // Mocks the API: a story registers the handlers it needs in beforeEach, with
    // `({ msw }) => { msw.use(…) }`, and they're removed after it. A request to the API that no
    // story's handler takes fails, as in the tests (src/test/setup.ts): the handler above survives
    // the reset between stories and takes it after theirs. Storybook's own requests pass.
    addonMsw(startMswWorker),
  ],
  // A docs page for every component, from its first story and its props.
  tags: ["autodocs"],
  parameters: {
    // An accessibility violation fails the story's test, not only the panel. axe skips React Aria's
    // live announcer, which lives in <body> outside the story: its messages are short-lived, and the
    // one that a pending Button announces is labelled by the button, which may be gone by then (a
    // dialog that closes once its action is done).
    a11y: { test: "error", context: { exclude: ["[data-live-announcer]"] } },
    docs: { page: DocsPage },
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
    colorScheme: {
      description: "The color scheme of the tokens",
      toolbar: {
        title: "Color scheme",
        icon: "contrast",
        items: ["light", "dark"],
        dynamicTitle: true,
      },
    },
  },
  // Light, not the operating system's scheme, so that a story's test sees the same colors anywhere.
  // The tests run every story a second time in dark: vitest.config.ts sets STORY_COLOR_SCHEME then.
  initialGlobals: {
    locale: baseLocale,
    colorScheme: import.meta.env["STORY_COLOR_SCHEME"] === "dark" ? "dark" : "light",
  },
  // The last one wraps the others: the locale is outermost, so that it's set before AppProviders
  // reads it for React Aria's I18nProvider.
  decorators: [withColorScheme, withClients, withLocale],
});
