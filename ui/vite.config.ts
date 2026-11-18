import { URL, fileURLToPath } from "node:url";

import { paraglideVitePlugin } from "@inlang/paraglide-js";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// The build goes to dist/, which the Go binary embeds with -tags webui (embed.go). The dev server
// proxies the API and MCP to the server of `task run`. Vitest reads the test section; its reports go
// to build/, which git ignores.
export default defineConfig({
  plugins: [
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
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/mcp": "http://localhost:8080",
    },
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    // Undo vi.spyOn, vi.stubEnv and vi.stubGlobal before each test, so that no test needs to clean up.
    restoreMocks: true,
    unstubEnvs: true,
    unstubGlobals: true,
    coverage: {
      provider: "v8",
      include: ["src/**/*.{ts,tsx}"],
      // The entry point only mounts App; src/test/ holds the test helpers; src/paraglide/ is generated.
      exclude: ["src/main.tsx", "src/test/**", "src/paraglide/**"],
      reporter: ["text", "html", "lcov"],
      reportsDirectory: "build/coverage",
    },
  },
});
