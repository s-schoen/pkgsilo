import { describe, expect, it } from "vitest";

import { initials } from "@/utils/initials.ts";

describe("initials", () => {
  it.each([
    ["Ada Lovelace", "AL"],
    ["admin", "A"],
    ["grace.hopper", "GH"],
    ["ci-bot", "CB"],
    ["Jean Jacques Rousseau", "JJ"],
    ["  émile  zola ", "ÉZ"],
    // An e and a combining acute accent.
    ["e\u0301mile Zola", "\u00c9Z".normalize("NFD")],
    ["𝓐da Lovelace", "𝓐L"],
    ["😀 Bot", "😀B"],
    ["👩‍💻 Dev", "👩‍💻D"],
  ])("of %s is %s", (name, want) => {
    expect(initials(name)).toBe(want);
  });
});
