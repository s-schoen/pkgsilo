import { useCallback } from "react";

import { useGetCurrentSession } from "@/api/gen/client/sessions/sessions.ts";
import { usePermissions } from "@/features/permissions/hooks/use-permissions.ts";
import { allows } from "@/features/permissions/utils/allows.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type { Action } from "@/features/permissions/types/action.ts";

// The creates of the fields that a new identity needs: creating one needs them all. The route of
// /identities/new guards with them too (requirePermission).
export const identityCreateActions = [
  "identity:create:username",
  "identity:create:displayName",
] as const satisfies ReadonlyArray<Action>;

// What the signed-in identity may do to one identity.
export type IdentityPermissions = {
  canDelete: boolean;
  canEdit: { username: boolean; displayName: boolean };
};

// What the signed-in identity may do with identities, as named booleans for the JSX: canCreate, and
// of(identity) for one identity, from its allowedActions. It's the only place with the identity
// action strings, and it folds in the UI's own rule: the signed-in identity can't delete itself,
// and nothing is deleted until the session tells which one that is.
export function useIdentityPermissions() {
  const { can } = usePermissions();
  const currentId = useGetCurrentSession().data?.data.data.identityId;

  const canCreate = identityCreateActions.every((action) => can(action));
  const of = useCallback(
    (identity: IdentityResponse): IdentityPermissions => ({
      canDelete:
        allows(identity, "identity:delete") && currentId !== undefined && identity.id !== currentId,
      canEdit: {
        username: allows(identity, "identity:update:username"),
        displayName: allows(identity, "identity:update:displayName"),
      },
    }),
    [currentId],
  );

  return { canCreate, of };
}
