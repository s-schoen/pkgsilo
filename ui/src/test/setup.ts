// Before any other import, as in main.tsx: zod reads the setting when it creates a schema.
import "@/lib/zod-jitless.ts";
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll } from "vitest";

import { configureZodLocale } from "@/lib/zod-locale.ts";
import { server } from "@/test/server.ts";

// The router restores the scroll position on navigation, and jsdom has no layout to scroll: its
// scrollTo() only logs that it isn't implemented.
window.scrollTo = () => {};

// jsdom has no matchMedia either: no media query matches, and none ever changes.
window.matchMedia = (media) => ({
  matches: false,
  media,
  onchange: null,
  addEventListener: () => {},
  removeEventListener: () => {},
  addListener: () => {},
  removeListener: () => {},
  dispatchEvent: () => false,
});

// zod's messages in English, as main.tsx sets them for jsdom's en-US. A test of another locale sets
// it back after itself, since the setting is global.
configureZodLocale("en");

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
