import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { useColorScheme } from "@/features/appearance/hooks/use-color-scheme.ts";
import { writeColorScheme } from "@/features/appearance/utils/color-scheme.ts";

afterEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset["colorScheme"];
});

describe("useColorScheme", () => {
  it("starts with the stored preference", () => {
    writeColorScheme("light");

    const { result } = renderHook(() => useColorScheme());

    expect(result.current[0]).toBe("light");
  });

  it("stores and applies a new preference", () => {
    const { result } = renderHook(() => useColorScheme());

    act(() => {
      result.current[1]("dark");
    });

    expect(result.current[0]).toBe("dark");
    expect(document.documentElement.dataset["colorScheme"]).toBe("dark");
    expect(renderHook(() => useColorScheme()).result.current[0]).toBe("dark");
  });
});
