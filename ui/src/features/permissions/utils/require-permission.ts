import { notFound } from "@tanstack/react-router";

import {
  allowedActionsOf,
  permissionsQueryOptions,
} from "@/features/permissions/api/permissions-query.ts";

import type { PermissionsOwner } from "@/features/permissions/api/permissions-query.ts";
import type { Action } from "@/features/permissions/types/action.ts";
import type { QueryClient } from "@tanstack/react-query";

// For a route's beforeLoad: renders the page that doesn't exist (NotFound) unless the signed-in
// identity's permissions list action, as the API answers an object that it may not read. With
// owner, the list of the actions on owner's resources, which it loads first:
//
//   beforeLoad: ({ context }) => requirePermission(context.queryClient, "identity:create:username"),
export async function requirePermission(
  queryClient: QueryClient,
  action: Action | ReadonlyArray<Action>,
  owner?: PermissionsOwner,
): Promise<void> {
  const allowedActions = allowedActionsOf(
    await queryClient.ensureQueryData(permissionsQueryOptions(owner)),
  );
  const required: ReadonlyArray<Action> = typeof action === "string" ? [action] : action;
  if (!required.every((each) => allowedActions.includes(each))) {
    throw notFound();
  }
}
