import { describe, expect, it } from "vitest";

import { adminId, fixedIdentities, pkgsiloId } from "@/mocks/identities-data.ts";
import { createIdentitiesStore } from "@/mocks/identities-store.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type { StoreResult } from "@/mocks/identities-store.ts";

const actorId = "00000000-0000-4000-8000-0000000000aa";
const newId = "00000000-0000-4000-8000-0000000000bb";
const at = new Date("2026-11-22T12:00:00Z");

const ada: IdentityResponse = {
  id: "00000000-0000-4000-8000-000000000001",
  username: "ada",
  displayName: "Ada Lovelace",
  createdBy: pkgsiloId,
  updatedBy: pkgsiloId,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};
const grace: IdentityResponse = {
  ...ada,
  id: "00000000-0000-4000-8000-000000000002",
  username: "grace",
};

function storeOf(identities: Array<IdentityResponse> = [ada, grace]) {
  return createIdentitiesStore({
    identities,
    actorId: () => actorId,
    now: () => at,
    newId: () => newId,
  });
}

// The value of a result that must have succeeded.
function valueOf<T>(result: StoreResult<T>): T {
  if (!result.ok) {
    throw new Error(`expected success, got ${result.problem.type}`);
  }
  return result.value;
}

// The problem of a result that must have failed.
function problemOf<T>(result: StoreResult<T>) {
  if (result.ok) {
    throw new Error("expected a problem");
  }
  return result.problem;
}

describe("fixedIdentities", () => {
  it("holds the signed-in admin and identities that refer to each other or to pkgsilo", () => {
    const ids = new Set(fixedIdentities.map(({ id }) => id));

    expect(fixedIdentities.length).toBeGreaterThanOrEqual(20);
    expect(ids.size).toBe(fixedIdentities.length);
    expect(ids).toContain(adminId);
    for (const { createdBy, updatedBy } of fixedIdentities) {
      expect([...ids, pkgsiloId]).toContain(createdBy);
      expect([...ids, pkgsiloId]).toContain(updatedBy);
    }
    expect(fixedIdentities.some(({ createdBy }) => createdBy === pkgsiloId)).toBe(true);
  });

  it("starts a store", () => {
    const store = createIdentitiesStore({ actorId: () => adminId });

    expect(valueOf(store.get(adminId))).toMatchObject({ username: "admin" });
  });
});

