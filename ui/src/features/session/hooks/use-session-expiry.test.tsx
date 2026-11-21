import { waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useSessionExpiry } from "@/features/session/hooks/use-session-expiry.ts";
import { renderHookWithClient } from "@/test/render.tsx";
import { server } from "@/test/server.ts";

const now = new Date("2026-11-21T12:00:00Z");

// Answers the current session, which ends at expiresAt, and counts the requests. With serverNow, the
// response has the Date header of a server whose clock shows that time.
function sessionEndingAt(expiresAt: Date, serverNow?: Date) {
  const requests = { count: 0 };
  server.use(
    http.get("/api/v1/sessions/current", () => {
      requests.count++;
      return HttpResponse.json(
        {
          data: {
            id: "00000000-0000-4000-8000-000000000001",
            identityId: "00000000-0000-4000-8000-000000000002",
            createdAt: now.toISOString(),
            expiresAt: expiresAt.toISOString(),
          },
        },
        serverNow && { headers: { Date: serverNow.toUTCString() } },
      );
    }),
  );
  return requests;
}

const hour = 60 * 60 * 1000;
const day = 24 * hour;

describe("useSessionExpiry", () => {
  beforeEach(() => {
    // The fake clock goes on with the real one too, so that waitFor, which polls with setTimeout,
    // goes on as well; a test leaves a second of slack for that.
    vi.useFakeTimers({
      toFake: ["Date", "setTimeout", "clearTimeout"],
      now,
      shouldAdvanceTime: true,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("checks the session again once it ends", async () => {
    const requests = sessionEndingAt(new Date(now.getTime() + 60_000));
    renderHookWithClient(() => {
      useSessionExpiry();
    });
    await waitFor(() => {
      expect(requests.count).toBe(1);
    });

    await vi.advanceTimersByTimeAsync(59_000);
    expect(requests.count).toBe(1);

    await vi.advanceTimersByTimeAsync(1_000);
    await waitFor(() => {
      expect(requests.count).toBe(2);
    });
  });

  it("waits by the server's clock when the browser's is off", async () => {
    // The browser's clock runs an hour ahead: by its own, the session ended long ago.
    const serverNow = new Date(now.getTime() - hour);
    const requests = sessionEndingAt(new Date(serverNow.getTime() + 60_000), serverNow);
    renderHookWithClient(() => {
      useSessionExpiry();
    });
    await waitFor(() => {
      expect(requests.count).toBe(1);
    });

    await vi.advanceTimersByTimeAsync(59_000);
    expect(requests.count).toBe(1);

    await vi.advanceTimersByTimeAsync(1_000);
    await waitFor(() => {
      expect(requests.count).toBe(2);
    });
  });

  it("checks a session that ends later than setTimeout can wait once it ends", async () => {
    // setTimeout waits about 24.8 days at most.
    const requests = sessionEndingAt(new Date(now.getTime() + 30 * day));
    renderHookWithClient(() => {
      useSessionExpiry();
    });
    await waitFor(() => {
      expect(requests.count).toBe(1);
    });

    await vi.advanceTimersByTimeAsync(30 * day - 1_000);
    expect(requests.count).toBe(1);

    await vi.advanceTimersByTimeAsync(1_000);
    await waitFor(() => {
      expect(requests.count).toBe(2);
    });
  });

  it("checks a session that has already ended at once", async () => {
    const requests = sessionEndingAt(new Date(now.getTime() - 1));
    renderHookWithClient(() => {
      useSessionExpiry();
    });

    await vi.advanceTimersByTimeAsync(0);
    await waitFor(() => {
      expect(requests.count).toBe(2);
    });
  });

  it("stops waiting once it unmounts", async () => {
    const requests = sessionEndingAt(new Date(now.getTime() + 60_000));
    const { unmount } = renderHookWithClient(() => {
      useSessionExpiry();
    });
    await waitFor(() => {
      expect(requests.count).toBe(1);
    });

    unmount();
    await vi.advanceTimersByTimeAsync(60_000);

    expect(requests.count).toBe(1);
  });
});
