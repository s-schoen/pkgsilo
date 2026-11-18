import react from "@vitejs/plugin-react";
import { URL, fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

// The build goes to dist/, which the Go binary embeds with -tags webui (embed.go). The dev server
// proxies the API and MCP to the server of `task run`. Vitest reads the test section; its reports go
// to build/, which git ignores.
export default defineConfig({
  plugins: [react()],
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
      // The entry point only mounts App; src/test/ holds the test helpers.
      exclude: ["src/main.tsx", "src/test/**"],
      reporter: ["text", "html", "lcov"],
      reportsDirectory: "build/coverage",
    },
  },
});