describe("createIdentitiesStore", () => {
  describe("list", () => {
    it("pages the identities in their order, with the offset of the next page", () => {
      const store = storeOf();

      expect(store.list({ offset: 0, limit: 1 })).toEqual({
        data: [ada],
        pagination: { offset: 0, limit: 1, nextOffset: 1 },
      });
      expect(store.list({ offset: 1, limit: 1 })).toEqual({
        data: [grace],
        pagination: { offset: 1, limit: 1, nextOffset: null },
      });
      expect(store.list({ offset: 5, limit: 1 }).data).toEqual([]);
    });
  });

  describe("get", () => {
    it("returns an identity", () => {
      expect(valueOf(storeOf().get(ada.id))).toEqual(ada);
    });

    it("answers an unknown id with 404", () => {
      expect(problemOf(storeOf().get(newId))).toMatchObject({
        status: 404,
        type: "https://pkgsilo/problems/identity-not-found",
      });
    });

    it("returns a copy, which doesn't change the store", () => {
      const store = storeOf();
      const identity = valueOf(store.get(ada.id));
      identity.username = "changed";

      expect(valueOf(store.get(ada.id)).username).toBe("ada");
    });
  });

  describe("create", () => {
    it("adds an identity by the actor at the end of the list", () => {
      const store = storeOf();

      const created = valueOf(store.create({ username: "linus", displayName: "Linus" }));

      const expected = {
        id: newId,
        username: "linus",
        displayName: "Linus",
        createdBy: actorId,
        updatedBy: actorId,
        createdAt: at.toISOString(),
        updatedAt: at.toISOString(),
      };
      expect(created).toEqual(expected);
      expect(store.list({ offset: 0, limit: 10 }).data.at(-1)).toEqual(expected);
    });

    it("answers a taken username with 409", () => {
      const store = storeOf();

      expect(problemOf(store.create({ username: "ada", displayName: "Ada" }))).toMatchObject({
        status: 409,
        type: "https://pkgsilo/problems/username-taken",
      });
      expect(store.list({ offset: 0, limit: 10 }).data).toHaveLength(2);
    });

    it("answers invalid or missing fields with 422 and their pointers", () => {
      const problem = problemOf(storeOf().create({ username: "A!" }));

      expect(problem).toMatchObject({
        status: 422,
        type: "https://pkgsilo/problems/invalid-body",
      });
      expect(problem.errors?.map(({ pointer }) => pointer)).toEqual(["/username", "/displayName"]);
    });

    it.each([
      ["a username that is too short", { username: "ab" }],
      ["a username with a double separator", { username: "a..b" }],
      ["a username with uppercase letters", { username: "Ada" }],
      ["a blank display name", { displayName: "   " }],
      ["a display name with a line break", { displayName: "Ada\nLovelace" }],
      ["a display name that is too long", { displayName: "x".repeat(129) }],
      ["a field that isn't a string", { displayName: 42 }],
    ])("rejects %s", (_, fields) => {
      const result = storeOf().create({ username: "valid", displayName: "Valid", ...fields });

      expect(problemOf(result).status).toBe(422);
    });
  });

  describe("replace", () => {
    it("sets both fields, as the actor", () => {
      const store = storeOf();

      const replaced = valueOf(store.replace(ada.id, { username: "ada2", displayName: "Ada" }));

      expect(replaced).toEqual({
        ...ada,
        username: "ada2",
        displayName: "Ada",
        updatedBy: actorId,
        updatedAt: at.toISOString(),
      });
      expect(valueOf(store.get(ada.id))).toEqual(replaced);
    });

    it("requires both fields", () => {
      expect(problemOf(storeOf().replace(ada.id, { username: "ada" })).status).toBe(422);
    });

    it("keeps an identity's own username, and rejects another's", () => {
      const store = storeOf();

      expect(store.replace(ada.id, { username: "ada", displayName: "A" }).ok).toBe(true);
      expect(problemOf(store.replace(ada.id, { username: "grace", displayName: "A" })).status).toBe(
        409,
      );
    });

    it("answers an unknown id with 404", () => {
      const result = storeOf().replace(newId, { username: "new", displayName: "New" });

      expect(problemOf(result).status).toBe(404);
    });
  });

  describe("patch", () => {
    it("changes only the fields that the patch sets", () => {
      const store = storeOf();

      const patched = valueOf(store.patch(ada.id, { displayName: "Countess" }));

      expect(patched).toEqual({
        ...ada,
        displayName: "Countess",
        updatedBy: actorId,
        updatedAt: at.toISOString(),
      });
    });

    it("rejects a null, which would remove a field", () => {
      const problem = problemOf(storeOf().patch(ada.id, { username: null }));

      expect(problem.status).toBe(422);
      expect(problem.errors).toEqual([{ pointer: "/username", detail: "cannot be blank" }]);
    });

    it("answers a taken username with 409 and keeps the identity", () => {
      const store = storeOf();

      expect(problemOf(store.patch(ada.id, { username: "grace" })).status).toBe(409);
      expect(valueOf(store.get(ada.id))).toEqual(ada);
    });

    it("answers an unknown id with 404", () => {
      expect(problemOf(storeOf().patch(newId, { displayName: "New" })).status).toBe(404);
    });
  });

  describe("remove", () => {
    it("deletes an identity, which frees its username", () => {
      const store = storeOf();

      expect(store.remove(ada.id).ok).toBe(true);
      expect(problemOf(store.get(ada.id)).status).toBe(404);
      expect(store.create({ username: "ada", displayName: "Ada" }).ok).toBe(true);
    });

    it("answers an unknown id with 404", () => {
      expect(problemOf(storeOf().remove(newId)).status).toBe(404);
    });
  });

  it("keeps a store's changes from the others", () => {
    const first = storeOf();
    const second = storeOf();

    first.remove(ada.id);

    expect(second.get(ada.id).ok).toBe(true);
  });
});
