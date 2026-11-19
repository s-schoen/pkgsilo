import { URL, fileURLToPath } from "node:url";

import { storybookTest } from "@storybook/addon-vitest/vitest-plugin";
import { playwright } from "@vitest/browser-playwright";
import { defineConfig, mergeConfig } from "vitest/config";

import viteConfig from "./vite.config.ts";

// The tests, with the plugins and options of the app (vite.config.ts). A file of its own, so that the
// dev server, the build and Storybook don't load the plugins of the tests. The reports go to build/,
// which git ignores.
export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      // Two projects, which inherit the plugins of vite.config.ts and the options of this section
      // (extends: true), and add up to one coverage report: `vitest --project unit` runs only the first.
      projects: [
        // The tests, which Vitest finds by its default pattern (*.test.tsx and its kin), in jsdom.
        {
          extends: true,
          test: {
            name: "unit",
            environment: "jsdom",
            setupFiles: ["./src/test/setup.ts"],
          },
        },
        // Every story (*.stories.tsx) as a test in Chromium, with its play function and the
        // accessibility check (.storybook/preview.tsx). `task ui:browsers` installs Chromium.
        {
          extends: true,
          plugins: [
            storybookTest({
              configDir: fileURLToPath(new URL("./.storybook", import.meta.url)),
              storybookScript: "pnpm run storybook",
            }),
          ],
          test: {
            name: "storybook",
            browser: {
              enabled: true,
              headless: true,
              provider: playwright(),
              instances: [{ browser: "chromium" }],
            },
          },
        },
      ],
      // Undo vi.spyOn, vi.stubEnv and vi.stubGlobal before each test, so that no test needs to clean up.
      restoreMocks: true,
      unstubEnvs: true,
      unstubGlobals: true,
      coverage: {
        provider: "v8",
        include: ["src/**/*.{ts,tsx}"],
        // The entry point only mounts the router; src/test/ holds the test helpers and the stories
        // are tests; src/api/gen/, src/app/routeTree.gen.ts and src/paraglide/ are generated.
        exclude: [
          "src/main.tsx",
          "src/test/**",
          "src/**/*.stories.tsx",
          "src/api/gen/**",
          "src/app/routeTree.gen.ts",
          "src/paraglide/**",
        ],
        reporter: ["text", "html", "lcov"],
        reportsDirectory: "build/coverage",
      },
    },
  }),
);
