import { describe, expect, it } from "vitest";

import { orderGroups } from "@/components/ui/data-table/grouping.ts";

function groupRow(id: string, groupingValue: unknown, subRows: Array<string> = [id]) {
  return { id, groupingValue, subRows };
}

describe("orderGroups", () => {
  it("orders the groups by their labels in the locale's collation", () => {
    const groups = orderGroups(
      [groupRow("b", "Zebra"), groupRow("a", "Ärger"), groupRow("c", "apple")],
      { id: "name", label: () => "Name" },
      "de",
    );

    // Ä sorts with A, and lower case with upper case, not after Z as their code points would.
    expect(groups.map((group) => group.label)).toEqual(["apple", "Ärger", "Zebra"]);
  });

  it("compares numbers in labels by their value", () => {
    const groups = orderGroups(
      [groupRow("a", "item 10"), groupRow("b", "item 9")],
      { id: "name", label: () => "Name" },
      "en",
    );

    expect(groups.map((group) => group.label)).toEqual(["item 9", "item 10"]);
  });

  it("labels the groups with formatValue", () => {
    const groups = orderGroups(
      [groupRow("a", "revoked"), groupRow("b", "active")],
      { id: "status", label: () => "Status", formatValue: (value) => `Status ${String(value)}` },
      "en",
    );

    expect(groups.map((group) => group.label)).toEqual(["Status active", "Status revoked"]);
  });

  it("orders the groups by compareValues instead of their labels", () => {
    const rank: Record<string, number> = { active: 0, expired: 1, revoked: 2 };
    const groups = orderGroups(
      [groupRow("a", "revoked"), groupRow("b", "active"), groupRow("c", "expired")],
      {
        id: "status",
        label: () => "Status",
        compareValues: (left, right) => rank[String(left)] - rank[String(right)],
      },
      "en",
    );

    expect(groups.map((group) => group.value)).toEqual(["active", "expired", "revoked"]);
  });

  it("keeps groups apart whose labels are equal, in the rows' order", () => {
    const groups = orderGroups(
      [groupRow("a", "id-1", ["x"]), groupRow("b", "id-2", ["y", "z"])],
      { id: "asset", label: () => "Asset", formatValue: () => "server" },
      "en",
    );

    expect(groups).toEqual([
      { id: "a", value: "id-1", label: "server", rows: ["x"] },
      { id: "b", value: "id-2", label: "server", rows: ["y", "z"] },
    ]);
  });

  it("labels a value without formatValue that isn't text as nothing", () => {
    const groups = orderGroups([groupRow("a", undefined)], { id: "x", label: () => "X" }, "en");

    expect(groups[0]?.label).toBe("");
  });
});
