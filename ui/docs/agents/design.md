# Design

How a view or a component of the UI looks, and how to take a new component's look from Plane.

The look follows Plane (`github.com/makeplane/plane`): calm and dense.
Neutral greys carry the structure; color marks only the main action, the selection and a status.
The tokens in `src/styles/tokens.css` hold Plane's values; `ui/docs/agents/components.md` explains how a module uses them.

## Principles

- A view has one primary button; its other actions are secondary, and the actions of a toolbar or a row tertiary or ghost.
- A border is `--color-border-subtle`; `--color-border-strong` sets a control apart.
- A shadow lifts only a raised control (`--shadow-raised`) and an overlay (`--shadow-overlay`); everything else is flat.
- Controls are md (28 px) with 13 px text, and the page's text is 14 px.
- The canvas is grey: white belongs to a surface, never to the page itself.

## Depth: canvas, surfaces, layers

- The canvas (`--color-canvas`) is the page itself: `global.css` sets it on `<body>`, and nothing else uses it.
- A surface (`--color-surface-1`, `-2`) sits on the canvas, as the frame of `AppLayout`'s page does.
  Surfaces are siblings, never nested; an overlay (a dialog, a popover) is on its own plane and may use a surface again.
- A layer (`--color-layer-1` to `-3`) adds depth inside a surface: layer-1 in surface-1, layer-2 in surface-2, and the next layer inside a layer.
  A control may take the next layer to stand out, as the secondary button's layer-2.
- A state's shade matches its layer: `--color-layer-1` turns `--color-layer-1-hovered`, never another layer's shade.
  A row or a button without a fill takes the transparent layer's shades (`--color-layer-transparent-hovered`).
- Text takes `--color-text`, then `-secondary`, `-tertiary` and `-placeholder` as it matters less.

## Typography

| Text            | Size             | Weight                   | Color                   |
| --------------- | ---------------- | ------------------------ | ----------------------- |
| Page title      | `--font-size-xl` | `--font-weight-semibold` | `--color-text`          |
| Section heading | `--font-size-lg` | `--font-weight-semibold` | `--color-text`          |
| Body            | `--font-size-md` | `--font-weight-normal`   | `--color-text`          |
| Control         | `--font-size-sm` | `--font-weight-medium`   | as the variant sets     |
| Metadata        | `--font-size-xs` | `--font-weight-normal`   | `--color-text-tertiary` |

The font is Inter Variable from `@fontsource-variable/inter`, which `global.css` loads from the UI's own files.

## Sizes and motion

- A control takes its height, inline padding and icon gap from the control sizes (`--control-height-md`, `--control-padding-md`, `--control-gap-md`), md unless a view needs another, and its text in `--font-size-sm`.
- A control's state changes over `--duration-normal` with `--ease-out`, as the button's background.
- An overlay enters and leaves on React Aria's `[data-entering]` and `[data-exiting]`: from `scale(var(--scale-overlay-start))` and `opacity: 0`, over `--duration-fast`, with `transform-origin: var(--trigger-anchor-point)`.
  Its backdrop (`--color-backdrop`) fades over `--duration-normal`, and a drawer slides over `--duration-slow`.
- `tokens.css` sets the durations to 0 when the user prefers reduced motion, so a module never needs its own query.

## A new component from Plane's recipe

Plane's components live in its published package `@makeplane/propel`, as Tailwind classes.

1. Unpack it in a scratch directory, outside the repo: `npm pack @makeplane/propel && tar xzf makeplane-propel-*.tgz`.
2. Read the recipe of the component: `package/dist/elements/<name>/variants.js`, and the chrome it composes from `package/dist/internal/`: `control-chrome.js` for a button-like control, `popup-surface.js` and `overlay-panel.js` for a popup, `backdrop.js` for a modal's backdrop, `surface.js` for a card.
   The values of its tokens are in `package/dist/styles/variables.css`, the light ones first, the dark ones under `@variant dark`.
3. Write the component's CSS module from the recipe, through the table below.
   A token that Plane uses and `tokens.css` lacks, such as a success or a warning color, comes from Plane's light and dark values, as `light-dark()`.
4. The look is done when every story of the component passes axe in the light and the dark scheme.
   Where Plane's pair fails the contrast, shift our token's lightness until it passes, and say why in `tokens.css`, as the dark danger fill does.

| Plane (Tailwind)                                                                   | Ours                                                                                                            |
| ---------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `bg-layer-2`, `hover:bg-layer-2-hover`, `active:bg-layer-2-active`                 | `--color-layer-2`, its `-hovered` on `[data-hovered]`, its `-pressed` on `[data-pressed]`                       |
| `bg-accent-primary`, `text-on-color`                                               | `--color-accent`, `--color-on-accent`                                                                           |
| `text-secondary`, `text-tertiary`                                                  | `--color-text-secondary`, `--color-text-tertiary`                                                               |
| `border-subtle`, `border-strong`                                                   | `--color-border-subtle`, `--color-border-strong`                                                                |
| `shadow-raised-100`, `shadow-overlay-100`                                          | `--shadow-raised`, `--shadow-overlay`                                                                           |
| `h-(--control-height-md)`, `px-(--control-padding-x-md)`, `gap-(--control-gap-md)` | `--control-height-md`, `--control-padding-md`, `--control-gap-md`                                               |
| `text-body-xs-medium`, `text-body-sm-regular`                                      | `--font-size-sm` medium, `--font-size-md` normal                                                                |
| `rounded-md`, `rounded-lg`                                                         | `--radius-md`, `--radius-lg`                                                                                    |
| `duration-150`, `duration-200`, `duration-300`                                     | `--duration-fast`, `--duration-normal`, `--duration-slow`                                                       |
| `data-starting-style:…`, `data-ending-style:…`                                     | `[data-entering]`, `[data-exiting]`                                                                             |
| `origin-(--transform-origin)`                                                      | `transform-origin: var(--trigger-anchor-point)`                                                                 |
| `focus-visible:ring-2 ring-accent-strong ring-offset-1`                            | `[data-focus-visible]` with the outline of `--focus-ring-width`, `--focus-ring-offset` and `--color-focus-ring` |
| `disabled:…`                                                                       | `[data-disabled]`                                                                                               |

Plane is licensed AGPL-3.0: take its values (colors, sizes, timings) and write the CSS module yourself, with no file or class string copied from it.
