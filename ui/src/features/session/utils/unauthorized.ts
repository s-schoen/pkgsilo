import { getCreateSessionMutationKey } from "@/api/gen/client/sessions/sessions.ts";
import { clearSessionData } from "@/features/session/utils/clear-session-data.ts";
import { signInPath } from "@/features/session/utils/redirect-target.ts";

import type { DbClient } from "@tanstack/react-db";
import type { QueryClient } from "@tanstack/react-query";
import type { RegisteredRouter } from "@tanstack/react-router";

// Whether error is the API's answer to a request without a valid session: its session expired or
// ended (a logout in another tab, a changed password), or the request had none.
export function isUnauthorized(error: unknown): boolean {
  return error instanceof Error && "status" in error && error.status === 401;
}

const createSessionKey = getCreateSessionMutationKey()[0];

// Leads to the sign-in page once a query or a mutation fails with 401, with the page that the user
// was on as its redirect, so that a sign-in leads back to it. The sign-in page replaces that page in
// the history: Back would lead to it, whose 401 would lead here again. Then it removes what the
// clients hold of the session that ended (clearSessionData). A failed sign-in answers 401 too, which
// its form shows instead, and a 401 on the sign-in page leads nowhere. Returns the function that
// stops it.
export function redirectOnUnauthorized(
  clients: { queryClient: QueryClient; dbClient: DbClient },
  router: RegisteredRouter,
) {
  function onError(error: unknown) {
    if (!isUnauthorized(error) || router.latestLocation.pathname === signInPath) {
      return;
    }
    void router
      .navigate({
        to: signInPath,
        search: { redirect: router.latestLocation.href },
        replace: true,
      })
      .then(() => clearSessionData(clients));
  }

  const unsubscribeQueries = clients.queryClient.getQueryCache().subscribe((event) => {
    if (event.type === "updated" && event.action.type === "error") {
      onError(event.action.error);
    }
  });
  const unsubscribeMutations = clients.queryClient.getMutationCache().subscribe((event) => {
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
