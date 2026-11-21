import { useEffect, useState } from "react";

import {
  applyColorScheme,
  readColorScheme,
  watchColorScheme,
  writeColorScheme,
} from "@/features/appearance/utils/color-scheme.ts";

import type { ColorSchemePreference } from "@/features/appearance/utils/color-scheme.ts";

// The color scheme that the user picked, and a setter that stores the new one and applies it to the
// page. public/color-scheme.js applies the stored one before the first paint, and a pick in another
// tab of this browser reaches this one too.
export function useColorScheme() {
  const [preference, setPreference] = useState(readColorScheme);

  useEffect(
    () =>
      watchColorScheme((next) => {
        applyColorScheme(next);
        setPreference(next);
      }),
    [],
  );

  function changePreference(next: ColorSchemePreference) {
    writeColorScheme(next);
    applyColorScheme(next);
    setPreference(next);
  }

  return [preference, changePreference] as const;
}
