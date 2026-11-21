import { useState } from "react";

import {
  applyColorScheme,
  readColorScheme,
  writeColorScheme,
} from "@/features/appearance/utils/color-scheme.ts";

import type { ColorSchemePreference } from "@/features/appearance/utils/color-scheme.ts";

// The color scheme that the user picked, and a setter that stores the new one and applies it to the
// page. main.tsx applies the stored one before the first render.
export function useColorScheme() {
  const [preference, setPreference] = useState(readColorScheme);

  function changePreference(next: ColorSchemePreference) {
    writeColorScheme(next);
    applyColorScheme(next);
    setPreference(next);
  }

  return [preference, changePreference] as const;
}
