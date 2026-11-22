import { useDbClient } from "@tanstack/react-db";
import { useCallback } from "react";

import { showToast } from "@/components/ui/toast";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { apiProblem, isUsernameTaken } from "@/features/identities/utils/api-problem.ts";
import { m } from "@/paraglide/messages.js";

import type { CreateIdentityRequest } from "@/api/gen/client/model/createIdentityRequest.ts";
import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type { InsertIdentityMetadata } from "@/features/identities/api/identities-collection.ts";

// The outcome of a create: the created identity, with the API's id; a taken username, for the form
// to show on its field; or another failure, which a toast has shown already.
export type CreateIdentityResult =
  | { status: "created"; identity: IdentityResponse }
  | { status: "username-taken" }
  | { status: "failed" };

// Creates an identity, for the create page: it inserts it into identitiesCollection, which sends it
// to the API, and resolves once the API has answered. A failure other than a taken username, a
// denied one (403) too, shows a toast.
export function useCreateIdentity() {
  const collection = useDbClient().collection(identitiesCollection);

  return useCallback(
    async (request: CreateIdentityRequest): Promise<CreateIdentityResult> => {
      let created: IdentityResponse | undefined;
      const metadata: InsertIdentityMetadata = {
        onCreated: (identity) => {
          created = identity;
        },
      };
      try {
        // The API gives the identity its id, which replaces this one once it has answered.
        await collection
          .insert({ id: crypto.randomUUID(), ...request }, { metadata })
          .when("settled");
      } catch (error) {
        if (isUsernameTaken(error)) {
          return { status: "username-taken" };
        }
        showToast({
          title:
            apiProblem(error).status === 403
              ? m.identityCreateForbidden()
              : m.identityCreateFailed(),
          variant: "danger",
        });
        return { status: "failed" };
      }
      if (created === undefined) {
        throw new Error("the identities collection didn't report the created identity");
      }
      return { status: "created", identity: created };
    },
    [collection],
  );
}
