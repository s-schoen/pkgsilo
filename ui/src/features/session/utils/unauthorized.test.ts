import { screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { getCreateSessionMutationKey } from "@/api/gen/client/sessions/sessions.ts";
import { isUnauthorized } from "@/features/session/utils/unauthorized.ts";
import { m } from "@/paraglide/messages.js";
import { renderRoute } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { signedIn, signedOut } from "@/test/session.ts";

import type { QueryClient } from "@tanstack/react-query";

// An error of the generated client: an Error with the response's status.
function apiError(status: number) {
  return Object.assign(new Error(`status ${status}`), { status });
}

// Runs a mutation that fails with error, as one of the API's hooks would.
async function failMutation(
  queryClient: QueryClient,
  error: Error,
  mutationKey?: ReadonlyArray<unknown>,
) {
  const mutation = queryClient.getMutationCache().build(queryClient, {
    mutationKey,
    mutationFn: () => Promise.reject(error),
  });
  await expect(mutation.execute(undefined)).rejects.toBe(error);
}

// A page of a user who was signed in, with the router. The session has ended since: the next check
// of it answers 401.
async function renderPage() {
  server.use(...signedIn());
  const result = renderRoute("/does-not-exist");
  await screen.findByRole("heading", { level: 1, name: m.notFound() });
  server.use(signedOut());
  return result;
}

describe("isUnauthorized", () => {
  it.each([
    ["a 401 of the API", apiError(401), true],
    ["a 403 of the API", apiError(403), false],
    ["an error without a status", new Error("offline"), false],
    ["an object with the status that isn't an Error", { status: 401 }, false],
  ])("is %s: %s", (_, error, want) => {
    expect(isUnauthorized(error)).toBe(want);
  });
});

// The queries' 401 is tested through the whole app in src/app/session.test.tsx.
describe("redirectOnUnauthorized", () => {
  it("leads to the sign-in page once a mutation fails with 401, in place of the page", async () => {
    const { router } = await renderPage();

    await failMutation(router.options.context.queryClient, apiError(401));

    await screen.findByRole("heading", { level: 1, name: m.signInTitle() });
    expect(router.state.location.search).toEqual({ redirect: "/does-not-exist" });
    expect(router.history.length).toBe(1);
  });

  it("removes the collections and the cached responses of the session that ended", async () => {
    const { router } = await renderPage();
    const { queryClient, dbClient } = router.options.context;
    const cleanup = vi.spyOn(dbClient, "cleanup");

    await failMutation(queryClient, apiError(401));

    await waitFor(() => {
      expect(cleanup).toHaveBeenCalledOnce();
    });
    await waitFor(() => {
      expect(queryClient.getQueryCache().getAll()).toEqual([]);
    });
  });

  it.each([
    ["an error that isn't 401", apiError(403), undefined],
    ["a failed sign-in, which its form shows", apiError(401), getCreateSessionMutationKey()],
  ])("leads nowhere for %s", async (_, error, mutationKey) => {
    const { router } = await renderPage();

    await failMutation(router.options.context.queryClient, error, mutationKey);

    expect(router.state.location.pathname).toBe("/does-not-exist");
    expect(screen.getByRole("heading", { level: 1, name: m.notFound() })).toBeInTheDocument();
  });
});
