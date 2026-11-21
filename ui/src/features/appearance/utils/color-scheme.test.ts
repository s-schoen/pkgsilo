import { afterEach, describe, expect, it, onTestFinished, vi } from "vitest";

import colorSchemeScript from "@/../public/color-scheme.js?raw";
import {
  applyColorScheme,
  readColorScheme,
  watchColorScheme,
  writeColorScheme,
} from "@/features/appearance/utils/color-scheme.ts";

import type { ColorSchemePreference } from "@/features/appearance/utils/color-scheme.ts";

afterEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset["colorScheme"];
});

// What the storage event of another tab tells this one.
function storeInAnotherTab(key: string | null, value: string | null) {
  if (key === null) {
    localStorage.clear();
  } else if (value !== null) {
    localStorage.setItem(key, value);
  }
  window.dispatchEvent(new StorageEvent("storage", { key, newValue: value }));
}

// Runs public/color-scheme.js as index.html does, as a classic script.
function runColorSchemeScript() {
  const script = document.createElement("script");
  script.textContent = colorSchemeScript;
  document.head.append(script);
  script.remove();
}

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

describe("watchColorScheme", () => {
  it("reports the preference that another tab stores", () => {
    const onChange = vi.fn<(preference: ColorSchemePreference) => void>();
    onTestFinished(watchColorScheme(onChange));

    storeInAnotherTab("pkgsilo.colorScheme", "dark");

    expect(onChange).toHaveBeenCalledExactlyOnceWith("dark");
  });

  it("reports system when another tab clears the storage", () => {
    writeColorScheme("dark");
    const onChange = vi.fn<(preference: ColorSchemePreference) => void>();
    onTestFinished(watchColorScheme(onChange));

    storeInAnotherTab(null, null);

    expect(onChange).toHaveBeenCalledExactlyOnceWith("system");
  });

  it("ignores the other keys", () => {
    const onChange = vi.fn<(preference: ColorSchemePreference) => void>();
    onTestFinished(watchColorScheme(onChange));

    storeInAnotherTab("another.key", "dark");

    expect(onChange).not.toHaveBeenCalled();
  });

  it("stops when it's told to", () => {
    const onChange = vi.fn<(preference: ColorSchemePreference) => void>();
    const stop = watchColorScheme(onChange);

    stop();
    storeInAnotherTab("pkgsilo.colorScheme", "dark");

    expect(onChange).not.toHaveBeenCalled();
  });
});

describe("public/color-scheme.js", () => {
  it.each(["light", "dark"] as const)("applies the stored preference %s", (preference) => {
    writeColorScheme(preference);

    runColorSchemeScript();

    expect(document.documentElement.dataset["colorScheme"]).toBe(preference);
  });

  it("leaves the scheme to the system when nothing is stored", () => {
    writeColorScheme("system");

    runColorSchemeScript();

    expect(document.documentElement).not.toHaveAttribute("data-color-scheme");
  });

  it("leaves the scheme to the system for a value it doesn't know", () => {
    localStorage.setItem("pkgsilo.colorScheme", "sepia");

    runColorSchemeScript();

    expect(document.documentElement).not.toHaveAttribute("data-color-scheme");
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
