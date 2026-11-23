import { beforeEach, describe, expect, it } from "vitest";

import { listIdentities, patchIdentity } from "@/api/gen/client/identities/identities.ts";
import { listPermissions } from "@/api/gen/client/permissions/permissions.ts";
import {
  createSession,
  deleteCurrentSession,
  getCurrentSession,
} from "@/api/gen/client/sessions/sessions.ts";
import { allActions } from "@/mocks/actions.ts";
import { createHandlers } from "@/mocks/handlers.ts";
import { adminId, readerId, selfId } from "@/mocks/identities-data.ts";
import { server } from "@/test/server.ts";

// The error that the generated client rejects with: the response's status and its problem.
async function errorOf(promise: Promise<unknown>) {
  try {
    await promise;
  } catch (error) {
    return error as { status?: number; info?: Record<string, unknown> };
  }
  throw new Error("expected the request to fail");
}

async function signInAs(username: string) {
  await deleteCurrentSession();
  return createSession({ username, password: "anything" });
}

async function permissions(owner?: string) {
  const response = await listPermissions("me", owner === undefined ? undefined : { owner });
  return response.data.data.allowedActions;
}

const reads = allActions.filter((action) => action.split(":")[1]?.startsWith("read"));

// The mock API of `task ui:dev:mock` as a whole: its session and personas.
describe("createHandlers", () => {
  beforeEach(() => {
    server.use(...createHandlers());
  });

  it("starts signed in as the admin, who may do everything", async () => {
    const session = await getCurrentSession();

    expect(session.data.data.identityId).toBe(adminId);
    expect(await permissions()).toEqual(allActions);
    expect(await permissions("me")).toEqual(
      allActions.filter(
        (action) => action.startsWith("apikey:create:") || action.startsWith("password:"),
      ),
    );
  });

  it("answers 401 to everything once signed out, but a sign-in", async () => {
    await deleteCurrentSession();

    expect((await errorOf(getCurrentSession())).status).toBe(401);
    expect((await errorOf(listIdentities())).status).toBe(401);
    expect((await errorOf(listPermissions("me"))).status).toBe(401);
    expect((await createSession({ username: "admin", password: "x" })).status).toBe(201);
    expect((await getCurrentSession()).data.data.identityId).toBe(adminId);
  });

  it("answers 401 to the sign-in of a username that isn't a persona", async () => {
    expect((await errorOf(signInAs("grace.hopper"))).status).toBe(401);
    expect((await errorOf(getCurrentSession())).status).toBe(401);
  });

  it("signs in as the reader, who may only read", async () => {
    const session = await signInAs("reader");

    expect(session.data.data.identityId).toBe(readerId);
    expect(await permissions()).toEqual(reads);
    expect(await permissions("me")).toEqual(["password:read", "password:read:setAt"]);
    expect((await errorOf(patchIdentity(readerId, { displayName: "Me" }))).status).toBe(403);
  });

  it("signs in as self, who may edit only its own identity", async () => {
    const session = await signInAs("self");

    expect(session.data.data.identityId).toBe(selfId);
    expect(await permissions()).toEqual(
      allActions.filter(
        (action) =>
          reads.includes(action) ||
          action === "identity:update:username" ||
          action === "identity:update:displayName",
      ),
    );
    const identities = (await listIdentities({ offset: 0, limit: 100 })).data.data;
    expect(identities.find(({ id }) => id === selfId)?.allowedActions).toEqual([
      "identity:update:username",
      "identity:update:displayName",
    ]);
    expect(identities.find(({ id }) => id === adminId)?.allowedActions).toEqual([]);
  });

  it("lists only the actor's own permissions", async () => {
    expect((await listPermissions(adminId)).status).toBe(200);
    expect((await errorOf(listPermissions(readerId))).status).toBe(403);
    expect(await errorOf(listPermissions("00000000-0000-4000-8000-0000000000ff"))).toMatchObject({
      status: 404,
      info: { type: "https://pkgsilo/problems/identity-not-found" },
    });
    expect(
      await errorOf(listPermissions("me", { owner: "00000000-0000-4000-8000-0000000000ff" })),
    ).toMatchObject({ status: 404 });
  });
});
