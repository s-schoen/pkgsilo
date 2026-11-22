import { describe, expect, it } from "vitest";

import { displayNameSchema, usernameSchema } from "@/features/identities/utils/identity-schema.ts";
import { m } from "@/paraglide/messages.js";

import type { z } from "zod/v4";

// The first message for value, or undefined for a valid one.
function errorOf(schema: z.ZodType<string, string>, value: string) {
  return schema.safeParse(value).error?.issues[0]?.message;
}

describe("usernameSchema", () => {
  it.each(["ada", "ada.lovelace", "a_1-b.2", "a".repeat(64)])("accepts %s", (value) => {
    expect(errorOf(usernameSchema, value)).toBeUndefined();
  });

  it.each(["ab", "a".repeat(65), "Ada", "ada..king", ".ada", "ada-", "ada king"])(
    "rejects %s",
    (value) => {
      expect(errorOf(usernameSchema, value)).toBe(m.identityUsernameInvalid());
    },
  );
});

describe("displayNameSchema", () => {
  it.each(["Ada Lovelace", "A", "😀".repeat(128)])("accepts %s", (value) => {
    expect(errorOf(displayNameSchema, value)).toBeUndefined();
  });

  it.each(["", "   "])("requires a name, not %j", (value) => {
    expect(errorOf(displayNameSchema, value)).toBe(m.identityDisplayNameRequired());
  });

  it.each(["Ada\nLovelace", "Ada\tLovelace", "a".repeat(129)])("rejects %j", (value) => {
    expect(errorOf(displayNameSchema, value)).toBe(m.identityDisplayNameInvalid());
  });
});
