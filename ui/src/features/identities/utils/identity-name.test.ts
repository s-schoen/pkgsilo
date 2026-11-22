import { describe, expect, it } from "vitest";

import { identityName } from "@/features/identities/utils/identity-name.ts";

describe("identityName", () => {
  it.each([
    [{ id: "1", username: "ada", displayName: "Ada Lovelace" }, "Ada Lovelace"],
    [{ id: "1", username: "ada" }, "ada"],
    [{ id: "1" }, "1"],
  ])("of %o is %s", (identity, want) => {
    expect(identityName(identity)).toBe(want);
  });
});
