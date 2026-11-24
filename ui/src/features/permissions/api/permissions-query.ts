import { queryOptions } from "@tanstack/react-query";

import {
  getListPermissionsQueryKey,
  listPermissions,
} from "@/api/gen/client/permissions/permissions.ts";

import type { listPermissionsResponseSuccess } from "@/api/gen/client/permissions/permissions.ts";

// The identity that owns the resources whose actions a list holds: its ID, or "me" for the
// signed-in identity's own.
export type PermissionsOwner = string;

// The prefix of every permissions query's key, with and without owner: invalidating it refreshes
// them all.
export const permissionsQueryKeyPrefix = getListPermissionsQueryKey("me");

// The query of the signed-in identity's permissions (GET /identities/me/permissions). Without owner,
// it lists every action that may be allowed on some resource of its type; with owner, exactly the
// actions on the resources that owner owns, such as creating its API keys. The authenticated route
// loads the list without owner before any page renders. It refetches on every focus of the window
// and every reconnect, however fresh ("always"), as a policy may have changed meanwhile. A denied
// request refetches it too (invalidateOnForbidden). Nothing else refetches it: it stays fresh
// otherwise, so that a component that mounts later, such as the navigation, reads the list that the
// route loaded. Built with queryOptions instead of the generated query options, whose queryFn
// useSuspenseQuery doesn't take.
export function permissionsQueryOptions(owner?: PermissionsOwner) {
  const params = owner === undefined ? undefined : { owner };
  return queryOptions({
    queryKey: getListPermissionsQueryKey("me", params),
    queryFn: ({ signal }) => listPermissions("me", params, { signal }),
    staleTime: Infinity,
    refetchOnWindowFocus: "always",
    refetchOnReconnect: "always",
    select: allowedActionsOf,
  });
}

// The actions that a response of the permissions endpoint lists.
export function allowedActionsOf(response: listPermissionsResponseSuccess): Array<string> {
  return response.data.data.allowedActions ?? [];
}
