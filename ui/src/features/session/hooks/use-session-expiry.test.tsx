import { waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useSessionExpiry } from "@/features/session/hooks/use-session-expiry.ts";
import { renderHookWithClient } from "@/test/render.tsx";
import { server } from "@/test/server.ts";

const now = new Date("2026-11-21T12:00:00Z");

// Answers the current session, which ends at expiresAt, and counts the requests.
function sessionEndingAt(expiresAt: Date) {
  const requests = { count: 0 };
  server.use(
    http.get("/api/v1/sessions/current", () => {
      requests.count++;
      return HttpResponse.json({
        data: {
          id: "00000000-0000-4000-8000-000000000001",
          identityId: "00000000-0000-4000-8000-000000000002",
          createdAt: now.toISOString(),
          expiresAt: expiresAt.toISOString(),
        },
      });
    }),
  );
  return requests;
}

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
