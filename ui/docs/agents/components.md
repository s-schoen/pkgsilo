# Components and styling

How to build a component of the UI library, style it with a CSS module and the tokens, and show an icon.

## The library

- `src/components/ui/` is the UI library: one folder per component, named after it, with the component, its CSS module and its stories, as `button/button.tsx`, `button/button.module.css` and `button/button.stories.tsx`.
  A helper of one component only lives in its folder too.
- The folder's `index.ts` exports what other code uses, which imports it by the folder: `import { Button } from "@/components/ui/button";`, the stories too.
  The other files of the folder import each other by their path, as `button.tsx` imports `@/components/ui/button/button.module.css`, but never the files of another component, which `no-restricted-imports` checks.
  Storybook titles the stories `components/ui/<name>`: it drops a file name that repeats its folder's.
- A component wraps one of React Aria Components, imported from its subpath (`react-aria-components/Button`), which brings the behavior, the keyboard and the ARIA.
  The wrapper adds the look and the props of the project, such as `variant`.
- A component styles itself and takes no `className` or `style`: its parent lays it out in its own CSS module, with `gap` or a grid.
- `button/` is the reference component.
- A field (`text-field/`, `select/`, `combo-box/`, `checkbox/`, `switch/`) takes `label`, `description` and `errorMessage` as messages, shows the error while `isInvalid` is set, and renders the parts of `field/` (`Label`, `Description`, `FieldError`) inside the React Aria field that it wraps.
  Its `validationBehavior` is always `"aria"`: a form's schema validates it, never the browser.
  A form binds it through `@/components/form` (`ui/docs/agents/forms.md`).
- A component that only React Aria's behavior needs, such as `MenuTrigger`, `TooltipTrigger` or `DialogTrigger`, is re-exported as it is from the folder of the component it pairs with.

## The components

| Need                                              | Component                                                     |
| ------------------------------------------------- | ------------------------------------------------------------- |
| An action                                         | `button` (`isPending` while it runs)                          |
| Navigation                                        | `link`, `tabs` (a `Tab` with `href`)                          |
| The links between the app's pages                 | `nav-list` (`NavList`, `NavItem`)                             |
| Text input                                        | `text-field` (`TextField`, `TextArea`)                        |
| A search, in a form or a toolbar                  | `search-field` (`variant="bare"` in a toolbar)                |
| One of a few options                              | `select`                                                      |
| One of many options, by typing                    | `combo-box`                                                   |
| A yes or no that a form submits                   | `checkbox`                                                    |
| A setting that applies at once                    | `switch`                                                      |
| Options that show or hide something, any of them  | `toggle-button` (`ToggleButton`, `ToggleButtonGroup`)         |
| Filters to check, out of many, by searching       | `filter-list` (`FilterList`, `FilterListSection`)             |
| Actions of a row or a toolbar                     | `menu`                                                        |
| The name of an icon-only button                   | `tooltip`                                                     |
| A task that interrupts the page                   | `modal`                                                       |
| A panel from the edge, as the small screen's menu | `drawer`                                                      |
| A question before an action that can't be undone  | `confirm-dialog`                                              |
| The outcome of an action                          | `toast` (`showToast()`; the root route renders the `Toaster`) |
| A failure on the page itself, as a form's         | `alert`                                                       |
| Loading                                           | `spinner`                                                     |
| A status                                          | `badge`                                                       |
| A line between content                            | `separator`                                                   |
| Building blocks of the above                      | `field`, `popover`, `list-box`                                |

`toast/` builds on React Aria's `UNSTABLE_` toasts: only that folder names them, so that a change of their API stays there.

## CSS modules

- A component's styles live in `<name>.module.css` in its folder, imported as `styles` and applied as `styles.<class>`.
  Vite compiles them with Lightning CSS (`vite.config.ts`), which lowers nesting and modern CSS for the target browsers.
- Style a state through React Aria's data attribute, such as `&[data-hovered]`, `&[data-pressed]`, `&[data-focus-visible]` or `&[data-disabled]`, which behaves the same for mouse, touch and keyboard.
  React Aria's page of each component lists its attributes.
