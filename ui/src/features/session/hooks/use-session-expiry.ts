import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import {
  getGetCurrentSessionQueryKey,
  useGetCurrentSession,
} from "@/api/gen/client/sessions/sessions.ts";

// The longest delay that setTimeout takes: a longer one fires at once.
const maxTimeout = 2 ** 31 - 1;

// Checks the session again once it ends at the latest, at its expiresAt, when the browser drops its
// cookie too: the check answers 401, and redirectOnUnauthorized leads to the sign-in page, even if
// the user does nothing. A session that ends earlier, since it was idle, is noticed by the next
// request, such as the check of the session when the tab gets the focus again. It never polls, which
// would keep an idle session alive.
export function useSessionExpiry() {
  const queryClient = useQueryClient();
  const { data } = useGetCurrentSession();
  const expiresAt = data?.data.data.expiresAt;

  useEffect(() => {
    if (expiresAt === undefined) {
      return;
    }
    const delay = Math.min(Math.max(new Date(expiresAt).getTime() - Date.now(), 0), maxTimeout);
    const timeout = setTimeout(() => {
      void queryClient.invalidateQueries({ queryKey: getGetCurrentSessionQueryKey() });
    }, delay);
    return () => {
      clearTimeout(timeout);
    };
  }, [expiresAt, queryClient]);
}
