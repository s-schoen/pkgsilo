import { afterEach, describe, expect, it, vi } from "vitest";

import {
  applyColorScheme,
  readColorScheme,
  writeColorScheme,
} from "@/features/appearance/utils/color-scheme.ts";

afterEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset["colorScheme"];
});

describe("readColorScheme", () => {
  it("follows the system when nothing is stored", () => {
    expect(readColorScheme()).toBe("system");
  });

  it("reads the stored preference", () => {
    writeColorScheme("dark");

    expect(readColorScheme()).toBe("dark");
  });

  it("follows the system for a value it doesn't know", () => {
    localStorage.setItem("pkgsilo.colorScheme", "sepia");

    expect(readColorScheme()).toBe("system");
  });

  it("follows the system when the storage is unavailable", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("blocked", "SecurityError");
    });

    expect(readColorScheme()).toBe("system");
  });
});

describe("writeColorScheme", () => {
  it("forgets the stored preference for system", () => {
    writeColorScheme("light");
    writeColorScheme("system");

    expect(localStorage.length).toBe(0);
  });

  it("does nothing when the storage is unavailable", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("full", "QuotaExceededError");
    });

    expect(() => {
      writeColorScheme("dark");
    }).not.toThrow();
  });
});

describe("applyColorScheme", () => {
  it("forces light or dark on <html>", () => {
    applyColorScheme("dark");
    expect(document.documentElement.dataset["colorScheme"]).toBe("dark");

    applyColorScheme("light");
    expect(document.documentElement.dataset["colorScheme"]).toBe("light");
  });

  it("leaves the scheme to the system", () => {
    applyColorScheme("dark");
    applyColorScheme("system");

    expect(document.documentElement).not.toHaveAttribute("data-color-scheme");
  });
});
