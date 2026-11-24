import { queryOptions } from "@tanstack/react-query";

import {
  getListPermissionsQueryKey,
  listPermissions,
} from "@/api/gen/client/permissions/permissions.ts";

import type { listPermissionsResponseSuccess } from "@/api/gen/client/permissions/permissions.ts";

// The identity that owns the resources whose actions a list holds: its ID, or "me" for the
// signed-in identity's own.
export type PermissionsOwner = string;

// What a permissions query holds: the API's answer, or null while the list is unknown, since the
// API failed or didn't answer.
export type PermissionsResponse = listPermissionsResponseSuccess | null;

// The actions that a permissions list allows, or undefined while the list is unknown: then every
// action counts as allowed (allowsAction), since the UI may offer what's then denied but never
// hides what's allowed, and the API still decides every request.
export type AllowedActions = ReadonlyArray<string> | undefined;

// How often an unknown list is loaded again, in milliseconds, while a component shows it.
export const unknownPermissionsRefetchInterval = 10_000;

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
//
// A list that fails to load with a server error, or without an answer, is unknown rather than an
// error, so that no page fails over a hint: it allows every action, and loads again every
// unknownPermissionsRefetchInterval until it's known. Any other error response is an answer: a
// 401 leads to the sign-in page (redirectOnUnauthorized), and a 404 tells an owner that the
// identity may not read.
export function permissionsQueryOptions(owner?: PermissionsOwner) {
  const params = owner === undefined ? undefined : { owner };
  return queryOptions({
    queryKey: getListPermissionsQueryKey("me", params),
    queryFn: async ({ signal }): Promise<PermissionsResponse> => {
      try {
        return await listPermissions("me", params, { signal });
      } catch (error) {
        if (signal.aborted || !leavesUnknown(error)) {
          throw error;
        }
        return null;
      }
    },
    staleTime: (query) => (query.state.data === null ? 0 : Infinity),
    refetchInterval: (query) =>
      query.state.data === null ? unknownPermissionsRefetchInterval : false,
    refetchOnWindowFocus: "always",
    refetchOnReconnect: "always",
    select: allowedActionsOf,
  });
}

// Whether a failed request leaves the list unknown: an error response of the server (5xx), or no
// response at all, which has no status.
function leavesUnknown(error: unknown): boolean {
  const status =
    error instanceof Error && "status" in error && typeof error.status === "number"
      ? error.status
      : undefined;
  return status === undefined || status >= 500;
}

// The actions that a permissions query's data lists, or undefined if the list is unknown.
export function allowedActionsOf(response: PermissionsResponse): AllowedActions {
  return response === null ? undefined : (response.data.data.allowedActions ?? []);
}

// Whether allowedActions allow action: if they list it, or if the list is unknown.
export function allowsAction(allowedActions: AllowedActions, action: string): boolean {
  return allowedActions === undefined || allowedActions.includes(action);
}
