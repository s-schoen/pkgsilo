import { beforeEach, describe, expect, it } from "vitest";

import {
  createIdentity,
  deleteIdentity,
  getIdentity,
  listIdentities,
  patchIdentity,
  replaceIdentity,
} from "@/api/gen/client/identities/identities.ts";
import { adminId, fixedIdentities, readerId, selfId } from "@/mocks/identities-data.ts";
import { createIdentitiesHandlers } from "@/mocks/identities-handlers.ts";
import { createIdentitiesStore } from "@/mocks/identities-store.ts";
import { server } from "@/test/server.ts";

import type { Actor } from "@/mocks/personas.ts";

const admin: Actor = { id: adminId, persona: "admin" };
const reader: Actor = { id: readerId, persona: "reader" };
const self: Actor = { id: selfId, persona: "self" };

// Serves the fixed identities to actor.
function serveAs(actor: Actor) {
  server.use(
    ...createIdentitiesHandlers(createIdentitiesStore({ actorId: () => actor.id }), () => actor),
  );
}

const everyIdentityAction = [
  "identity:delete",
  "identity:update:username",
  "identity:update:displayName",
];

// The error that the generated client rejects with: the response's status and its problem.
async function errorOf(promise: Promise<unknown>) {
  try {
    await promise;
  } catch (error) {
    return error as { status?: number; info?: Record<string, unknown> };
  }
  throw new Error("expected the request to fail");
}

// The handlers over a store of the fixed identities, through the generated client, as the UI calls
// them.
describe("createIdentitiesHandlers", () => {
  beforeEach(() => {
    serveAs(admin);
  });

  it("lists a page in the collection envelope, with the actions that the admin may do", async () => {
    const response = await listIdentities({ offset: 0, limit: 5 });

    expect(response.status).toBe(200);
    expect(response.data.data).toEqual(
      fixedIdentities
        .slice(0, 5)
        .map((identity) => ({ ...identity, allowedActions: everyIdentityAction })),
    );
    expect(response.data.pagination).toEqual({ offset: 0, limit: 5, nextOffset: 5 });
  });

  it("lists the first 50 without a page, and every one in pages of 100", async () => {
    const first = await listIdentities();
    const all = await listIdentities({ offset: 0, limit: 100 });

    expect(first.data.pagination).toMatchObject({ offset: 0, limit: 50 });
    expect(all.data.data).toHaveLength(fixedIdentities.length);
    expect(all.data.pagination.nextOffset).toBeNull();
  });

  it("answers an invalid page with 400", async () => {
    const error = await errorOf(listIdentities({ offset: -1, limit: 101 }));

    expect(error.status).toBe(400);
    expect(error.info).toMatchObject({
      type: "https://pkgsilo/problems/invalid-query",
      errors: [
        { parameter: "offset", detail: "must be no less than 0" },
        { parameter: "limit", detail: "must be no greater than 100" },
      ],
    });
  });

  it("creates an identity with its Location, which later requests find", async () => {
    const created = await createIdentity({ username: "new.user", displayName: "New User" });

    expect(created.status).toBe(201);
    const { id } = created.data.data;
    expect(created.headers.get("Location")).toBe(`/api/v1/identities/${id}`);
    const fetched = await getIdentity(id);
    expect(fetched.data.data).toMatchObject({
      username: "new.user",
      displayName: "New User",
      createdBy: adminId,
    });
  });

  it("answers a taken username with the problem 409", async () => {
    const error = await errorOf(createIdentity({ username: "admin", displayName: "Again" }));

    expect(error.status).toBe(409);
    expect(error.info).toEqual({
      type: "https://pkgsilo/problems/username-taken",
      title: "Username taken",
      status: 409,
      detail: "The username is already taken.",
    });
  });

  it("answers an invalid body with 422", async () => {
    const error = await errorOf(patchIdentity(adminId, { displayName: "" }));

    expect(error.status).toBe(422);
    expect(error.info).toMatchObject({
      type: "https://pkgsilo/problems/invalid-body",
      errors: [{ pointer: "/displayName" }],
    });
  });

  it("answers a body that isn't a JSON object with 400", async () => {
    const response = await fetch("/api/v1/identities", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "[]",
    });

    expect(response.status).toBe(400);
  });

  it("patches and replaces an identity", async () => {
    const patched = await patchIdentity(adminId, { displayName: "Root" });
    const replaced = await replaceIdentity(adminId, { username: "root", displayName: "Root" });

    expect(patched.data.data).toMatchObject({ username: "admin", displayName: "Root" });
    expect(replaced.data.data).toMatchObject({ username: "root", displayName: "Root" });
    expect((await getIdentity(adminId)).data.data.username).toBe("root");
  });

  it("deletes an identity, which then answers 404", async () => {
    const deleted = await deleteIdentity(adminId);

    expect(deleted.status).toBe(204);
    const error = await errorOf(getIdentity(adminId));
    expect(error.status).toBe(404);
    expect(error.info).toMatchObject({ type: "https://pkgsilo/problems/identity-not-found" });
    expect((await errorOf(deleteIdentity(adminId))).status).toBe(404);
  });

  it("takes me for the actor", async () => {
    const fetched = await getIdentity("me");

    expect(fetched.data.data).toMatchObject({ id: adminId, username: "admin" });
  });
});

describe("createIdentitiesHandlers for the reader", () => {
  beforeEach(() => {
    serveAs(reader);
  });

  it("allows no action on any identity", async () => {
    const response = await listIdentities({ offset: 0, limit: 100 });

    expect(response.data.data.map(({ allowedActions }) => allowedActions)).toEqual(
      fixedIdentities.map(() => []),
    );
  });

  it("denies every write", async () => {
    const created = await errorOf(createIdentity({ username: "new.user", displayName: "New" }));
    const patched = await errorOf(patchIdentity(readerId, { displayName: "Me" }));
    const deleted = await errorOf(deleteIdentity(adminId));

    expect(created).toMatchObject({
      status: 403,
      info: {
        type: "https://pkgsilo/problems/fields-denied",
        errors: [{ pointer: "/username" }, { pointer: "/displayName" }],
      },
    });
    expect(patched).toMatchObject({
      status: 403,
      info: {
        type: "https://pkgsilo/problems/fields-denied",
        errors: [{ pointer: "/displayName" }],
      },
    });
    expect(deleted).toMatchObject({ status: 403, info: { type: "about:blank" } });
    expect((await getIdentity(adminId)).data.data.displayName).toBe("Administrator");
  });
});

describe("createIdentitiesHandlers for self", () => {
  beforeEach(() => {
    serveAs(self);
  });

  it("allows editing only its own identity", async () => {
    const own = await getIdentity(selfId);
    const other = await getIdentity(adminId);

    expect(own.data.data.allowedActions).toEqual([
      "identity:update:username",
      "identity:update:displayName",
    ]);
    expect(other.data.data.allowedActions).toEqual([]);
  });

  it("edits its own identity, and no other one", async () => {
    const patched = await patchIdentity("me", { displayName: "Me" });
    const denied = await errorOf(patchIdentity(adminId, { displayName: "Me" }));

    expect(patched.data.data).toMatchObject({ id: selfId, displayName: "Me", updatedBy: selfId });
    expect(denied).toMatchObject({
      status: 403,
      info: { type: "https://pkgsilo/problems/fields-denied" },
    });
    expect((await errorOf(deleteIdentity(selfId))).status).toBe(403);
  });
});
