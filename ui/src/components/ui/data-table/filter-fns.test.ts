import { useTable } from "@tanstack/react-table";
import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { withVariantFilterFns } from "@/components/ui/data-table/filter-fns.ts";
import { dataTableFeatures } from "@/components/ui/data-table/types.ts";

import type { DataTableColumnDef } from "@/components/ui/data-table/types.ts";
import type { ColumnFiltersState } from "@tanstack/react-table";

type Item = {
  id: string;
  name: string;
  status: string | null;
  scopes: number;
  tags: Array<string>;
  // A count that the API gives as text.
  size: string;
};

const items: Array<Item> = [
  { id: "1", name: "CI publish", status: "active", scopes: 3, tags: ["ci"], size: "10" },
  { id: "2", name: "deploy", status: "inactive", scopes: 1, tags: ["ci", "prod"], size: "20" },
  { id: "3", name: "backup", status: "expired", scopes: 3, tags: [], size: "10" },
  { id: "4", name: "legacy", status: null, scopes: 5, tags: ["prod"], size: "" },
];

const columns: Array<DataTableColumnDef<Item>> = [
  { accessorKey: "name", meta: { filterVariant: "text" } },
  { accessorKey: "status", meta: { filterVariant: "select" } },
  // Numbers, which an option's value names as text.
  { id: "scopeCount", accessorFn: (item) => item.scopes, meta: { filterVariant: "select" } },
  { accessorKey: "tags", meta: { filterVariant: "select" } },
  { accessorKey: "scopes", meta: { filterVariant: "number" } },
  { accessorKey: "size", meta: { filterVariant: "number" } },
  // A number that the text filter searches as text.
  { id: "scopeText", accessorFn: (item) => item.scopes, meta: { filterVariant: "text" } },
];

// The ids of the rows that the column filters keep.
function filteredIds(
  columnFilters: ColumnFiltersState,
  definitions: Array<DataTableColumnDef<Item>> = columns,
) {
  const { result } = renderHook(() =>
    useTable({
      features: dataTableFeatures,
      columns: withVariantFilterFns(definitions),
      data: items,
      getRowId: (item) => item.id,
      initialState: { columnFilters },
    }),
  );
  return result.current.getRowModel().rows.map((row) => row.id);
}

describe("a select filter", () => {
  it("keeps the rows whose value is one of the checked options", () => {
    expect(filteredIds([{ id: "status", value: ["active", "expired"] }])).toEqual(["1", "3"]);
  });

  it("matches the whole value, not a part of it", () => {
    expect(filteredIds([{ id: "status", value: ["active"] }])).toEqual(["1"]);
  });

  it("compares a number as text", () => {
    expect(filteredIds([{ id: "scopeCount", value: ["3"] }])).toEqual(["1", "3"]);
  });

  it("keeps a row whose list holds one of the options", () => {
    expect(filteredIds([{ id: "tags", value: ["prod"] }])).toEqual(["2", "4"]);
  });

  it("keeps no row without a value", () => {
    expect(filteredIds([{ id: "status", value: ["null", ""] }])).toEqual([]);
  });
});

describe("a number filter", () => {
  it("keeps the rows whose value is the number", () => {
    expect(filteredIds([{ id: "scopes", value: "3" }])).toEqual(["1", "3"]);
  });

  it("reads a value that's text as a number", () => {
    expect(filteredIds([{ id: "size", value: "10.0" }])).toEqual(["1", "3"]);
  });

  it("keeps no row for text that isn't a number", () => {
    expect(filteredIds([{ id: "scopes", value: "three" }])).toEqual([]);
  });
});

describe("a text filter", () => {
  it("keeps the rows that contain the text, ignoring case", () => {
    expect(filteredIds([{ id: "name", value: "ci" }])).toEqual(["1"]);
  });

  it("searches a number as text", () => {
    expect(filteredIds([{ id: "scopeText", value: "5" }])).toEqual(["4"]);
  });
});

describe("withVariantFilterFns", () => {
  it("keeps a column's own filter function", () => {
    const atLeast: DataTableColumnDef<Item> = {
      accessorKey: "scopes",
      meta: { filterVariant: "number" },
      filterFn: (row, _columnId, minimum: string) => row.original.scopes >= Number(minimum),
    };

    expect(filteredIds([{ id: "scopes", value: "3" }], [atLeast])).toEqual(["1", "3", "4"]);
  });

  it("gives the columns of a group their variant's function", () => {
    const grouped: Array<DataTableColumnDef<Item>> = [
      { id: "details", columns: [{ accessorKey: "status", meta: { filterVariant: "select" } }] },
    ];

    expect(filteredIds([{ id: "status", value: ["expired"] }], grouped)).toEqual(["3"]);
  });

  it("leaves a column without a variant as it is", () => {
    const plain: DataTableColumnDef<Item> = { accessorKey: "name" };

    expect(withVariantFilterFns([plain])).toEqual([plain]);
  });
});
