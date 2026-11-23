import { waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it, vi } from "vitest";

import { onErrorResponse } from "@/api/fetch.ts";
import { getCheckHealthMockHandler } from "@/api/gen/client/health/health.msw.ts";
import { checkHealth, useCheckHealth } from "@/api/gen/client/health/health.ts";
import { renderHookWithClient } from "@/test/render.tsx";
import { server } from "@/test/server.ts";

import type { ErrorResponse } from "@/api/fetch.ts";

// The client is generated (orval.config.ts); these tests check its configuration, through one
// operation: the path, the response that a query resolves with, and the error that it fails with.
describe("generated API client", () => {
  it("resolves a query with the response", async () => {
    server.use(getCheckHealthMockHandler({ status: "healthy", service: "pkgsilo" }));

    const { result } = renderHookWithClient(() => useCheckHealth());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.status).toBe(200);
    expect(result.current.data?.data).toEqual({ status: "healthy", service: "pkgsilo" });
  });

  it("fails a query with the problem of an error response", async () => {
    const problem = { type: "about:blank", title: "Internal Server Error", status: 500 };
    server.use(
      http.get("/api/v1/health", () =>
        HttpResponse.json(problem, {
          status: 500,
          headers: { "Content-Type": "application/problem+json" },
        }),
      ),
    );

    const { result } = renderHookWithClient(() => useCheckHealth());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
    expect(result.current.error?.status).toBe(500);
    expect(result.current.error?.info).toEqual(problem);
  });

  it("shows every error response to the listeners of onErrorResponse", async () => {
    const problem = { type: "about:blank", title: "Forbidden", status: 403 };
    server.use(
      http.get("/api/v1/health", () =>
        HttpResponse.json(problem, {
          status: 403,
          headers: { "Content-Type": "application/problem+json" },
        }),
      ),
    );
    const listener = vi.fn<(response: ErrorResponse) => void>();
    const stop = onErrorResponse(listener);

    await expect(checkHealth()).rejects.toMatchObject({ status: 403, info: problem });
    stop();
    await expect(checkHealth()).rejects.toMatchObject({ status: 403 });

    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith({
      method: "GET",
      path: "/api/v1/health",
      status: 403,
      info: problem,
    });
  });

  it("doesn't show a success to the listeners", async () => {
    server.use(getCheckHealthMockHandler({ status: "healthy", service: "pkgsilo" }));
    const listener = vi.fn<(response: ErrorResponse) => void>();
    const stop = onErrorResponse(listener);

    await expect(checkHealth()).resolves.toMatchObject({ status: 200 });
    stop();

    expect(listener).not.toHaveBeenCalled();
  });
});
