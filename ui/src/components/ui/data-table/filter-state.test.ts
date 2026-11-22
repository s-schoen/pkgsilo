import { useTable } from "@tanstack/react-table";
import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  EMPTY_FILTER_STATE,
  filterVariants,
  fromColumnFilters,
  hasActiveFilters,
  isSameFilterState,
  toColumnFilters,
} from "@/components/ui/data-table/filter-state.ts";
import { dataTableFeatures } from "@/components/ui/data-table/types.ts";

import type { DataTableColumnDef, DataTableFilterState } from "@/components/ui/data-table/types.ts";

function filters(overrides: Partial<DataTableFilterState>): DataTableFilterState {
  return { ...EMPTY_FILTER_STATE, ...overrides };
}

type Item = { id: string; name: string; status: string; size: number };

const columns: Array<DataTableColumnDef<Item>> = [
  { accessorKey: "name", meta: { placement: "title", filterVariant: "text" } },
  {
    id: "status",
    accessorFn: (item) => item.status,
    meta: { filterVariant: "select" },
    filterFn: (row, columnId, values: Array<string>) => values.includes(row.getValue(columnId)),
  },
  { accessorKey: "size", meta: { filterVariant: "number" } },
  { accessorKey: "id" },
];

describe("filterVariants", () => {
  it("maps the columns with a variant by their id or accessor key", () => {
    expect(filterVariants(columns)).toEqual(
      new Map([
        ["name", "text"],
        ["status", "select"],
        ["size", "number"],
      ]),
    );
  });

  it("skips a column without an id", () => {
    const anonymous: DataTableColumnDef<Item> = {
      header: "Size",
      meta: { filterVariant: "number" },
    };

    expect(filterVariants([anonymous])).toEqual(new Map());
  });
});

describe("toColumnFilters", () => {
  it("converts each kind of filter", () => {
    expect(
      toColumnFilters(
        filters({
          globalFilter: "web",
          selectFilters: { status: ["active", "revoked"] },
          textFilters: { name: "ci" },
          numberFilters: { size: "3" },
        }),
      ),
    ).toEqual([
      { id: "status", value: ["active", "revoked"] },
      { id: "name", value: "ci" },
      { id: "size", value: "3" },
    ]);
  });

  it("drops empty filters", () => {
    expect(
      toColumnFilters(
        filters({
          selectFilters: { status: [], kind: undefined },
          textFilters: { name: "  " },
          numberFilters: { size: "" },
        }),
      ),
    ).toEqual([]);
  });

  it("gives the column filters that TanStack filters the rows by", () => {
    const { result } = renderHook(() =>
      useTable({
        features: dataTableFeatures,
        columns,
        data: [
          { id: "1", name: "ci", status: "active", size: 1 },
          { id: "2", name: "deploy", status: "revoked", size: 2 },
          { id: "3", name: "backup", status: "expired", size: 3 },
        ],
        getRowId: (item) => item.id,
        initialState: {
          columnFilters: toColumnFilters(
            filters({ selectFilters: { status: ["active", "expired"] } }),
          ),
        },
      }),
    );

    expect(result.current.getRowModel().rows.map((row) => row.id)).toEqual(["1", "3"]);
  });
});

describe("fromColumnFilters", () => {
  const variants = filterVariants(columns);

  it("converts back what toColumnFilters gave, keeping the search", () => {
    const state = filters({
      globalFilter: "web",
      selectFilters: { status: ["active"] },
      textFilters: { name: "ci" },
      numberFilters: { size: "3" },
    });

    expect(fromColumnFilters(state, toColumnFilters(state), variants)).toEqual(state);
  });

  it("replaces the filters of the current state", () => {
    const current = filters({ globalFilter: "web", selectFilters: { status: ["active"] } });

    expect(fromColumnFilters(current, [{ id: "name", value: "ci" }], variants)).toEqual(
      filters({ globalFilter: "web", textFilters: { name: "ci" } }),
    );
  });

  it.each([
    ["a column without a variant", { id: "id", value: "1" }],
    ["an unknown column", { id: "owner", value: ["me"] }],
    ["an empty select filter", { id: "status", value: [] }],
    ["a select filter that isn't a list", { id: "status", value: "active" }],
    ["a blank text filter", { id: "name", value: " " }],
    ["a text filter that isn't text", { id: "name", value: 3 }],
    ["a blank number filter", { id: "size", value: "" }],
  ])("drops %s", (_, filter) => {
    expect(fromColumnFilters(EMPTY_FILTER_STATE, [filter], variants)).toEqual(EMPTY_FILTER_STATE);
  });

  it("turns select values into strings", () => {
    expect(
      fromColumnFilters(EMPTY_FILTER_STATE, [{ id: "status", value: [1, 2] }], variants),
    ).toEqual(filters({ selectFilters: { status: ["1", "2"] } }));
  });
});

describe("hasActiveFilters", () => {
  it.each([
    ["no filters", EMPTY_FILTER_STATE, false],
    [
      "only empty filters",
      filters({ selectFilters: { status: [] }, textFilters: { name: " " } }),
      false,
    ],
    ["a search", filters({ globalFilter: "web" }), true],
    ["a select filter", filters({ selectFilters: { status: ["active"] } }), true],
    ["a text filter", filters({ textFilters: { name: "ci" } }), true],
    ["a number filter", filters({ numberFilters: { size: "3" } }), true],
  ])("tells whether %s narrow the rows", (_, state, active) => {
    expect(hasActiveFilters(state)).toBe(active);
  });
});

describe("isSameFilterState", () => {
  it("ignores empty filters and the order of filters and values", () => {
    expect(
      isSameFilterState(
        filters({ selectFilters: { status: ["a", "b"], severity: ["high"], assignee: [] } }),
        filters({
          selectFilters: { severity: ["high"], status: ["b", "a"] },
          textFilters: { x: " " },
        }),
      ),
    ).toBe(true);
  });

  it("takes a state without text and number filters as one with empty ones", () => {
    expect(isSameFilterState({ globalFilter: "", selectFilters: {} }, EMPTY_FILTER_STATE)).toBe(
      true,
    );
  });

  it.each([
    ["a different value", filters({ selectFilters: { status: ["b"] } })],
    ["an extra filter", filters({ selectFilters: { status: ["a"], severity: ["high"] } })],
    ["a search", filters({ globalFilter: "web", selectFilters: { status: ["a"] } })],
    ["a text filter", filters({ selectFilters: { status: ["a"] }, textFilters: { name: "ci" } })],
    ["no filters", EMPTY_FILTER_STATE],
  ])("tells %s apart", (_, other) => {
    expect(isSameFilterState(filters({ selectFilters: { status: ["a"] } }), other)).toBe(false);
  });
});
