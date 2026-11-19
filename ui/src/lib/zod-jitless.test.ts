import { describe, expect, it, vi } from "vitest";
import { z } from "zod/v4";

// src/test/setup.ts imports zod-jitless.ts first, as main.tsx does.
describe("zod-jitless", () => {
  it("parses an object without compiling code", () => {
    const compile = vi.spyOn(globalThis, "Function");

    const schema = z.object({ name: z.string() });

    expect(schema.parse({ name: "pkgsilo" })).toEqual({ name: "pkgsilo" });
    expect(compile).not.toHaveBeenCalled();
  });
});
