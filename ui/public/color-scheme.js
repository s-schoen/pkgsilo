// Applies the color scheme that the user picked in this browser before the first paint, so that a
// page whose scheme differs from the operating system's doesn't flash in the wrong one. index.html
// loads it as a classic script in <head>, which runs before the browser renders the body, unlike
// the module of main.tsx; a file, since the Content-Security-Policy forbids an inline script.
// src/features/appearance/utils/color-scheme.ts stores the preference, and its test runs this file,
// so that the key and the values stay in step.
try {
  const preference = localStorage.getItem("pkgsilo.colorScheme");
  if (preference === "light" || preference === "dark") {
    document.documentElement.dataset.colorScheme = preference;
  }
} catch {
  // The storage is unavailable: the page follows the operating system.
}
