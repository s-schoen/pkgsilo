import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { URL, fileURLToPath } from "node:url";

// The build goes to dist/, which the Go binary embeds with -tags webui (embed.go). The dev server
// proxies the API and MCP to the server of `task run`.
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
});
