import { describe, expect, it } from "vitest";

import { createClients } from "@/app/clients.ts";

// Fetches a query whose every attempt fails with error, through the default options of the clients,
// and returns how often it tried.
async function attempts(clients: ReturnType<typeof createClients>, error: Error) {
  let count = 0;
  await expect(
    clients.queryClient.fetchQuery({
      queryKey: ["test"],
      queryFn: () => {
        count++;
        throw error;
      },
    }),
  ).rejects.toBe(error);
  return count;
}

// An error of the generated client: an Error with the response's status.
function apiError(status: number) {
  return Object.assign(new Error(`status ${status}`), { status });
}

// The app's clients, without the delay between the retries.
function retryingClients() {
  return createClients({ defaultOptions: { queries: { retryDelay: 0 } } });
}

describe("createClients", () => {
  it.each([400, 401, 403, 404, 422, 499])("doesn't retry a %i response", async (status) => {
    expect(await attempts(retryingClients(), apiError(status))).toBe(1);
  });

  it.each([
    ["a 500 response", apiError(500)],
    ["a 502 response", apiError(502)],
    ["an error without a response", new Error("offline")],
  ])("retries %s three times", async (_, error) => {
    expect(await attempts(retryingClients(), error)).toBe(4);
  });

  it("takes the retry of its config instead", async () => {
    const own = createClients({ defaultOptions: { queries: { retry: false } } });

    expect(await attempts(own, apiError(500))).toBe(1);
  });
});
