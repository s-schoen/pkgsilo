import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import type { StorybookConfig } from "@storybook/react-vite";
import type { Plugin } from "vite";

// Storybook builds the stories with vite.config.ts, as the app, and the storybook project of Vitest
// (vitest.config.ts) runs each story as a test in Chromium. `task ui:storybook` serves it on :6006.
const config: StorybookConfig = {
  framework: "@storybook/react-vite",
  stories: ["../src/**/*.stories.tsx"],
  addons: [
    "@storybook/addon-docs",
    "@storybook/addon-a11y",
    "@storybook/addon-vitest",
    // Serves the components and their stories to agents at /mcp (.mcp.json at the repository root).
    "@storybook/addon-mcp",
  ],
  typescript: {
    // react-docgen-typescript needs TypeScript's JavaScript API, which TypeScript 7 doesn't have.
    reactDocgen: "react-docgen",
  },
  core: {
    disableTelemetry: true,
  },
  // Storybook and the storybook project of Vitest add the plugins of viteFinal.
  viteFinal: (viteConfig) => ({
    ...viteConfig,
    plugins: [...(viteConfig.plugins ?? []), mswServiceWorker()],
  }),
};

// Serves the service worker of MSW, which msw-storybook-addon registers at /mockServiceWorker.js,
// from the installed msw, and adds it to the build. So no copy of it has to be generated first, as
// `msw init` would, and none goes stale when msw is updated.
function mswServiceWorker(): Plugin {
  const path = fileURLToPath(import.meta.resolve("msw/mockServiceWorker.js"));
  return {
    name: "pkgsilo:msw-service-worker",
    configureServer(server) {
      server.middlewares.use("/mockServiceWorker.js", (_request, response) => {
        response.setHeader("Content-Type", "text/javascript");
        response.end(readFileSync(path));
      });
    },
    generateBundle() {
      this.emitFile({
        type: "asset",
        fileName: "mockServiceWorker.js",
        source: readFileSync(path),
      });
    },
  };
}

export default config;
