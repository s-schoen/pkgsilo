import { describe, expect, it, vi } from "vitest";

import { clearSessionData } from "@/features/session/utils/clear-session-data.ts";
import { createTestClients } from "@/test/clients.ts";

describe("clearSessionData", () => {
  it("removes the collections and every cached response", async () => {
    const clients = createTestClients();
    clients.queryClient.setQueryData(["identities", "1"], { displayName: "Ada Lovelace" });
    const cleanup = vi.spyOn(clients.dbClient, "cleanup");

    await clearSessionData(clients);

    expect(cleanup).toHaveBeenCalledOnce();
    expect(clients.queryClient.getQueryCache().getAll()).toEqual([]);
  });

  it("removes the cached responses even when a collection fails to clean up", async () => {
    const clients = createTestClients();
    clients.queryClient.setQueryData(["identities", "1"], { displayName: "Ada Lovelace" });
    const failure = new Error("cleanup failed");
    vi.spyOn(clients.dbClient, "cleanup").mockRejectedValue(failure);

    await expect(clearSessionData(clients)).rejects.toBe(failure);

    expect(clients.queryClient.getQueryCache().getAll()).toEqual([]);
  });
});
