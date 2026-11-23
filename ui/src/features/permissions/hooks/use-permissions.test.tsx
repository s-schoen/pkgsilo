import { waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { getListPermissionsMockHandler } from "@/api/gen/client/permissions/permissions.msw.ts";
import { usePermissions } from "@/features/permissions/hooks/use-permissions.ts";
import { renderHookWithClient } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { testIdentity } from "@/test/session.ts";

// Answers the permissions of the signed-in identity: ownedActions with an owner, actions without,
// and records the owners that it was asked for.
function permissions(actions: Array<string>, ownedActions: Array<string>, owners: Array<string>) {
  return getListPermissionsMockHandler(({ request, params }) => {
    expect(params.identityID).toBe("me");
    const owner = new URL(request.url).searchParams.get("owner");
    owners.push(owner ?? "");
    return {
      data: { id: testIdentity.id, allowedActions: owner === null ? actions : ownedActions },
    };
  });
}

describe("usePermissions", () => {
  it("tells the actions that the signed-in identity's permissions list", async () => {
    const owners: Array<string> = [];
    server.use(permissions(["identity:read", "identity:create:username"], [], owners));

    const { result } = renderHookWithClient(() => usePermissions());

    await waitFor(() => {
      expect(result.current).not.toBeNull();
    });
    expect(result.current.can("identity:read")).toBe(true);
    expect(result.current.can("identity:create:username")).toBe(true);
    expect(result.current.can("identity:delete")).toBe(false);
    expect(owners).toEqual([""]);
  });

  it("tells the actions on an owner's resources", async () => {
    const owners: Array<string> = [];
    server.use(permissions(["identity:read"], ["apikey:create:name"], owners));

    const { result } = renderHookWithClient(() => usePermissions("me"));

    await waitFor(() => {
      expect(result.current).not.toBeNull();
    });
    expect(result.current.can("apikey:create:name")).toBe(true);
    expect(result.current.can("identity:read")).toBe(false);
    expect(owners).toEqual(["me"]);
  });

  it("allows nothing when the list is empty", async () => {
    server.use(getListPermissionsMockHandler({ data: { id: testIdentity.id } }));

    const { result } = renderHookWithClient(() => usePermissions());

    await waitFor(() => {
      expect(result.current).not.toBeNull();
    });
    expect(result.current.can("identity:read")).toBe(false);
  });
});
