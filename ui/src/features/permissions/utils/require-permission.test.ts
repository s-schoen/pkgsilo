import { isNotFound } from "@tanstack/react-router";
import { describe, expect, it } from "vitest";

import { getListPermissionsMockHandler } from "@/api/gen/client/permissions/permissions.msw.ts";
import { requirePermission } from "@/features/permissions/utils/require-permission.ts";
import { createTestClients } from "@/test/clients.ts";
import { server } from "@/test/server.ts";
import { testIdentity } from "@/test/session.ts";

// Answers the permissions: ownedActions with an owner, actions without.
function permissions(actions: Array<string>, ownedActions: Array<string> = []) {
  return getListPermissionsMockHandler(({ request }) => ({
    data: {
      id: testIdentity.id,
      allowedActions: new URL(request.url).searchParams.has("owner") ? ownedActions : actions,
    },
  }));
}

async function rejection(promise: Promise<unknown>): Promise<unknown> {
  try {
    await promise;
  } catch (error) {
    return error;
  }
  throw new Error("expected a rejection");
}

describe("requirePermission", () => {
  it("passes an allowed action", async () => {
    server.use(permissions(["identity:create:username", "identity:create:displayName"]));
    const { queryClient } = createTestClients();

    await expect(
      requirePermission(queryClient, ["identity:create:username", "identity:create:displayName"]),
    ).resolves.toBeUndefined();
  });

  it("throws notFound for a denied action", async () => {
    server.use(permissions(["identity:create:username"]));
    const { queryClient } = createTestClients();

    expect(isNotFound(await rejection(requirePermission(queryClient, "identity:delete")))).toBe(
      true,
    );
    // Every action of a list must be allowed.
    expect(
      isNotFound(
        await rejection(
          requirePermission(queryClient, [
            "identity:create:username",
            "identity:create:displayName",
          ]),
        ),
      ),
    ).toBe(true);
  });

  it("checks the actions on an owner's resources", async () => {
    server.use(permissions([], ["apikey:create:name"]));
    const { queryClient } = createTestClients();

    await expect(
      requirePermission(queryClient, "apikey:create:name", "me"),
    ).resolves.toBeUndefined();
    expect(isNotFound(await rejection(requirePermission(queryClient, "apikey:create:name")))).toBe(
      true,
    );
  });
});