- A class is camelCase (`.iconOnly`), as `styles.iconOnly` reads it.
- A variant is a class, such as `.primary`, that sets local custom properties (`--button-bg`), which the base class reads.
  A module names its own properties after itself and never sets a token.
- Every color, space, size, radius, font and duration is a token (`var(--space-2)`); add one to `src/styles/tokens.css` when none fits.
  Only `em` (an icon's size) and `%` stay literal.
- A property is logical, such as `inline-size`, `padding-inline` or `margin-block-end`, so that it follows the writing mode.
  Scrolling is the exception: Vite's target browsers lack `overflow-block` and `overflow-inline` (Chrome 135, Safari 26), so a module writes `overflow-y` or `overflow-x` and disables `property-layout-mappings` for that line.
- Name a grid's areas in `grid-template-areas`, and set its sizes in `grid-template-rows` and `grid-template-columns`, not all in the `grid-template` shorthand: Lightning CSS renames a module's area names, but leaves a value with `var()` as it is, so the names of a shorthand with a token among its sizes no longer match the renamed `grid-area`.
- The properties follow a grouped order (position, box, text, look, motion), which `task ui:lint:fix` sorts.
- TypeScript types a module's classes as any string, so a mistyped class is `undefined` at runtime: check the stories after renaming one.

`stylelint.config.ts` checks these rules, the breakpoints below, and that a module styles only its own classes (no `:root`, `body` or `--color-canvas`); `task ui:lint` runs it.

## Tokens and global styles

`src/styles/` holds the global CSS:

- `tokens.css`: the tokens, named by role and then state, as `--color-accent-hovered`; a state is named as React Aria's attribute (`hovered`, `pressed`).
- `global.css`: the tokens, a small reset and the base of the page; `main.tsx` and `.storybook/preview.tsx` import it once.
- `media.css`: the breakpoints, as custom media queries.

What each token means and when a view or a component takes it: `ui/docs/agents/design.md`.

## Color schemes

- A color token is `light-dark(<light>, <dark>)`, in `oklch()`, so the page follows the operating system's scheme; a color that both schemes share is a single value, and one that repeats another token is `var()` of it.
  `data-color-scheme="light"` or `"dark"` on `<html>` forces one.
- Lightning CSS lowers `light-dark()` to variables that it switches on `:root`.
  So a color token is defined on `:root`, and a scheme applies to the whole page, never to a part of it.
- The tests run every story in both schemes (the browser instances `storybook light` and `storybook dark` in `vitest.config.ts`), so its axe check tests the contrast of each.

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

On top of `ui/docs/agents/stories.md`: a story for each variant and state, and a play function for each way to use the component (pointer and keyboard); the tests run each story in the dark scheme too.

## Content-Security-Policy

Styles live in CSS files: the UI's CSP (`docs/agents/ui.md`) blocks `<style>` elements and the `style` attributes of HTML.
React's `style` prop, and the positioning of React Aria's overlays, pass: they set the styles through the CSSOM.

## Locale

React Aria formats dates and numbers, and has strings of its own (a ComboBox's button, a toast region's name), in the locale of the messages: `AppProviders` wraps the app, the tests and the stories in its `I18nProvider` with paraglide's `getLocale()`.
`@react-aria/optimize-locales-plugin` in `vite.config.ts` keeps only React Aria's strings of the UI's locales (`project.inlang/settings.json`).

## Links and routes

React Aria's links (`Link`, a `MenuItem`'s or a `Tab`'s `href`) navigate through TanStack Router: `src/app/root-layout.tsx` wraps the routes in React Aria's `RouterProvider`, and types `href` as a route, which the type check checks as it checks `<Link to>`.
Outside a router, as in a story, a link is a plain `<a>`.

Unlike TanStack's `<Link>`, a React Aria link doesn't preload its route on the user's intent by itself: spread `useRoutePreload(href)` (`src/hooks/use-route-preload.ts`) onto it, as `NavItem` and the layout's brand link do, and the router preloads it once the pointer rests on it, it's focused, or a touch starts on it.

## Overlays

A popover, a menu, a tooltip, a modal and a toast render in a portal at the end of `<body>`, which stacks them in the order they open: none sets a `z-index`.
They enter and leave on `[data-entering]` and `[data-exiting]` (`ui/docs/agents/design.md`).
