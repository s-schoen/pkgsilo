import type { StorybookConfig } from "@storybook/react-vite";
import type { PluginOption } from "vite";

// Storybook builds the stories with vite.config.ts, as the app, and the storybook project of Vitest
// (vite.config.ts) runs each story as a test in Chromium. `task ui:storybook` serves it on :6006.
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
  // mockServiceWorker.js of msw-storybook-addon, which `task ui:msw` generates. Not in ui/public/,
  // which Vite would copy into the build that the binary embeds.
  staticDirs: ["./public"],
  typescript: {
    // react-docgen-typescript needs TypeScript's JavaScript API, which TypeScript 7 doesn't have.
    reactDocgen: "react-docgen",
  },
  core: {
    disableTelemetry: true,
  },
  viteFinal: (viteConfig) => ({
    ...viteConfig,
    // Without TanStack Devtools: the stories don't mount them, and their event-bus server would
    // take the port of a running `task ui:dev`.
    plugins: withoutDevtools(viteConfig.plugins ?? []),
  }),
};

function withoutDevtools(plugins: Array<PluginOption>): Array<PluginOption> {
  return plugins.flat().filter((plugin) => {
    const name = plugin && "name" in plugin ? plugin.name : "";
    return !name.startsWith("@tanstack/devtools");
  });
}

export default config;
