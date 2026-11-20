# Components and styling

How to build a component of the UI library, style it with a CSS module and the tokens, and show an icon.

## The library

- `src/components/ui/` is the UI library: one folder per component, named after it, with the component, its CSS module and its stories, as `button/button.tsx`, `button/button.module.css` and `button/button.stories.tsx`.
  A helper of one component only lives in its folder too.
- The folder's `index.ts` exports what other code uses, which imports it by the folder: `import { Button } from "@/components/ui/button";`, the stories too.
  The other files of the folder import each other by their path, as `button.tsx` imports `@/components/ui/button/button.module.css`.
  Storybook titles the stories `components/ui/<name>`: it drops a file name that repeats its folder's.
- A component wraps one of React Aria Components, imported from its subpath (`react-aria-components/Button`), which brings the behavior, the keyboard and the ARIA.
  The wrapper adds the look and the props of the project, such as `variant`.
- A component styles itself and takes no `className` or `style`: its parent lays it out in its own CSS module, with `gap` or a grid.
- `button/` is the reference component.

## CSS modules

- A component's styles live in `<name>.module.css` in its folder, imported as `styles` and applied as `styles.<class>`.
  Vite compiles them with Lightning CSS (`vite.config.ts`), which lowers nesting and modern CSS for the target browsers.
- Style a state through React Aria's data attribute, such as `&[data-hovered]`, `&[data-pressed]`, `&[data-focus-visible]` or `&[data-disabled]`, which behaves the same for mouse, touch and keyboard.
  React Aria's page of each component lists its attributes.
- A variant is a class, such as `.primary`, that sets local custom properties (`--button-bg`), which the base class reads.
- Every color, space, size, radius, font and duration is a token (`var(--space-2)`); add one to `src/styles/tokens.css` when none fits.
- TypeScript types a module's classes as any string, so a mistyped class is `undefined` at runtime: check the stories after renaming one.

## Tokens and global styles

`src/styles/` holds the global CSS:

- `tokens.css`: the tokens, named by role and then state, as `--color-accent-hovered`; a state is named as React Aria's attribute (`hovered`, `pressed`).
- `global.css`: the tokens, a small reset and the base of the page; `main.tsx` and `.storybook/preview.tsx` import it once.
- `media.css`: the breakpoints, as custom media queries.

What each token means and when a view or a component takes it: `ui/docs/agents/design.md`.

## Color schemes

- A color token is `light-dark(<light>, <dark>)`, in `oklch()`, so the page follows the operating system's scheme.
  `data-color-scheme="light"` or `"dark"` on `<html>` forces one.
- Lightning CSS lowers `light-dark()` to variables that it switches on `:root`.
  So a color token is defined on `:root`, and a scheme applies to the whole page, never to a part of it.
- Every story's axe check tests the contrast of the light scheme; a story with `globals: { colorScheme: "dark" }` tests the dark one.

## Breakpoints

Import the breakpoints at the top of the module, then use them as media queries:

```css
@import "@/styles/media.css";

.list {
  @media (--md) {
    grid-template-columns: 1fr 1fr;
  }
}
```

Lightning CSS replaces the queries when it compiles the module, so `media.css` adds no CSS, however many modules import it.

## Icons

- The icons are Tabler's, from `tabler.io/icons`: import each one from its own module, as `import IconPlus from "@tabler/icons-react/icons/IconPlus";`.
  `no-restricted-imports` rejects the package root, which makes Vite, Vitest and Storybook compile all ~6,000 icons; a type such as `TablerIcon` comes from the root with `import type`.
- A component that shows an icon takes it as a prop of type `TablerIcon` (`icon={IconPlus}`), renders it with `aria-hidden`, and sizes it in `em` in its module, so that it follows the text.
- An icon is decorative: the control carries the name, through its text or its `aria-label`.

## Stories

On top of `ui/docs/agents/stories.md`: a story for each variant and state, a play function for each way to use the component (pointer and keyboard), and one story in the dark scheme.

## Content-Security-Policy

Styles live in CSS files: the UI's CSP (`docs/agents/ui.md`) blocks `<style>` elements and the `style` attributes of HTML.
React's `style` prop, and the positioning of React Aria's overlays, pass: they set the styles through the CSSOM.

## Locale

React Aria formats dates and numbers, and has strings of its own (in ComboBox, DatePicker and others), in the locale of its `I18nProvider`, else the browser's.
Before the first such component:

- Wrap the app in `I18nProvider` with paraglide's `getLocale()`, wherever `AppProviders` wraps it: `main.tsx`, `src/test/render.tsx` and `.storybook/preview.tsx`.
- Add `@react-aria/optimize-locales-plugin` to `vite.config.ts` with the UI's locales, so that the build drops React Aria's other languages.
