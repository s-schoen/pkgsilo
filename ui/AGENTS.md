# AGENTS.md: the web UI

`ui/` is pkgsilo's web UI: a Vite + React + TypeScript app, managed with pnpm (Node and pnpm are pinned in `mise.toml`).
The Go binary embeds its build and serves it at `/` (`docs/agents/ui.md`).

## Stack

Versions are in `package.json`.

- React, TypeScript and Vite.
- React Aria Components as the base of the components, styled with CSS modules, which Vite compiles with Lightning CSS; Tabler's icons (`@tabler/icons-react`).
- TanStack Router (routes by file), Query (the API's data), DB (collections on the client) and Form.
  TanStack Devtools show their panels on the dev server only; add the panel of a new TanStack library in `main.tsx`.
- zod v4 for validation, imported from `zod/v4`.
- Orval generates the API client, its zod schemas and its MSW handlers from the OpenAPI document.
- Paraglide JS compiles the messages: English (`en`) is the base locale, German (`de`) the other one.
- Vitest (`vitest.config.ts`) with two projects: `unit` runs the tests in jsdom, `storybook` runs the stories in Chromium.
  Testing Library and MSW in both.
- Storybook with CSF Next and the addons for docs, a11y, Vitest and MCP.
- oxlint (type-aware, so it reports TypeScript's errors too) and oxfmt.

## Commands

Run the tasks from the repo root: each one generates the files it needs first, which a bare `pnpm` script skips.

- `task ui:storybook`: Storybook on `:6006` and its MCP server.
  Start it in the background before you work on components.
- `task ui:dev`: the Vite dev server, which proxies `/api/…` and `/mcp` to `task run` on `:8080`.
  Open the URL that Vite prints.
- `task ui:test`: every test and story, with coverage, as CI runs them.
  `task ui:test:unit` runs the tests alone, without a browser; `task ui:test:watch` reruns them all on every change.
- `task ui:lint`: oxlint with the type check; every warning fails.
  `task ui:lint:fix` applies its fixes.
- `task ui:fmt`: oxfmt, which also sorts the imports and `package.json`.
- `task ui:gen:api`: the API client, after `go test ./app -run TestDocs -update` rewrote the OpenAPI document.
- `task ui:build`: the production build in `ui/dist/`, which `task build:prod` embeds.

The prek hooks `oxfmt` and `oxlint` run on a commit that touches `ui/`.

## Layout and rules

`ui/src/` is cut by feature, like the Go service:

- `app/` assembles the UI: the router (`router.tsx`), the routes (`routes/`) and the clients of the router's context (`clients.ts`).
- `features/<feature>/` is one slice, with only the folders it needs: `components/`, `hooks/`, `api/` (query options and collections for the loaders), `types/`, `utils/`.
- `components/`, `hooks/`, `lib/` and `utils/` hold the code that features share; create such a folder when its first file comes.
  `components/ui/` is the UI library, which every view builds on.
- `styles/` holds the global CSS: the tokens, the base of the page and the breakpoints.
- `test/` holds the test helpers; `main.tsx` mounts the app.

Rules:

- Imports go one way: shared code (`api`, `components`, `hooks`, `lib`, `utils`) → `features` → `app`; `no-restricted-imports` in `.oxlintrc.json` checks it.
  A route composes features: a feature imports no other feature, which no lint rule checks.
- Import the file itself through the `@/` alias, as in `@/features/home/components/home-page.tsx`.
  A component of the UI library is the exception: import it by its folder, as `@/components/ui/button`, whose `index.ts` is its public API; `no-restricted-imports` checks it.
- File names are kebab-case, except the route files, whose names are the URL's syntax.
- Every text the user reads is a message (`m.welcome()`), and tests and stories find it through the message (`ui/docs/agents/translations.md`).
- The UI's Content-Security-Policy allows only its own files: scripts and styles live in files, and everything loads from the UI's origin (images may also be `data:` URLs).
  It forbids the code that zod compiles, so zod parses `jitless`: `@/lib/zod-jitless.ts` stays the first import of `main.tsx`, `src/test/setup.ts` and `.storybook/preview.tsx`, since zod reads the setting when it creates a schema, and no lint rule checks the order.
- Disable a lint rule with an `oxlint-disable` comment (oxlint ignores `eslint-disable`); an unused one is an error; Always add a comment on why you disable a rule

## Generated files

Change the source; the tasks regenerate the output, which git ignores.

| Output                     | Source                                          |
| -------------------------- | ----------------------------------------------- |
| `src/api/gen/`             | `app/testdata/openapi.json` (`task ui:gen:api`) |
| `src/paraglide/`           | `messages/<locale>.json`                        |
| `src/app/routeTree.gen.ts` | the route files in `src/app/routes/`            |

## Testing strategy

- A component is tested by its stories: a story for each state it reaches, a play function for each interaction, and axe on every story, whose violations fail it.
- A hook, a route, the client store and plain logic are tested by a `*.test.ts(x)` next to the code.
- Both mock the API with the generated MSW handlers.

## Storybook MCP

`.mcp.json` registers the `storybook` server at `http://localhost:6006/mcp`, which `task ui:storybook` serves.
Agents connect to it when the session starts: when its tools are missing, start Storybook and ask the user to reconnect with `/mcp`.

Follow the server's workflow.
Its guidance is generic, and this repo differs from it:

- Stories use CSF Next: `preview.meta({ component })` and `meta.story({ … })`, with `preview` from `#.storybook/preview`.
- The API is mocked with MSW, in `beforeEach: ({ msw }) => { msw.use(…); }`; there are no module mocks.
- A story pins a locale with `globals: { locale: "de" }`.
- `test-run` runs the stories while you iterate; `task ui:test` is the final pass, since it runs the tests too.

## Done

A UI change is done when:

- `task ui:lint` and `task ui:test` pass.
- The stories of every changed component cover its states.
- Every new message is in each locale's file.
- Your final reply lists the links from `stories-preview`.

## Topic docs

Read before you work on:

- **Components and styling** (`src/components/ui/`, `*.module.css`, `src/styles/`, icons): `ui/docs/agents/components.md`.
- **Routes** (`src/app/routes/`, `router.tsx`, loaders, links): `ui/docs/agents/routing.md`.
- **API calls** (the generated client and its hooks): `ui/docs/agents/api-client.md`.
- **Collections** (TanStack DB, `useLiveQuery`): `ui/docs/agents/client-store.md`.
- **Forms** (`useAppForm`, field components, zod schemas): `ui/docs/agents/forms.md`.
- **Messages** (`messages/`, locales): `ui/docs/agents/translations.md`.
- **Tests** (`*.test.ts(x)`, MSW, `@/test/render`): `ui/docs/agents/tests.md`.
- **Stories** (`*.stories.tsx`, `.storybook/`): `ui/docs/agents/stories.md`.
