import { useDbClient } from "@tanstack/react-db";
import { useCallback } from "react";

import { InlineEditError } from "@/components/ui/inline-edit";
import { showToast } from "@/components/ui/toast";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import {
  apiProblem,
  isUsernameTaken,
  problemTypes,
} from "@/features/identities/utils/api-problem.ts";
import { m } from "@/paraglide/messages.js";

// The fields of an identity that its page edits in place.
export type IdentityField = "username" | "displayName";

// The message on the field for the API's 422: the field's rules, which the form checks too, so the
// API rejects only a value that the form's schema let through.
const invalidMessages = {
  username: m.identityUsernameInvalid,
  displayName: m.identityDisplayNameInvalid,
} satisfies Record<IdentityField, () => string>;

// Saves one field of the identity with this id, for its page's InlineTextFields: it changes the
// field in identitiesCollection, which shows the change at once, the list too, and patches only that
// field. A taken username or a value that the API rejects (422) rejects with InlineEditError, which
// keeps the field in edit mode and shows the message on it; any other failure, a denied change
// (403, fields-denied) too, shows a toast and rejects, so that the field reverts, as the
// collection's change does.
export function useSaveIdentityField(id: string) {
  const collection = useDbClient().collection(identitiesCollection);

  return useCallback(
    async (field: IdentityField, value: string) => {
      const transaction = collection.update(id, (draft) => {
        draft[field] = value;
      });
      try {
        await transaction.when("settled");
      } catch (error) {
        const { status, info } = apiProblem(error);
        if (field === "username" && isUsernameTaken(error)) {
          throw new InlineEditError(m.identityUsernameTaken(), { cause: error });
        }
        if (status === 422 && info?.type === problemTypes.invalidBody) {
          throw new InlineEditError(invalidMessages[field](), { cause: error });
        }
        showToast({
          title: status === 403 ? m.identitySaveForbidden() : m.identitySaveFailed(),
          variant: "danger",
        });
        throw error;
      }
    },
    [collection, id],
  );
}
