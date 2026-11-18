# The web UI

`ui/` is a Vite + React + TypeScript app, managed with pnpm (Node and pnpm are pinned in `mise.toml`).
The Go binary embeds its build and serves it at `/`.

## Build tag

- `ui/embed.go` (`//go:build webui`) embeds `ui/dist`, the output of `task ui:build`.
- Without the tag, `ui/embed_stub.go` embeds `ui/stub/index.html`, a page that says the UI isn't built, and the server logs a warning at startup.
  So `go build`, `go test` and the linters need no Node.
- `task build:prod` builds the UI first and passes `-tags webui`; the CI job `build` runs it.
- `go.mod` ignores `ui/node_modules`, since npm packages may ship `.go` files.

## Development

`task run` serves the API on `:8080`, and `task ui:dev` runs the Vite dev server, which proxies `/api` and `/mcp` to it.
Open the URL that Vite prints.

## Layout

`ui/src/` is cut by feature, like the Go service:

- `app/` assembles the UI, as `app/` does the service: the router (`router.tsx`) and the routes (`routes/`).
- `features/<feature>/` is one slice, like `feature/<feature>/`, such as `features/home/`.
  It holds only the folders it needs: `components/`, `hooks/`, `api/` (query options built from the generated client, for the loaders), `types/` and `utils/`.
- `components/`, `hooks/`, `lib/` and `utils/` hold the code that features share, such as `components/layouts/app-layout.tsx`.
  Create such a folder when its first file comes.
- `api/gen/` is the generated API client, `paraglide/` the compiled messages, and `test/` the test helpers.
- `main.tsx` mounts the app.

Rules:

- Imports go one way: shared code (`api`, `components`, `hooks`, `lib`, `utils`) → `features` → `app`.
  Overrides of `no-restricted-imports` in `.oxlintrc.json` reject an import against that direction.
- A feature doesn't import another feature: a route composes them.
  No lint rule checks that, since oxlint has no `import/no-restricted-paths`.
- No barrel files (`index.ts`): import the file itself, as in `@/features/home/components/home-page.tsx`.
- File names are kebab-case (`unicorn/filename-case`), except the route files, whose names are the URL's syntax.

## Routing

TanStack Router routes by the files in `src/app/routes/`: `__root.tsx` is the layout around every page and the page for a path that no route takes, `index.tsx` is `/`, and `groups/$groupId.tsx` would be `/groups/:groupId`.

- A route file is thin: it maps its URL to a page that a feature exports, with the route's loader and search validation, and has no UI of its own.
- The router generates `src/app/routeTree.gen.ts` from the route files.
  The Vite plugin does so for `task ui:dev`, `task ui:build` and `task ui:test`, and `task ui:routes` (`tsr generate`) for the type check of the build and the lint, which depend on that task.
  Both read their options from `tsr.config.json`.
  The file is ignored by git, as is `.tanstack/`, the generator's temporary folder.
- The router splits each route's component into its own chunk (`autoCodeSplitting`).
- `createAppRouter` in `src/app/router.tsx` creates the router, with the `QueryClient` in the context of every route (`RouterContext` in `__root.tsx`).
  A loader fills the cache of the API's hooks with `context.queryClient.ensureQueryData(…)`, and the page then reads it with the same hook.
  The router preloads a route when the pointer rests on a link to it, and leaves it to TanStack Query whether the cached data is fresh (`defaultPreloadStaleTime: 0`).
- Link with `<Link to="/…">`, which the routes type: a link to a route that doesn't exist fails the type check.
- `core/spa` answers every path without an extension with `index.html`, so a deep link reaches the router.
- Test a route with `renderRoute(path)` from `@/test/render`, which renders the whole app at that path, with a memory history and its own `QueryClient`, and returns the `router` too.
  A route's component loads from its own chunk, so find it with a `findBy` query.
- A test file in `src/app/routes/` isn't a route (`routeFileIgnorePattern` in `tsr.config.json`).

## Devtools

`main.tsx` mounts TanStack Devtools, with the panels of TanStack Query and TanStack Router.
They show on the dev server only: the `devtools()` plugin in `vite.config.ts` strips them from the build.
Add the panel of a new TanStack library there.

## Linting and formatting

`task ui:fmt` formats with oxfmt (`ui/.oxfmtrc.json`), which also sorts the imports and `package.json`.
`task ui:lint` lints with oxlint (`ui/.oxlintrc.json`); `task ui:lint:fix` applies its fixes.
Both cover all of `ui/`.
The prek hooks `oxfmt` and `oxlint` run them on a commit that touches `ui/`, and the CI job `lint` on every push.

