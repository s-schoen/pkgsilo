import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "./server.ts";

beforeAll(() => {
  server.listen({ onUnhandledFrame: "error" });
});

// Testing Library unmounts after each test by itself only with Vitest's globals, which are off.
afterEach(() => {
  cleanup();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});
