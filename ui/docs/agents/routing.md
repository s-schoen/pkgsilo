# Routing

How the UI's routes are built, loaded and tested (`ui/src/app/`).

TanStack Router routes by the files in `src/app/routes/`: `__root.tsx` is the layout around every page, `signin.tsx` is the sign-in page at `/signin`, and `_authenticated.tsx` is the pathless route of every page that needs a session, in the app's frame: `_authenticated/index.tsx` is `/`, `_authenticated/$.tsx` the page for a path that no route takes, and `_authenticated/groups/$groupId.tsx` would be `/groups/:groupId`.

## Sessions

- A page that needs a session, which is every page but the sign-in page, lives below `routes/_authenticated/`.
  Its `beforeLoad` loads the current session, and without one leads to `/signin?redirect=<the page's URL>`, so that a sign-in leads back; `safeRedirectTarget` (`features/session/utils/redirect-target.ts`) drops a redirect that isn't a path of the app.
- Once a query or a mutation fails with 401 on a page, as when the session ends, `redirectOnUnauthorized`, which `createAppRouter` starts, leads to the sign-in page the same way and removes every cached response.
  A failed sign-in's 401 is the form's to show.
- The session's cookie lasts until the session ends at the latest, so a sign-in outlasts a restart of the browser.
  `useSessionExpiry` checks the session once that time comes; a session that ends earlier, since it was idle, is noticed by the next request, such as the check when the tab gets the focus.
- A query doesn't retry a 4xx response (`createClients`), so that a 401 leads on at once.
- A route file is thin: it maps its URL to a page that a feature exports, with the route's loader and search validation, and has no UI of its own.
- The router generates `src/app/routeTree.gen.ts` from the route files.
  The Vite plugin does so for `task ui:dev`, `task ui:build` and `task ui:test`, and `task ui:routes` (`tsr generate`) for the type check of the build and the lint, which depend on that task.
  Both read their options from `tsr.config.json`.
  The file is ignored by git, as is `.tanstack/`, the generator's temporary folder.
- The router splits each route's component into its own chunk (`autoCodeSplitting`).
- `createAppRouter` in `src/app/router.tsx` creates the router, with the `QueryClient` and the `DbClient` in the context of every route (`RouterContext` in `__root.tsx`).
  A loader fills the cache of the API's hooks with `context.queryClient.ensureQueryData(…)`, and the page then reads it with the same hook.
  A loader loads a collection with `await context.dbClient.collection(groupsCollection).preload()`, and the page then reads it with `useLiveQuery` (`ui/docs/agents/client-store.md`).
  The router preloads a route when the pointer rests on a link to it, and leaves it to TanStack Query whether the cached data is fresh (`defaultPreloadStaleTime: 0`).
  A React Aria link preloads only with `useRoutePreload(href)` (`ui/docs/agents/components.md`).
- Link with `<Link to="/…">`, which the routes type: a link to a route that doesn't exist fails the type check.
- `core/spa` answers a browser's navigation to every path that names no file with `index.html`, so a deep link reaches the router.
- A stale tab whose route chunk is gone after a deploy reloads the page once when it renders that route, so that it loads the new build (`lazyRouteComponent` of the router, which the code splitting uses).
- Test a route with `renderRoute(path)` from `@/test/render`, which renders the whole app at that path, with a memory history and its own `QueryClient` and `DbClient`, and returns the `router` too.
  A page below `_authenticated` needs a session first: `server.use(...signedIn())`, or `signedOut()`, from `@/test/session`.
  A route's component loads from its own chunk, so find it with a `findBy` query.
- A test file in `src/app/routes/` isn't a route (`routeFileIgnorePattern` in `tsr.config.json`).
