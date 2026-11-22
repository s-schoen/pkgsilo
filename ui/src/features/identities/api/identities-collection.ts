import {
  DeleteOperationItemNotFoundError,
  queryCollectionOptions,
} from "@tanstack/query-db-collection";
import { collectionOptions } from "@tanstack/react-db";

import {
  createIdentity,
  deleteIdentity,
  getGetIdentityQueryKey,
  getListIdentitiesQueryKey,
  listIdentities,
  patchIdentity,
} from "@/api/gen/client/identities/identities.ts";
import { isIdentityNotFound } from "@/features/identities/utils/api-problem.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type { QueryClient } from "@tanstack/react-query";

// The metadata of an insert into identitiesCollection, which learns the API's identity: the
// optimistic row has an id of the client's, and onCreated gets the created identity with its own,
// as a create page needs to show it.
export type InsertIdentityMetadata = {
  onCreated: (identity: IdentityResponse) => void;
};

function isInsertIdentityMetadata(metadata: unknown): metadata is InsertIdentityMetadata {
  return (
    typeof metadata === "object" &&
    metadata !== null &&
    "onCreated" in metadata &&
    typeof metadata.onCreated === "function"
  );
}

// The most identities that a page of the API's list holds.
const pageSize = 100;

// Every identity that the actor may read, with the fields it may read: the others are absent, as
// the API leaves them out. The pages' loaders preload it; the pages read it with useLiveQuery and
// change it through useDbClient().collection(identitiesCollection). A change shows at once and is
// sent to the API; once the API answers, its answer replaces the change, and if it fails, the
// change rolls back and the caller's transaction rejects with the API's error.
//
// A refetch, as on the window's focus, may bring the API's change before its answer does: the
// answer then applies over it.
export const identitiesCollection = collectionOptions("identities", (client) => {
  const queryClient = client.requireDependency<QueryClient>("queryClient");

  return queryCollectionOptions({
    id: "identities",
    queryClient,
    queryKey: getListIdentitiesQueryKey(),
    // Every page, since the collection's queries see only what it has loaded.
    queryFn: async ({ signal }) => {
      const identities: Array<IdentityResponse> = [];
      let offset: number | null = 0;
      while (offset !== null) {
        const page = await listIdentities({ offset, limit: pageSize }, { signal });
        identities.push(...page.data.data);
        offset = page.data.pagination.nextOffset;
      }
      return identities;
    },
    getKey: (identity) => identity.id,
    onInsert: async ({ transaction, collection }) => {
      const created = await Promise.all(
        transaction.mutations.map(async ({ modified }) => {
          const response = await createIdentity({
            username: modified.username ?? "",
            displayName: modified.displayName ?? "",
          });
          return response.data.data;
        }),
      );
      // The optimistic row has an id of the client's: the API's row, with its own id, replaces it.
      await collection.utils.writeUpsert(created);
      transaction.mutations.forEach(({ metadata }, index) => {
        // Promise.all keeps the order: the created identity of each mutation is at its index.
        if (isInsertIdentityMetadata(metadata)) {
          metadata.onCreated(created[index]);
        }
      });
    },
    // Patches only the fields that changed, so that a field the actor may not write, or another
    // user changed meanwhile, stays as it is.
    onUpdate: async ({ transaction, collection }) => {
      const updated = await Promise.all(
        transaction.mutations.map(async ({ key, changes }) => {
          const response = await patchIdentity(String(key), {
            username: changes.username,
            displayName: changes.displayName,
          });
          return response.data.data;
        }),
      );
      await collection.utils.writeUpsert(updated);
      // A single identity's query, such as the account menu's of the signed-in one, loads the
      // change too.
      for (const { id } of updated) {
        void queryClient.invalidateQueries({ queryKey: getGetIdentityQueryKey(id) });
      }
    },
    onDelete: async ({ transaction, collection }) => {
      const keys = transaction.mutations.map(({ key }) => String(key));
      await Promise.all(
        keys.map(async (id) => {
          try {
            await deleteIdentity(id);
          } catch (error) {
            // Deleted already, or hidden from the actor since: either way, it leaves the list.
            if (!isIdentityNotFound(error)) {
              throw error;
            }
          }
        }),
      );
      await Promise.all(
        keys.map(async (id) => {
          try {
            await collection.utils.writeDelete(id);
          } catch (error) {
            // A refetch removed it first.
            if (!(error instanceof DeleteOperationItemNotFoundError)) {
              throw error;
            }
          }
        }),
      );
    },
  });
});
