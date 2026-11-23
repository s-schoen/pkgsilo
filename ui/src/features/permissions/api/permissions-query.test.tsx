import { focusManager, onlineManager } from "@tanstack/react-query";
import { waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { getListPermissionsMockHandler } from "@/api/gen/client/permissions/permissions.msw.ts";
import { usePermissions } from "@/features/permissions/hooks/use-permissions.ts";
import { renderHookWithClient } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { testIdentity } from "@/test/session.ts";

// Answers the permissions and counts the requests.
function countedPermissions() {
  const requests = { count: 0 };
  server.use(
    getListPermissionsMockHandler(() => {
      requests.count++;
      return { data: { id: testIdentity.id, allowedActions: ["identity:read"] } };
    }),
  );
  return requests;
}

async function renderPermissions() {
  const { result } = renderHookWithClient(() => usePermissions());
  await waitFor(() => {
    expect(result.current).not.toBeNull();
  });
}

describe("permissionsQueryOptions", () => {
  afterEach(() => {
    focusManager.setFocused(undefined);
    onlineManager.setOnline(true);
  });

  it("refetches the permissions when the window gets the focus", async () => {
    const requests = countedPermissions();
    await renderPermissions();
    expect(requests.count).toBe(1);

    focusManager.setFocused(false);
    focusManager.setFocused(true);

    await waitFor(() => {
      expect(requests.count).toBe(2);
    });
  });

  it("refetches the permissions on a reconnect", async () => {
    const requests = countedPermissions();
    await renderPermissions();
    expect(requests.count).toBe(1);

    onlineManager.setOnline(false);
    onlineManager.setOnline(true);

    await waitFor(() => {
      expect(requests.count).toBe(2);
    });
  });
});
