import { useDbClient } from "@tanstack/react-db";
import { useCallback } from "react";

import { showToast } from "@/components/ui/toast";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { apiProblem } from "@/features/identities/utils/api-problem.ts";
import { identityName } from "@/features/identities/utils/identity-name.ts";
import { m } from "@/paraglide/messages.js";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

// The outcome of a delete: how many of the identities the API deleted, of how many.
export type DeleteIdentitiesResult = { deleted: number; count: number };

// Deletes identities, for the list and an identity's page: it deletes each one from
// identitiesCollection in a transaction of its own, which removes it at once and sends the delete to
// the API, and resolves once the API has answered for every one. A failed delete, a denied one (403)
// too, rolls that identity back alone. A toast reports the outcome: the deleted one by its name or
// the count of them, "Deleted n of m" when some failed, or the failure when none was deleted.
export function useDeleteIdentities() {
  const collection = useDbClient().collection(identitiesCollection);

  return useCallback(
    async (identities: Array<IdentityResponse>): Promise<DeleteIdentitiesResult> => {
      const results = await Promise.allSettled(
        identities.map(async ({ id }) => collection.delete(id).when("settled")),
      );
      const failures = results.flatMap((result) =>
        result.status === "rejected" ? [result.reason] : [],
      );
      const count = identities.length;
      const deleted = count - failures.length;
      if (failures.length === 0) {
        showToast({
          title:
            count === 1
              ? m.identityDeleted({ name: identityName(identities[0]) })
              : m.identitiesDeleted({ count }),
          variant: "success",
        });
      } else if (deleted > 0) {
        showToast({ title: m.identitiesDeletedPartly({ deleted, count }), variant: "danger" });
      } else {
        const isForbidden = failures.every((error) => apiProblem(error).status === 403);
        showToast({
          title: isForbidden
            ? m.identitiesDeleteForbidden({ count })
            : m.identitiesDeleteFailed({ count }),
          variant: "danger",
        });
      }
      return { deleted, count };
    },
    [collection],
  );
}
