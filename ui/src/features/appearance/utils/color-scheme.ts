import { z } from "zod/v4";

// The color scheme the user picked: system follows the operating system, light and dark force one.
export const colorSchemePreferences = ["system", "light", "dark"] as const;

export type ColorSchemePreference = (typeof colorSchemePreferences)[number];

const storageKey = "pkgsilo.colorScheme";

const preferenceSchema = z.enum(colorSchemePreferences);

// The preference that this browser stored, or system when it stored none, or the storage is
// unavailable (a private window, blocked site data).
export function readColorScheme(): ColorSchemePreference {
  try {
    const parsed = preferenceSchema.safeParse(localStorage.getItem(storageKey));
    return parsed.success ? parsed.data : "system";
  } catch {
    return "system";
  }
}

// Stores the preference in this browser; without storage it lasts until the page reloads.
export function writeColorScheme(preference: ColorSchemePreference) {
  try {
    if (preference === "system") {
      localStorage.removeItem(storageKey);
    } else {
      localStorage.setItem(storageKey, preference);
    }
  } catch {
    // The storage is unavailable: the page still applies the preference.
  }
}

// Applies the preference to the page: src/styles/tokens.css forces the scheme of data-color-scheme
// on <html>, and follows the operating system without it.
export function applyColorScheme(preference: ColorSchemePreference) {
  const root = document.documentElement;
  if (preference === "system") {
    delete root.dataset["colorScheme"];
  } else {
    root.dataset["colorScheme"] = preference;
  }
}
