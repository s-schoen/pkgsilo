import { afterEach, describe, expect, it } from "vitest";
import { z } from "zod/v4";
import { de, en } from "zod/v4/locales";

import { configureZodLocale } from "@/lib/zod-locale.ts";

const schema = z.string().min(3);

function message() {
  return schema.safeParse("ab").error?.issues[0]?.message;
}

// The message of a locale's own error map, so that the tests don't repeat zod's text.
function messageIn(locale: typeof en) {
  return schema.safeParse("ab", { error: locale().localeError }).error?.issues[0]?.message;
}

describe("configureZodLocale", () => {
  // The setting is global: set it back to the locale of src/test/setup.ts.
  afterEach(() => {
    configureZodLocale("en");
  });

  it("gives zod's messages in German", () => {
    configureZodLocale("de");

    expect(message()).toBe(messageIn(de));
    expect(message()).not.toBe(messageIn(en));
  });

  it("gives zod's messages in English", () => {
    configureZodLocale("de");

    configureZodLocale("en");

    expect(message()).toBe(messageIn(en));
  });
});
