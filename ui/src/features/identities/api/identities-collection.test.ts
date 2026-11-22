import { http, HttpResponse } from "msw";
import { describe, expect, it, vi } from "vitest";

import {
  getDeleteIdentityMockHandler,
  getListIdentitiesMockHandler,
} from "@/api/gen/client/identities/identities.msw.ts";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { createTestClients } from "@/test/clients.ts";
import { server } from "@/test/server.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type { PatchIdentityRequest } from "@/api/gen/client/model/patchIdentityRequest.ts";
import type { InsertIdentityMetadata } from "@/features/identities/api/identities-collection.ts";

const ada: IdentityResponse = { id: "id-ada", username: "ada", displayName: "Ada Lovelace" };
const grace: IdentityResponse = { id: "id-grace", username: "grace", displayName: "Grace Hopper" };

// The list of the API over identities, in pages of limit.
function listOf(identities: Array<IdentityResponse>) {
  return getListIdentitiesMockHandler(({ request }) => {
    const url = new URL(request.url);
    const offset = Number(url.searchParams.get("offset") ?? 0);
    const limit = Number(url.searchParams.get("limit") ?? 100);
    const next = offset + limit;
    return {
      data: identities.slice(offset, next),
      pagination: { offset, limit, nextOffset: next < identities.length ? next : null },
    };
  });
}

function forbidden() {
  return HttpResponse.json(
    { type: "about:blank", title: "Forbidden", status: 403 },
    { status: 403, headers: { "Content-Type": "application/problem+json" } },
  );
}

async function loadedCollection() {
  const { dbClient } = createTestClients();
  const collection = dbClient.collection(identitiesCollection);
  await collection.preload();
  return collection;
}

describe("identitiesCollection", () => {
  it("loads every page of the list", async () => {
    const many = Array.from({ length: 150 }, (_, index) => ({
      id: `id-${String(index)}`,
      username: `user${String(index)}`,
    }));
    server.use(listOf(many));

    const collection = await loadedCollection();

    expect(collection.size).toBe(150);
    expect(collection.get("id-149")).toMatchObject(many[149] ?? {});
  });

  it("patches only the changed fields and keeps the API's answer", async () => {
    server.use(listOf([ada, grace]));
    let body: PatchIdentityRequest | undefined;
    server.use(
      http.patch("*/api/v1/identities/:id", async ({ request, params }) => {
        body = (await request.json()) as PatchIdentityRequest;
        return HttpResponse.json({
          data: { ...ada, ...body, id: String(params.id), updatedAt: "2026-11-22T12:00:00Z" },
        });
      }),
    );
    const collection = await loadedCollection();

    const transaction = collection.update(ada.id, (draft) => {
      draft.displayName = "Ada King";
    });
    expect(collection.get(ada.id)?.displayName).toBe("Ada King");
    await transaction.when("settled");

    expect(body).toEqual({ displayName: "Ada King" });
    expect(collection.get(ada.id)).toMatchObject({
      displayName: "Ada King",
      updatedAt: "2026-11-22T12:00:00Z",
    });
  });

  it("rolls a denied change back, and rejects with the API's error", async () => {
    server.use(listOf([ada]), http.patch("*/api/v1/identities/:id", forbidden));
    const collection = await loadedCollection();

    const transaction = collection.update(ada.id, (draft) => {
      draft.username = "ada.king";
    });

    await expect(transaction.when("settled")).rejects.toMatchObject({ status: 403 });
    expect(collection.get(ada.id)?.username).toBe("ada");
  });

  it("deletes an identity", async () => {
    server.use(listOf([ada, grace]), getDeleteIdentityMockHandler());
    const collection = await loadedCollection();

    await collection.delete(grace.id).when("settled");

    expect(collection.has(grace.id)).toBe(false);
    expect(collection.size).toBe(1);
  });

  it("creates an identity with the API's id", async () => {
    server.use(
      listOf([ada]),
      http.post("*/api/v1/identities", async ({ request }) => {
        const body = (await request.json()) as { username: string; displayName: string };
        return HttpResponse.json({ data: { id: "id-new", ...body } }, { status: 201 });
      }),
    );
    const collection = await loadedCollection();

    await collection
      .insert({ id: "pending", username: "alan", displayName: "Alan Turing" })
      .when("settled");

    expect(collection.get("id-new")).toMatchObject({
      id: "id-new",
      username: "alan",
      displayName: "Alan Turing",
    });
    expect(collection.has("pending")).toBe(false);
  });

  it("hands the created identity to the insert's onCreated", async () => {
    server.use(
      listOf([ada]),
      http.post("*/api/v1/identities", async ({ request }) => {
        const body = (await request.json()) as { username: string; displayName: string };
        return HttpResponse.json({ data: { id: "id-new", ...body } }, { status: 201 });
      }),
    );
    const collection = await loadedCollection();
    const onCreated = vi.fn<InsertIdentityMetadata["onCreated"]>();

    await collection
      .insert(
        { id: "pending", username: "alan", displayName: "Alan Turing" },
        { metadata: { onCreated } satisfies InsertIdentityMetadata },
      )
      .when("settled");

    expect(onCreated).toHaveBeenCalledWith({
      id: "id-new",
      username: "alan",
      displayName: "Alan Turing",
    });
  });
});
