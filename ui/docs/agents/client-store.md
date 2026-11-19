# Client store (TanStack DB)

How the UI keeps the API's resources in collections on the client.

TanStack DB (`@tanstack/react-db`, a 1.0 release candidate) keeps the API's resources in collections on the client, which live queries read and join, and which apply a change at once (optimistically) while it's sent to the API.
Use it for the resources that a page lists, filters or changes; a single read or a call such as a login stays with the API's hooks.

- `createClients()` in `src/app/clients.ts` creates a `DbClient` over a `QueryClient`; `AppProviders` (`src/app/app-providers.tsx`) provides both, and `main.tsx` passes them to the router's context too.
- A collection is a descriptor that `collectionOptions` creates, with `queryCollectionOptions` from `@tanstack/query-db-collection`, so that it loads through TanStack Query.
  It goes in `features/<feature>/api/<name>-collection.ts`, or `src/api/collections/` once several features share it.
  Its `queryFn` and its handlers call the functions of the generated client, not `fetch`:

  ```ts
  export const groupsCollection = collectionOptions("groups", (client) =>
    queryCollectionOptions({
      id: "groups",
      queryClient: client.requireDependency<QueryClient>("queryClient"),
      queryKey: getListGroupsQueryKey(),
      queryFn: async ({ signal }) => {
        /* listGroups({ offset, limit: 100 }, { signal }) until nextOffset is null */
      },
      getKey: (group) => group.id,
      onInsert: async ({ transaction, collection }) => {
        await Promise.all(transaction.mutations.map((m) => createGroup({ name: m.modified.name })));
        await collection.utils.refetch();
      },
      // onUpdate with patchGroup, onDelete with deleteGroup, the same way.
    }),
  );
  ```

- A collection's `queryFn` loads every page: the lists of the API return at most 100 resources per request, and the collection's queries see only what it has loaded.
- A loader preloads the collection (`ui/docs/agents/routing.md`); a component reads it with `useLiveQuery((q) => q.from({ group: groupsCollection }))` and changes it with `useDbClient().collection(groupsCollection).insert(…)`, `.update(…)` or `.delete(…)`.
  When a handler rejects, the collection rolls the change back.
- Test with `renderWithClient`, `renderHookWithClient` or `renderRoute`, which give each test its own `DbClient`, and mock the requests with MSW as for the API's hooks.
- TanStack DB holds most of the entry chunk's size (about 290 kB minified, 83 kB gzipped), since `main.tsx` creates the `DbClient`.
