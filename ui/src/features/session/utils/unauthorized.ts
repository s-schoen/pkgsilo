import { getCreateSessionMutationKey } from "@/api/gen/client/sessions/sessions.ts";
import { signInPath } from "@/features/session/utils/redirect-target.ts";

import type { QueryClient } from "@tanstack/react-query";
import type { RegisteredRouter } from "@tanstack/react-router";

// Whether error is the API's answer to a request without a valid session: its session expired or
// ended (a logout in another tab, a changed password), or the request had none.
export function isUnauthorized(error: unknown): boolean {
  return error instanceof Error && "status" in error && error.status === 401;
}

const createSessionKey = getCreateSessionMutationKey()[0];

// Leads to the sign-in page once a query or a mutation fails with 401, with the page that the user
// was on as its redirect, so that a sign-in leads back to it. Then it removes every cached
// response: they belong to the session that ended. A failed sign-in answers 401 too, which its form
// shows instead, and a 401 on the sign-in page leads nowhere. Returns the function that stops it.
export function redirectOnUnauthorized(queryClient: QueryClient, router: RegisteredRouter) {
  function onError(error: unknown) {
    if (!isUnauthorized(error) || router.latestLocation.pathname === signInPath) {
      return;
    }
    void router
      .navigate({ to: signInPath, search: { redirect: router.latestLocation.href } })
      .then(() => {
        queryClient.removeQueries();
      });
  }

  const unsubscribeQueries = queryClient.getQueryCache().subscribe((event) => {
    if (event.type === "updated" && event.action.type === "error") {
      onError(event.action.error);
    }
  });
  const unsubscribeMutations = queryClient.getMutationCache().subscribe((event) => {
    if (
      event.type === "updated" &&
      event.action.type === "error" &&
      event.mutation.options.mutationKey?.[0] !== createSessionKey
    ) {
      onError(event.action.error);
    }
  });

  return () => {
    unsubscribeQueries();
    unsubscribeMutations();
  };
}
