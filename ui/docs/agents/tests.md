# Tests

How to write the `*.test.ts(x)` files: the tests of hooks, routes, the client store and plain logic.

`task ui:test` runs the tests and the stories once, with coverage; `task ui:test:watch` reruns them on every change.
The CI job `ui-test` runs them on every push and uploads the coverage and the JUnit report from `ui/build/` to Codecov.

- Vitest, configured in `vitest.config.ts` on top of `vite.config.ts`, has two projects, which add up to one coverage report and one JUnit report: `unit` runs the tests (`*.test.ts`, `*.test.tsx`) in jsdom, and `storybook` runs every story in Chromium (`ui/docs/agents/stories.md`).
- `task ui:browsers`, a dependency of the tasks that run the stories, installs Playwright's Chromium unless it's there.
- Vitest's globals are off: import `describe`, `it` and `expect` from `vitest`.
- A test sits next to the code it tests, as `<name>.test.tsx`.
- Render with React Testing Library and query by role or label, as a user finds things; interact with `userEvent.setup()`.
  The jest-dom matchers, such as `toBeInTheDocument`, are loaded for every test.
- The Testing Library lint rules (eslint-plugin-testing-library, an ESLint plugin that oxlint runs) apply: await `userEvent` calls and `findBy` queries, and don't await the synchronous ones.
- Find the text that the user reads through its message, as in `getByText(m.welcome())` or `getByRole("link", { name: m.goHome() })`, never as a literal, so that rewording a message breaks no test.
  A test of the locale passes it (`{ locale: "en" }`): without it, the message follows the same locale as the component and can't tell a wrong one.
  No lint rule checks this.
- MSW mocks the API: register the handlers a test needs with `server.use(…)`, `server` from `@/test/server`.
  They're removed after each test.
  Take a handler from the generated `src/api/gen/client/<tag>/<tag>.msw.ts`, such as `getGetGroupMockHandler({ data: { id: "…" } })`, which answers with faker data for the members that the argument leaves out, or pass a function to build the response from the request.
  Write an `http.get("/api/v1/…", …)` handler yourself for an error response.
- Render a component or hook that uses the API's hooks or a collection with `renderWithClient` or `renderHookWithClient` from `@/test/render`, which give each test its own `QueryClient` that doesn't retry and its own `DbClient` (`createTestClients` from `@/test/clients`, which each story gets too).
  A request that no handler takes rejects with an MSW error and never reaches the network.
- Vitest undoes `vi.spyOn`, `vi.stubEnv` and `vi.stubGlobal` before each test, so a test doesn't restore them itself.
- `src/test/setup.ts` runs before every test file.
  Coverage leaves out `src/test/`, `src/main.tsx`, the stories and the generated code.
- `setup.ts` replaces jsdom's `window.scrollTo`, which only logs that it isn't implemented, since the router restores the scroll position on navigation.
