import { useTable } from "@tanstack/react-table";
import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { orderGroups } from "@/components/ui/data-table/grouping.ts";
import { dataTableFeatures } from "@/components/ui/data-table/types.ts";

import type { DataTableColumnDef } from "@/components/ui/data-table/types.ts";

// A group of the grouped row model, whose rows all give `value`.
function groupRow(id: string, value: unknown, rowIds: Array<string> = [id]) {
  return {
    id,
    subRows: rowIds.map((rowId) => ({ id: rowId, getGroupingValue: () => value })),
  };
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

    expect(
      groups.map(({ rows, ...group }) => ({ ...group, rows: rows.map((row) => row.id) })),
    ).toEqual([
      { id: "a", value: "id-1", label: "server", rows: ["x"] },
      { id: "b", value: "id-2", label: "server", rows: ["y", "z"] },
    ]);
  });

  it("labels a value without formatValue that isn't text as nothing", () => {
    const groups = orderGroups([groupRow("a", undefined)], { id: "x", label: () => "X" }, "en");

    expect(groups[0]?.label).toBe("");
  });

  it("gives the values as the column gives them, not as TanStack's text keys", () => {
    type Key = { id: string; owner: string | null; isAdmin: boolean; created: Date };
    const columns: Array<DataTableColumnDef<Key>> = [
      { accessorKey: "owner" },
      { accessorKey: "isAdmin" },
      { accessorKey: "created" },
    ];
    const keys: Array<Key> = [
      { id: "1", owner: null, isAdmin: false, created: new Date("2026-01-02") },
      { id: "2", owner: "alice", isAdmin: true, created: new Date("2026-01-01") },
    ];
    // The groups of the table, grouped by the column.
    const groupsBy = (columnId: string) => {
      const { result } = renderHook(() =>
        useTable({
          features: dataTableFeatures,
          columns,
          data: keys,
          getRowId: (key) => key.id,
          state: { grouping: [columnId] },
        }),
      );
      return result.current.getPreExpandedRowModel().rows;
    };

    const owners = orderGroups(
      groupsBy("owner"),
      {
        id: "owner",
        label: () => "Owner",
        formatValue: (value) => (typeof value === "string" ? value : "nobody"),
      },
      "en",
    );
    expect(owners.map((group) => group.label)).toEqual(["alice", "nobody"]);

    const admins = orderGroups(
      groupsBy("isAdmin"),
      { id: "isAdmin", label: () => "Admin", formatValue: (value) => (value ? "yes" : "no") },
      "en",
    );
    expect(admins.map((group) => group.label)).toEqual(["no", "yes"]);

    const created = orderGroups(
      groupsBy("created"),
      {
        id: "created",
        label: () => "Created",
        compareValues: (left, right) => (left as Date).getTime() - (right as Date).getTime(),
      },
      "en",
    );
    expect(created.map((group) => group.rows.map((row) => row.id))).toEqual([["2"], ["1"]]);
  });
});
