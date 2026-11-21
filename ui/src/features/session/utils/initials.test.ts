import { describe, expect, it } from "vitest";

import { initials } from "@/features/session/utils/initials.ts";

describe("initials", () => {
  it.each([
    ["Ada Lovelace", "AL"],
    ["admin", "A"],
    ["grace.hopper", "GH"],
    ["ci-bot", "CB"],
    ["Jean Jacques Rousseau", "JJ"],
    ["  émile  zola ", "ÉZ"],
  ])("of %s is %s", (name, want) => {
    expect(initials(name)).toBe(want);
  });
});
