import { describe, expect, it } from "vitest";

import { allows } from "@/features/permissions/utils/allows.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

describe("allows", () => {
  const id = "00000000-0000-4000-8000-000000000003";

  it("allows the actions that the row lists", () => {
    const identity: IdentityResponse = { id, allowedActions: ["identity:update:displayName"] };

    expect(allows(identity, "identity:update:displayName")).toBe(true);
    expect(allows(identity, "identity:delete")).toBe(false);
  });

  it("allows nothing on a row without allowedActions", () => {
    const identity: IdentityResponse = { id };

    expect(allows(identity, "identity:delete")).toBe(false);
  });

  it("takes only the actions of the row's type", () => {
    const identity: IdentityResponse = { id, allowedActions: [] };

    // @ts-expect-error group:delete isn't an action of an identity.
    expect(allows(identity, "group:delete")).toBe(false);
  });
});
