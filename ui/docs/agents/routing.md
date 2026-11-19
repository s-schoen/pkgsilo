# Routing

How the UI's routes are built, loaded and tested (`ui/src/app/`).

TanStack Router routes by the files in `src/app/routes/`: `__root.tsx` is the layout around every page and the page for a path that no route takes, `index.tsx` is `/`, and `groups/$groupId.tsx` would be `/groups/:groupId`.

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
- Link with `<Link to="/…">`, which the routes type: a link to a route that doesn't exist fails the type check.
- `core/spa` answers a browser's navigation to every path that names no file with `index.html`, so a deep link reaches the router.
- A stale tab whose route chunk is gone after a deploy reloads the page once when it renders that route, so that it loads the new build (`lazyRouteComponent` of the router, which the code splitting uses).
- Test a route with `renderRoute(path)` from `@/test/render`, which renders the whole app at that path, with a memory history and its own `QueryClient` and `DbClient`, and returns the `router` too.
  A route's component loads from its own chunk, so find it with a `findBy` query.
- A test file in `src/app/routes/` isn't a route (`routeFileIgnorePattern` in `tsr.config.json`).
