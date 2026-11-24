import { waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useIdentityPermissions } from "@/features/identities/hooks/use-identity-permissions.ts";
import { renderHookWithClient } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { permissions, signedIn, testIdentity } from "@/test/session.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

const grace: IdentityResponse = {
  id: "00000000-0000-4000-8000-000000000003",
  allowedActions: ["identity:delete", "identity:update:username", "identity:update:displayName"],
};

// Renders the hook once the permissions list and the session have loaded.
async function render() {
  const { result } = renderHookWithClient(() => useIdentityPermissions());
  await waitFor(() => {
    expect(result.current).not.toBeNull();
  });
  return result;
}

describe("useIdentityPermissions", () => {
  it("may create with the create of both fields", async () => {
    server.use(...signedIn());
    server.use(permissions(["identity:create:username", "identity:create:displayName"]));

    const result = await render();

    expect(result.current.canCreate).toBe(true);
  });

  it("may not create without the create of one field", async () => {
    server.use(...signedIn());
    server.use(permissions(["identity:create:username"]));

    const result = await render();

    expect(result.current.canCreate).toBe(false);
  });

  it("takes an identity's permissions from its allowedActions", async () => {
    server.use(...signedIn());
    const result = await render();
    await waitFor(() => {
      expect(result.current.of(grace).canDelete).toBe(true);
    });

    expect(result.current.of(grace).canEdit).toEqual({ username: true, displayName: true });
    expect(
      result.current.of({ ...grace, allowedActions: ["identity:update:displayName"] }),
    ).toEqual({ canDelete: false, canEdit: { username: false, displayName: true } });
    // A row without allowedActions, as a new one that the actor may not read, allows nothing.
    expect(result.current.of({ id: grace.id })).toEqual({
      canDelete: false,
      canEdit: { username: false, displayName: false },
    });
  });

  it("doesn't delete the signed-in identity, even where the policies allow it", async () => {
    server.use(...signedIn());
    const result = await render();
    // The session has loaded once another identity may be deleted.
    await waitFor(() => {
      expect(result.current.of(grace).canDelete).toBe(true);
    });

    const self = { ...grace, id: testIdentity.id };
    expect(result.current.of(self).canDelete).toBe(false);
    expect(result.current.of(self).canEdit).toEqual({ username: true, displayName: true });
  });
});
