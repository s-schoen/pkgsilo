import { describe, expect, it } from "vitest";

import { routeHref } from "@/utils/route-href.ts";

describe("routeHref", () => {
  it("fills in the path params", () => {
    expect(routeHref("/identities/$id", { id: "0001" })).toBe("/identities/0001");
  });

  it("encodes a param", () => {
    expect(routeHref("/identities/$id", { id: "a/b c" })).toBe("/identities/a%2Fb%20c");
  });
});
