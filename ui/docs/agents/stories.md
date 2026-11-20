# Stories (Storybook)

How to write the `*.stories.tsx` files, which show and test the components.

Storybook (`ui/.storybook/`) shows the components of the UI and tests them: `task ui:storybook` serves it on `:6006`, and the `storybook` project of Vitest runs each story as a test, its play function included.
It builds the stories with `vite.config.ts`, as the app, without the app's devtools and proxy.

- A story file sits next to its component, as `<name>.stories.tsx`, and uses CSF Next: `preview.meta({ component })` and `meta.story({ … })`, with `preview` from `#.storybook/preview` (the `imports` of `package.json`, since `src/` imports nothing by a relative path).
  Storybook titles it by its path, as `components/form/text-field`.
- A play function interacts through its context, `async ({ args, canvas, userEvent }) => { … }`, and asserts with `expect` from `storybook/test`; the lint rules of `eslint-plugin-storybook` check both.
  It finds text through its message and an input through its label, as a test does.
- Every story's test runs axe: an accessibility violation fails it (`a11y: { test: "error" }` in `preview.tsx`).
  The tests run every story twice, in the light and the dark scheme: `vitest.config.ts` sets `STORY_COLOR_SCHEME` for the dark run, which `preview.tsx` reads.
- A text prop that the caller passes, such as a field's `label`, is an arg with sample text (`args: { label: "Name" }`), where the app passes a message.
  A play function finds that text through `args.label`, and the text that the component renders itself through its message.
- `preview.tsx` wraps each story in its own clients, as `renderWithClient` does a test, so a component that uses the API's hooks or a collection renders.
  No story has a router yet: a component with a `<Link>` needs a decorator with a memory history first.
- The toolbar's locale (`en`, `de`) sets the messages, zod's messages and `<html lang>`; a story pins one with `globals: { locale: "de" }`.
- The toolbar's color scheme (`light`, `dark`) sets `data-color-scheme` on `<html>`, light whatever the operating system prefers; a story pins one with `globals: { colorScheme: "dark" }`, which holds in both runs of the tests.
- MSW mocks the API in the stories as in the tests, with the generated handlers: `beforeEach: ({ msw }) => { msw.use(getGetGroupMockHandler({ … })); }`.
  They're removed after the story, and a request to `/api/` that no handler takes fails.
  `main.ts` serves MSW's service worker from the installed `msw`.
- Every component gets a docs page with its first story and its props (`tags: ["autodocs"]`, `DocsPage` in `preview.tsx`), so put the story that shows the component best first.
  The page renders that one story alone: the MSW handlers and the locale are global, and the stories of one page would share them.
  The props come from `react-docgen`, since `react-docgen-typescript` needs the JavaScript API that TypeScript 7 lacks.
- `task ui:storybook:build` builds it into `ui/build/storybook`.
  It's not part of the binary: `ui/dist/` holds the app only.
