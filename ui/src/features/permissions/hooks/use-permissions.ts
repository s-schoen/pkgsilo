import { useSuspenseQuery } from "@tanstack/react-query";
import { useCallback } from "react";

import { permissionsQueryOptions } from "@/features/permissions/api/permissions-query.ts";

import type { PermissionsOwner } from "@/features/permissions/api/permissions-query.ts";
import type { Action } from "@/features/permissions/types/action.ts";

// The signed-in identity's permissions: can(action) tells whether the policies may allow action on
// some resource of its type, or, with owner, exactly on the resources that owner owns. It's a hint
// that errs only toward yes: an object's own allowedActions (allows) tell exactly, and the API
// still denies what isn't allowed.
//
// Call it in a resource's permissions hook (use<Resource>Permissions), which gives the JSX named
// booleans; a lint rule keeps can() out of JSX. allowedActions is the list itself, for a shared
// component that takes it, such as the NavList. The authenticated route loads the list without
// owner before any page renders; a list with owner suspends until it loaded, unless the route's
// loader or requirePermission loaded it first.
export function usePermissions(owner?: PermissionsOwner) {
  const { data: allowedActions } = useSuspenseQuery(permissionsQueryOptions(owner));
  const can = useCallback((action: Action) => allowedActions.includes(action), [allowedActions]);
  return { can, allowedActions };
}
