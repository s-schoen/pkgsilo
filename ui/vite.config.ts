import { URL, fileURLToPath } from "node:url";

import { paraglideVitePlugin } from "@inlang/paraglide-js";
import { devtools } from "@tanstack/devtools-vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The app's dev server and build, not Storybook (whose CLI sets STORYBOOK) or Vitest (VITEST), which
// build the stories and tests with this config too: the devtools and the proxy below are the app's.
const app = !process.env["STORYBOOK"] && !process.env["VITEST"];

// The build goes to dist/, which the Go binary embeds with -tags webui (embed.go). The dev server
// proxies the API and MCP to the server of `task run`. vitest.config.ts adds the tests.
export default defineConfig({
  plugins: [
    // TanStack Devtools (src/main.tsx) for the dev server only: it strips them from the build.
    app && devtools(),
    // Generates src/app/routeTree.gen.ts from src/app/routes/ for the dev server, the build and the
    // tests, and splits each route's component into its own chunk. It reads tsr.config.json, as the
    // routes script in package.json does. It must come before react().
    tanstackRouter(),
    react(),
    // Compiles messages/ into src/paraglide/ for the dev server, the build and the tests. The i18n
    // script in package.json compiles the same for the type check: keep the options in step.
    paraglideVitePlugin({
      project: "./project.inlang",
      outdir: "./src/paraglide",
      // The browser's language, else English. localStorage goes first once the UI can switch the
      // language: the runtime stores the first locale it detects there, which would then stick.
      strategy: ["preferredLanguage", "baseLocale"],
    }),
  ],
  css: {
    // Lightning CSS instead of PostCSS, for the dev server, the build and the tests. It lowers
    // nesting and, with the draft, @custom-media for Vite's default target (baseline widely
    // available), the target of the JS and of the CSS minification as well: so no targets here.
    transformer: "lightningcss",
    lightningcss: {
      drafts: { customMedia: true },
      // The class names of a CSS module (*.module.css) name the file and the class, so that the
      // browser's devtools show where they come from. [local] goes last, as Lightning CSS requires
      // for the line names of a grid's areas.
      cssModules: { pattern: "[name]_[hash]_[local]" },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    // A key matches a URL that starts with it, or one that its pattern matches if it starts with ^.
    // So /api/ and /mcp alone, not a route of the UI that only begins with them, such as /api-keys.
    proxy: app
      ? {
          "/api/": "http://localhost:8080",
          "^/mcp(?:[/?]|$)": "http://localhost:8080",
        }
      : undefined,
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
});
