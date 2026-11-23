import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";

import { getGetGroupQueryKey } from "@/api/gen/client/groups/groups.ts";
import {
  getGetIdentityQueryKey,
  getListIdentitiesQueryKey,
  patchIdentity,
} from "@/api/gen/client/identities/identities.ts";
import { getListPermissionsQueryKey } from "@/api/gen/client/permissions/permissions.ts";
import {
  collectionPath,
  invalidateOnForbidden,
} from "@/features/permissions/utils/invalidate-on-forbidden.ts";
import { createTestClients } from "@/test/clients.ts";
import { server } from "@/test/server.ts";

const id = "00000000-0000-4000-8000-000000000003";

// Answers a patch of an identity with a problem of this status and type.
function patchProblem(status: number, type: string) {
  server.use(
    http.patch("/api/v1/identities/:id", () =>
      HttpResponse.json(
        { type, title: "Problem", status },
        { status, headers: { "Content-Type": "application/problem+json" } },
      ),
    ),
  );
}

// A query client that caches a response for each of these keys, all fresh.
function clientWith(keys: Array<ReadonlyArray<unknown>>) {
  const { queryClient } = createTestClients();
  for (const key of keys) {
    queryClient.setQueryData(key, { data: {} });
  }
  return queryClient;
}

const keys = {
  permissions: getListPermissionsQueryKey("me"),
  ownedPermissions: getListPermissionsQueryKey("me", { owner: "me" }),
  identities: getListIdentitiesQueryKey(),
  identity: getGetIdentityQueryKey(id),
  apiKeys: [`/api/v1/identities/${id}/api-keys`],
  group: getGetGroupQueryKey(id),
};

describe("collectionPath", () => {
  it("leaves out an object's id", () => {
    expect(collectionPath(`/api/v1/identities/${id}`)).toBe("/api/v1/identities");
    expect(collectionPath("/api/v1/identities/me")).toBe("/api/v1/identities");
    expect(collectionPath(`/api/v1/groups/${id}/members/${id}`)).toBe(
      `/api/v1/groups/${id}/members`,
    );
  });

  it("keeps the path of a collection", () => {
    expect(collectionPath("/api/v1/identities")).toBe("/api/v1/identities");
    expect(collectionPath(`/api/v1/identities/${id}/api-keys`)).toBe(
      `/api/v1/identities/${id}/api-keys`,
    );
  });
});

describe("invalidateOnForbidden", () => {
  it("invalidates the permissions and the collection after a denied request", async () => {
    patchProblem(403, "https://pkgsilo/problems/fields-denied");
    const queryClient = clientWith(Object.values(keys));
    const stop = invalidateOnForbidden(queryClient);

    await expect(patchIdentity(id, { displayName: "Grace" })).rejects.toMatchObject({
      status: 403,
    });
    stop();

    const isInvalidated = (key: ReadonlyArray<unknown>) =>
      queryClient.getQueryState(key)?.isInvalidated;
    expect(isInvalidated(keys.permissions)).toBe(true);
    expect(isInvalidated(keys.ownedPermissions)).toBe(true);
    expect(isInvalidated(keys.identities)).toBe(true);
    expect(isInvalidated(keys.identity)).toBe(true);
    // Not the collections below an identity, nor another type's.
    expect(isInvalidated(keys.apiKeys)).toBe(false);
    expect(isInvalidated(keys.group)).toBe(false);
  });

  it("leaves the queries after another error", async () => {
    patchProblem(422, "https://pkgsilo/problems/invalid-body");
    const queryClient = clientWith(Object.values(keys));
    const stop = invalidateOnForbidden(queryClient);

    await expect(patchIdentity(id, { displayName: "" })).rejects.toMatchObject({ status: 422 });
    stop();

    expect(queryClient.getQueryState(keys.permissions)?.isInvalidated).toBe(false);
    expect(queryClient.getQueryState(keys.identities)?.isInvalidated).toBe(false);
  });

  it("stops", async () => {
    patchProblem(403, "about:blank");
    const queryClient = clientWith(Object.values(keys));
    invalidateOnForbidden(queryClient)();

    await expect(patchIdentity(id, { displayName: "Grace" })).rejects.toMatchObject({
      status: 403,
    });

    expect(queryClient.getQueryState(keys.permissions)?.isInvalidated).toBe(false);
  });
});