- oxlint is type-aware (oxlint-tsgolint) and also reports TypeScript's errors, so a type error fails the lint as well as `task ui:build`.
  Every warning fails it too.
- Import from `src/` with the `@/` alias (`@/app/router.tsx`), not with a relative path.
- Tests get the Testing Library rules (eslint-plugin-testing-library, an ESLint plugin that oxlint runs): await `userEvent` calls and `findBy` queries, and don't await the synchronous ones.
- Disable a rule only with an `oxlint-disable` comment; oxlint ignores `eslint-disable`, and an unused disable comment is an error.

## API client

Orval generates the API client into `ui/src/api/gen/` from `app/testdata/openapi.json`, the OpenAPI document that `app/openapi_test.go` pins (`ui/orval.config.ts`).
`src/api/gen/` is ignored by git, and `task ui:gen:api` generates it; `ui:dev`, `ui:build`, `ui:test` and `ui:lint` depend on that task.

- Call the API through the generated TanStack Query hooks in `src/api/gen/client/<tag>/<tag>.ts`, such as `useGetGroup` and `useCreateGroup`, not with `fetch`.
  `main.tsx` provides their `QueryClient`.
- A hook resolves with `{ data, status, headers }`, where `data` is the response body, so the resource is in `data.data`.
  An error response rejects: the hook's `error` has the HTTP status in `status` and the problem in `info`.
- The types of the bodies are in `src/api/gen/client/model/`.
  `src/api/gen/zod/` has zod schemas of the requests and responses, for validating forms.
  They check the types and the required members, not the rules that only a `jsonschema` description states.
  The client doesn't check the responses with them.
- After a change of the API, `go test ./app -run TestDocs -update` rewrites the document, and `task ui:lint` then reports the code that the change breaks.
  The prek hook `oxlint` runs on a commit that changes the document too.

## Translations

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

## Tests

`task ui:test` runs the tests once, with coverage; `pnpm run test:watch` in `ui/` reruns them on change.
The CI job `ui-test` runs them on every push and uploads the coverage and the JUnit report from `ui/build/` to Codecov.

- Vitest, configured in the `test` section of `vite.config.ts`, runs the tests in jsdom.
  Its globals are off: import `describe`, `it` and `expect` from `vitest`.
- A test sits next to the code it tests, as `<name>.test.tsx`.
- Render with React Testing Library and query by role or label, as a user finds things; interact with `userEvent.setup()`.
  The jest-dom matchers, such as `toBeInTheDocument`, are loaded for every test.
- Find the text that the user reads through its message, as in `getByText(m.welcome())` or `getByRole("link", { name: m.goHome() })`, never as a literal, so that rewording a message breaks no test.
  A test of the locale passes it (`{ locale: "en" }`): without it, the message follows the same locale as the component and can't tell a wrong one.
  No lint rule checks this.
- MSW mocks the API: register the handlers a test needs with `server.use(…)`, `server` from `@/test/server`.
  They're removed after each test.
  Take a handler from the generated `src/api/gen/client/<tag>/<tag>.msw.ts`, such as `getGetGroupMockHandler({ data: { id: "…" } })`, which answers with faker data for the members that the argument leaves out, or pass a function to build the response from the request.
  Write an `http.get("/api/v1/…", …)` handler yourself for an error response.
- Render a component or hook that uses the API's hooks with `renderWithClient` or `renderHookWithClient` from `@/test/render`, which give each test its own `QueryClient` that doesn't retry.
  A request that no handler takes rejects with an MSW error and never reaches the network.
- Vitest undoes `vi.spyOn`, `vi.stubEnv` and `vi.stubGlobal` before each test, so a test doesn't restore them itself.
- `src/test/setup.ts` runs before every test file.
  Coverage leaves out `src/test/`, `src/main.tsx` and the generated code.
- `setup.ts` replaces jsdom's `window.scrollTo`, which only logs that it isn't implemented, since the router restores the scroll position on navigation.

## Serving (`core/spa`)

`app.NewHandler` mounts `spa.Handler(ui.FS(), …)` at `/*`, after `/mcp` and `/api/v1`, so a path under `/api/v1` that no route takes still gets the 404 problem.

- A path that names a file gets it.
  A path without an extension that names none, such as `/identities/42`, is a client-side route and gets `index.html`.
  A missing path with an extension gets the 404 problem, so that a stale tab gets no HTML for an asset of an older build.
- `assets/*` (names hashed by Vite) are cached forever; every other file gets `no-cache`.
- The UI's files get the Content-Security-Policy `default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'`, instead of the API's, which blocks every script.
  So the UI has no inline scripts or styles and loads nothing from other origins.
