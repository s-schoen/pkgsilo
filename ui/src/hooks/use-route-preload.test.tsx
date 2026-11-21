import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RoutePreloadContext, useRoutePreload } from "@/hooks/use-route-preload.ts";

import type { ReactNode } from "react";
import type { PressEvent } from "react-aria-components";

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

// The handlers of a link to /, under a provider whose preload is a mock.
function renderPreload() {
  const preload = vi.fn<(href: string) => void>();
  const { result, unmount } = renderHook(() => useRoutePreload("/"), {
    wrapper: ({ children }: { children: ReactNode }) => (
      <RoutePreloadContext value={preload}>{children}</RoutePreloadContext>
    ),
  });
  return { preload, handlers: result.current, unmount };
}

function pressStart(pointerType: PressEvent["pointerType"]) {
  return { pointerType } as PressEvent;
}

describe("useRoutePreload", () => {
  it("preloads the route once the pointer rests on the link", () => {
    const { preload, handlers } = renderPreload();

    handlers.onHoverStart();
    expect(preload).not.toHaveBeenCalled();

    vi.advanceTimersByTime(50);
    expect(preload).toHaveBeenCalledExactlyOnceWith("/");
  });

  it("preloads nothing for a pointer that only crosses the link", () => {
    const { preload, handlers } = renderPreload();

    handlers.onHoverStart();
    vi.advanceTimersByTime(20);
    handlers.onHoverEnd();
    vi.advanceTimersByTime(50);

    expect(preload).not.toHaveBeenCalled();
  });

  it("preloads at once when the link is focused", () => {
    const { preload, handlers } = renderPreload();

    handlers.onFocus();

    expect(preload).toHaveBeenCalledExactlyOnceWith("/");
  });

  it("preloads at once when a touch starts on the link, which has no hover", () => {
    const { preload, handlers } = renderPreload();

    handlers.onPressStart(pressStart("mouse"));
    expect(preload).not.toHaveBeenCalled();

    handlers.onPressStart(pressStart("touch"));
    expect(preload).toHaveBeenCalledExactlyOnceWith("/");
  });

  it("preloads nothing once the link is gone", () => {
    const { preload, handlers, unmount } = renderPreload();

    handlers.onHoverStart();
    unmount();
    vi.advanceTimersByTime(50);

    expect(preload).not.toHaveBeenCalled();
  });

  it("preloads nothing without a router, as in a story", () => {
    const { result } = renderHook(() => useRoutePreload("/"));

    expect(() => {
      result.current.onFocus();
      result.current.onHoverStart();
      vi.advanceTimersByTime(50);
    }).not.toThrow();
  });
});
