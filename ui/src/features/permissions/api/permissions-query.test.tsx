import { focusManager, onlineManager } from "@tanstack/react-query";
import { waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, describe, expect, it, vi } from "vitest";

import { getListPermissionsMockHandler } from "@/api/gen/client/permissions/permissions.msw.ts";
import {
  permissionsQueryOptions,
  unknownPermissionsRefetchInterval,
} from "@/features/permissions/api/permissions-query.ts";
import { usePermissions } from "@/features/permissions/hooks/use-permissions.ts";
import { createTestClients } from "@/test/clients.ts";
import { renderHookWithClient } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { testIdentity } from "@/test/session.ts";

const permissionsPath = "/api/v1/identities/me/permissions";

// An error response of the permissions endpoint, with the status.
function problem(status: number) {
  return HttpResponse.json(
    { type: "about:blank", title: "Error", status },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

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
    vi.useRealTimers();
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

  it("allows every action while the list doesn't load, and loads it again", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const answer = { failing: true };
    server.use(
      http.get(permissionsPath, () =>
        answer.failing
          ? problem(502)
          : HttpResponse.json({ data: { id: testIdentity.id, allowedActions: ["identity:read"] } }),
      ),
    );
    const { result } = renderHookWithClient(() => usePermissions());
    await waitFor(() => {
      expect(result.current).not.toBeNull();
    });
    expect(result.current.allowedActions).toBeUndefined();
    expect(result.current.can("identity:delete")).toBe(true);

    answer.failing = false;
    await vi.advanceTimersByTimeAsync(unknownPermissionsRefetchInterval);

    await waitFor(() => {
      expect(result.current.allowedActions).toEqual(["identity:read"]);
    });
    expect(result.current.can("identity:delete")).toBe(false);
  });

  it("leaves the list unknown without an answer", async () => {
    server.use(http.get(permissionsPath, () => HttpResponse.error()));
    const { queryClient } = createTestClients();

    await expect(queryClient.fetchQuery(permissionsQueryOptions())).resolves.toBeNull();
  });

  it.each([401, 404])("fails on an error response %i, which answers", async (status) => {
    server.use(http.get(permissionsPath, () => problem(status)));
    const { queryClient } = createTestClients();

    await expect(queryClient.fetchQuery(permissionsQueryOptions("someone"))).rejects.toMatchObject({
      status,
    });
  });
});
