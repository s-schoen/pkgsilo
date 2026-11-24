# Permissions

How the UI hides what the signed-in identity may not do (`ui/src/features/permissions/`, each resource's `use<Resource>Permissions`).

The UI hides what's denied: a button, a menu item or a navigation entry disappears, and a field that may not be edited shows its value as plain text.
Nothing shows disabled with a "no permission" hint.
The answers are hints: the API still enforces every request, and the UI keeps handling its `403` and `fields-denied` (rollback and toast).
Where an answer can't be exact, the UI may offer what's then denied, but never hides what's allowed.

## Where the answers come from

- **The permissions list**, `GET /identities/me/permissions`: every action that the policies may allow on some resource of its type, such as `identity:create:username`.
  It's exact for the creates of a type that isn't owned, and errs toward yes otherwise.
  With `owner`, it lists exactly the actions on the resources that owner owns, such as creating its API keys or changing its password.
- **An object's `allowedActions`**: the actions that the signed-in identity may do to that object, exactly, as the API decided when it answered with it, such as `identity:delete`.
  It leaves out reading the object and its fields (a field that may not be read is absent) and the creates.
  A row without `allowedActions`, as a new one that the identity may not read, allows nothing.

Where an object is in hand, ask its `allowedActions`; ask the list only for what has no object yet, creates and navigation.

## The permissions feature

`features/permissions/`:

- `api/permissions-query.ts`: `permissionsQueryOptions(owner?)`, the list's query.
  `_authenticated`'s `beforeLoad` loads the list without owner together with the session, so no page renders without it.
- `hooks/use-permissions.ts`: `usePermissions(owner?)` returns `can(action)` and the list itself (`allowedActions`), for a shared component that takes it.
  It suspends; a list with owner loads on first use, unless a loader or `requirePermission` loaded it first.
- `utils/allows.ts`: `allows(row, action)`, typed by the row's `allowedActions` union, so a typo in the action fails the type check.
- `types/action.ts`: `Action`, every action that the UI may ask the list about.
- `utils/require-permission.ts`: the route guard (below).
- `utils/invalidate-on-forbidden.ts`: the freshness rule (below).

The resource features import the permissions feature in their permissions hook, the one exception to "a feature imports no other feature".

## A resource's permissions hook

Each resource has one hook, `use<Resource>Permissions`, in its feature's `hooks/`, such as `features/identities/hooks/use-identity-permissions.ts`:

```ts
const { canCreate, of } = useIdentityPermissions();
const { canDelete, canEdit } = of(identity); // canEdit.username, canEdit.displayName
```

- It returns named booleans, and is the only place that holds the resource's action strings: `can` for what has no object, `allows` for an object's.
- It folds in the UI's own business rules, so that the JSX reads `canDelete` whatever the reason: the signed-in identity can't delete itself.
- "May create" means every required create field is allowed; an optional create field without its action hides its input.
- It exports the create actions as a constant too, for the route guard of the "new" page, so that the route and the page agree.
- Unit tests cover each boolean, a row without `allowedActions` and each business rule.

The lint rule `pkgsilo/no-permission-checks-in-jsx` (`ui/lint/pkgsilo-plugin.ts`) forbids calling `can` or `allows` anywhere inside JSX, callbacks included: compute the booleans in the hook, then pass them.

## The components' props

Shared components take the decision as a prop instead of being wrapped in a condition:

- `NavItem` takes `action`, and hides when `NavList`'s `allowedActions` (from `usePermissions()`) don't list it; a `NavSection` without a visible item hides.
  `NavSection` inspects its children, so pass the `NavItem`s themselves, not wrappers.
- `InlineEdit`, `InlineTextField` and the form's `InlineTextField` take `isReadOnly`: the value as plain text, without the edit mode.
- `DataTable` takes `canDelete(row)`: Delete shows while one of the selected rows qualifies, and deletes only those.
- `ActionMenu` items and `PageHeader` actions take `isVisible`; a menu without a visible item doesn't render its trigger, and a header without a visible action has no actions container.
  Hide the header action that holds a menu with the same booleans, so that no empty place stays.
- An empty state's button renders only with the permission: `{canCreate && <Button …/>}`, a boolean from the hook, not a call.

## Route guards

A route that the identity may not use renders `NotFound` in the app's frame, as an object it may not read does:

```ts
beforeLoad: async ({ context }) => {
  await requirePermission(context.queryClient, identityCreateActions);
},
notFoundComponent: NotFound,
```

`requirePermission(queryClient, action | actions, owner?)` throws `notFound()` unless the list holds every action.
The route's own `notFoundComponent` keeps the frame (`ui/docs/agents/routing.md`).
An object's page needs no guard: an object that isn't there, since the identity may not read it, already renders `NotFound`.

## Freshness

- The list refetches on every window focus and reconnect, and stays fresh otherwise, so that a component that mounts later reads the list the route loaded.
- Any `403`, `fields-denied` included, invalidates every permissions query and the affected collection, in one place for every request (`invalidateOnForbidden`, which `AuthenticatedLayout` subscribes to through `useInvalidateOnForbidden`): don't handle that per feature.
- An object's `allowedActions` refresh with its collection.
- The slices of policies, groups and group members invalidate the permissions queries (`permissionsQueryKeyPrefix`) after the signed-in identity's own writes there, since those writes change what it may do.
- Signing out removes every query, the permissions too.

## Tests

- `signedIn()` from `@/test/session` may do everything.
  A test of what a page hides replaces the list in a later `server.use(permissions([…]))`, since MSW takes the handler registered last first, and gives the rows the `allowedActions` of that actor.
- Route tests cover the setups of the mock personas: everything allowed, read-only, and self-service, where the list allows more than one object does.
- `task ui:dev:mock` signs in as `admin`; sign in as `reader` or `self` to click through with fewer permissions (`src/mocks/personas.ts`).
