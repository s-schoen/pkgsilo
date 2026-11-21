import { describe, expect, it } from "vitest";

import { safeRedirectTarget } from "@/features/session/utils/redirect-target.ts";

describe("safeRedirectTarget", () => {
  it.each([
    ["/", "/"],
    ["/groups", "/groups"],
    ["/groups/1?tab=members#top", "/groups/1?tab=members#top"],
  ])("keeps the path %s of this app", (redirect, want) => {
    expect(safeRedirectTarget(redirect)).toBe(want);
  });

  it.each([
    ["no redirect", undefined],
    ["an empty one", ""],
    ["a URL of another host", "https://evil.example/"],
    ["a protocol-relative URL", "//evil.example/"],
    ["a backslash that browsers read as a slash", "/\\evil.example/"],
    ["a relative path", "groups"],
    ["a script", "javascript:alert(1)"],
    ["the sign-in page itself", "/signin?redirect=%2F"],
  ])("leads to the start page for %s", (_, redirect) => {
    expect(safeRedirectTarget(redirect)).toBe("/");
  });
});
