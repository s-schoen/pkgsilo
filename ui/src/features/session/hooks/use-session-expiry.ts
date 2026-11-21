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
// would keep an idle session alive. It waits by the server's clock (endsAt), and in steps of
// setTimeout's longest delay for a session that ends later than that.
export function useSessionExpiry() {
  const queryClient = useQueryClient();
  const { data, dataUpdatedAt } = useGetCurrentSession();
  const expiresAt = data?.data.data.expiresAt;
  const deadline =
    expiresAt === undefined
      ? undefined
      : endsAt(expiresAt, data?.headers.get("Date") ?? null, dataUpdatedAt);

  useEffect(() => {
    if (deadline === undefined) {
      return;
    }
    let timeout: ReturnType<typeof setTimeout>;
    const wait = () => {
      const delay = deadline - Date.now();
      if (delay > maxTimeout) {
        timeout = setTimeout(wait, maxTimeout);
        return;
      }
      timeout = setTimeout(
        () => {
          void queryClient.invalidateQueries({ queryKey: getGetCurrentSessionQueryKey() });
        },
        Math.max(delay, 0),
      );
    };
    wait();
    return () => {
      clearTimeout(timeout);
    };
  }, [deadline, queryClient]);
}

// When the session ends by the browser's clock, which may be off: when its response arrived
// (receivedAt), plus the time that it had left then by the server's clock, from the response's Date
// header. The header counts whole seconds and the response took a while to arrive, so this is never
// before the session ends, which would find it still valid; each check before then answers a later
// time. A response without a Date, such as a mock's, counts by the browser's clock, and its time
// stays the same from one check to the next, so that a session that a check finds valid after that
// time isn't checked again at once.
function endsAt(expiresAt: string, serverDate: string | null, receivedAt: number): number {
  const end = Date.parse(expiresAt);
  const serverNow = serverDate === null ? Number.NaN : Date.parse(serverDate);
  return Number.isNaN(serverNow) ? end : receivedAt + (end - serverNow);
}
