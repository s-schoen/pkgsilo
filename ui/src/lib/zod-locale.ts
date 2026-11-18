import { z } from "zod/v4";
import { de, en } from "zod/v4/locales";

import type { Locale } from "@/paraglide/runtime.js";

// zod's messages for each locale of the UI, so that a form's validation errors (from the schemas in
// src/api/gen/zod) are in the user's language. A new locale fails the type check until it's here.
// Imported one by one, so that the bundle has only these.
const zodLocales: Record<Locale, typeof en> = { en, de };

// Sets the locale of zod's messages, for every schema: main.tsx calls it with the UI's locale.
export function configureZodLocale(locale: Locale) {
  z.config(zodLocales[locale]());
}
