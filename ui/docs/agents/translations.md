# Translations

How the UI's texts become messages in every locale.

Paraglide JS compiles the messages in `ui/messages/{locale}.json` into typed functions in `ui/src/paraglide/`.
English (`en`) is the base locale, and German (`de`) is the other one (`ui/project.inlang/settings.json`).

- Every text the user reads is a message: `import { m } from "@/paraglide/messages.js"`, then `m.welcome()`.
  The product name pkgsilo isn't translated.
- The lint rule `react/jsx-no-literals` rejects text in JSX outside the tests: text and strings as children, and strings on the user-facing attributes it lists in `restrictedAttributes` (`alt`, `aria-label`, `placeholder`, `title` and a few more), on any element or component.
  Add an attribute there when a component takes text in a new prop, and a string that must stay as it is to `allowedStrings`, as pkgsilo is.
  It doesn't check the strings in a condition, as in `{ok ? "Yes" : "No"}`, or in a variable: call messages there too (`{ok ? m.yes() : m.no()}`).
- Add a new key to every locale's file.
- Name a message statically (`m.welcome`): the lint rule `paraglide/no-dynamic-message-access` rejects `m[key]`, since that would bundle every message.
  To pick a message at runtime, index a local map such as `{ welcome: m.welcome }`.
- `src/paraglide/` is generated and ignored by git.
  The Vite plugin compiles it for `task ui:dev`, `task ui:build` and `task ui:test`, and `task ui:i18n` compiles it for the type check of the build and the lint, which depend on that task.
  Its options are set twice, in `vite.config.ts` and in the `i18n` script of `package.json`: keep them in step.
- The message-format plugin is loaded from `node_modules`, so the compiler needs no network.
  Paths in `settings.json` are relative to `ui/`, not to `project.inlang/`.
  If the plugin can't be loaded, the compiler only warns and emits no messages, and the type check then fails on every `m.…` call.
- The locale follows the browser's languages (`preferredLanguage`), else English (`baseLocale`).
  `main.tsx` sets `<html lang>` to it.
  A language switcher would put `localStorage` first and call `setLocale()`, which stores the choice and reloads the page.
  Not before: the runtime stores the first locale it detects there, which would then stick.
- A test picks another locale with `vi.spyOn(navigator, "languages", "get").mockReturnValue(["de-DE"])`, and finds the text with the message in that locale: `m.welcome({}, { locale: "de" })`.
  The tests run with jsdom's `en-US` otherwise.
