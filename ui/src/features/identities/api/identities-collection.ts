import { queryCollectionOptions } from "@tanstack/query-db-collection";
import { collectionOptions } from "@tanstack/react-db";

import {
  createIdentity,
  deleteIdentity,
  getListIdentitiesQueryKey,
  listIdentities,
  patchIdentity,
} from "@/api/gen/client/identities/identities.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type { QueryClient } from "@tanstack/react-query";

// The most identities that a page of the API's list holds.
const pageSize = 100;

// Every identity that the actor may read, with the fields it may read: the others are absent, as
// the API leaves them out. The pages' loaders preload it; the pages read it with useLiveQuery and
// change it through useDbClient().collection(identitiesCollection). A change shows at once and is
// sent to the API; once the API answers, its answer replaces the change, and if it fails, the
// change rolls back and the caller's transaction rejects with the API's error.
export const identitiesCollection = collectionOptions("identities", (client) =>
  queryCollectionOptions({
    id: "identities",
    queryClient: client.requireDependency<QueryClient>("queryClient"),
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
      await collection.utils.writeInsert(created);
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
    },
    onDelete: async ({ transaction, collection }) => {
      const keys = transaction.mutations.map(({ key }) => String(key));
      await Promise.all(keys.map(async (id) => deleteIdentity(id)));
      await collection.utils.writeDelete(keys);
    },
  }),
);
