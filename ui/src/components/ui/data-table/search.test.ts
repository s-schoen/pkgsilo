import { defaultParseSearch, defaultStringifySearch } from "@tanstack/react-router";
import { describe, expect, it } from "vitest";

import {
  dataTableSearchSchema,
  filterStateFromSearch,
  groupingFromSearch,
  searchFromFilterState,
  searchFromGrouping,
  searchFromSorting,
  sortingFromSearch,
} from "@/components/ui/data-table/search.ts";

import type { DataTableFilterState } from "@/components/ui/data-table/types.ts";

const filters = ["status", "owner"] as const;
const schema = dataTableSearchSchema({ filters });

// Parses a query string as the router does, then with the schema.
function parse(query: string) {
  return schema.parse(defaultParseSearch(query));
}

const listsActive: DataTableFilterState = {
  globalFilter: "",
  selectFilters: { status: ["active"] },
};

describe("dataTableSearchSchema", () => {
  it("parses every param", () => {
    expect(parse("?q=ci&group=owner&sort=-created&status=active,expired&owner=alice")).toEqual({
      q: "ci",
      group: "owner",
      sort: "-created",
      status: "active,expired",
      owner: "alice",
    });
  });

  it("leaves out the params that are missing", () => {
    expect(parse("")).toEqual({});
  });

  it("takes the numbers and booleans that the router's parser makes as text", () => {
    expect(parse("?q=42&status=true")).toEqual({ q: "42", status: "true" });
  });

  it("joins a repeated key and drops blanks and repeats of a list", () => {
    expect(schema.parse({ status: ["active", "expired,active"], owner: " ,alice,, bob " })).toEqual(
      {
        status: "active,expired",
        owner: "alice,bob",
      },
    );
  });

  it("keeps an empty param, which turns off a default", () => {
    expect(parse("?status=&sort=")).toEqual({ status: "", sort: "" });
  });

  it("falls back to none for a value that doesn't parse", () => {
    expect(schema.parse({ q: { a: 1 }, sort: "-", group: [], status: { b: 2 } })).toEqual({});
    expect(schema.parse({ sort: "--name" })).toEqual({});
  });

  it("ignores the params of other filters", () => {
    expect(parse("?kind=x")).toEqual({});
  });

  it.each(["q", "group", "sort"])("rejects a filter named %s, a param of the table", (id) => {
    expect(() => dataTableSearchSchema({ filters: [id] })).toThrow(`"${id}"`);
  });
});

describe("the filter state", () => {
  it("reads the search and the select filters", () => {
    expect(filterStateFromSearch({ q: "ci", status: "active,expired" }, filters)).toEqual({
      globalFilter: "ci",
      selectFilters: { status: ["active", "expired"] },
    });
  });

  it("takes a filter's default without its param, and no value from an empty one", () => {
    expect(filterStateFromSearch({}, filters, listsActive)).toEqual(listsActive);
    expect(filterStateFromSearch({ status: "" }, filters, listsActive)).toEqual({
      globalFilter: "",
      selectFilters: {},
    });
  });

  it("leaves the defaults out of the params", () => {
    expect(
      searchFromFilterState(
        { globalFilter: "", selectFilters: { status: ["active"] }, textFilters: { name: "x" } },
        filters,
        listsActive,
      ),
    ).toEqual({ q: undefined, status: undefined, owner: undefined });
  });

  it("writes an empty param when the user clears a default", () => {
    expect(
      searchFromFilterState({ globalFilter: "", selectFilters: {} }, filters, listsActive),
    ).toEqual({ q: undefined, status: "", owner: undefined });
  });

  it.each<[string, DataTableFilterState, DataTableFilterState | undefined]>([
    ["nothing", { globalFilter: "", selectFilters: {} }, undefined],
    [
      "search and filters",
      { globalFilter: "ci bot", selectFilters: { status: ["expired", "revoked"], owner: ["a"] } },
      undefined,
    ],
    ["the default", listsActive, listsActive],
    ["a cleared default", { globalFilter: "", selectFilters: {} }, listsActive],
    [
      "another value than the default",
      { globalFilter: "x", selectFilters: { status: ["revoked"] } },
      listsActive,
    ],
  ])("survives the round trip through the URL: %s", (_, state, defaults) => {
    const query = defaultStringifySearch(searchFromFilterState(state, filters, defaults));
    expect(filterStateFromSearch(parse(query), filters, defaults)).toEqual(state);
  });
});

describe("the order", () => {
  it("reads a column, descending with a minus", () => {
    expect(sortingFromSearch({ sort: "name" })).toEqual([{ id: "name", desc: false }]);
    expect(sortingFromSearch({ sort: "-name" })).toEqual([{ id: "name", desc: true }]);
  });

  it("takes the default without a param, and none from an empty one", () => {
    const defaults = [{ id: "created", desc: true }];
    expect(sortingFromSearch({}, defaults)).toBe(defaults);
    expect(sortingFromSearch({ sort: "" }, defaults)).toEqual([]);
  });

  it("leaves the default out and writes an empty param when the user turns it off", () => {
    const defaults = [{ id: "created", desc: true }];
    expect(searchFromSorting([{ id: "created", desc: true }], defaults)).toBeUndefined();
    expect(searchFromSorting([], defaults)).toBe("");
    expect(searchFromSorting([], [])).toBeUndefined();
    expect(searchFromSorting([{ id: "created", desc: false }], defaults)).toBe("created");
  });

  it.each([
    [[], []],
    [[{ id: "name", desc: false }], []],
    [[{ id: "name", desc: true }], [{ id: "created", desc: true }]],
    [[], [{ id: "created", desc: true }]],
  ])("survives the round trip through the URL: %j with the default %j", (sorting, defaults) => {
    const query = defaultStringifySearch({ sort: searchFromSorting(sorting, defaults) });
    expect(sortingFromSearch(parse(query), defaults)).toEqual(sorting);
  });
});

describe("the grouping", () => {
  it("reads a column, none for 'none', and the default without a param", () => {
    expect(groupingFromSearch({ group: "owner" })).toBe("owner");
    expect(groupingFromSearch({ group: "none" }, "status")).toBeUndefined();
    expect(groupingFromSearch({}, "status")).toBe("status");
  });

  it.each([
    [undefined, undefined],
    ["owner", undefined],
    ["status", "status"],
    [undefined, "status"],
    ["owner", "status"],
  ])("survives the round trip through the URL: %s with the default %s", (grouping, defaults) => {
    const query = defaultStringifySearch({ group: searchFromGrouping(grouping, defaults) });
    expect(groupingFromSearch(parse(query), defaults)).toBe(grouping);
  });

  it("leaves the default out of the URL", () => {
    expect(searchFromGrouping("status", "status")).toBeUndefined();
    expect(searchFromGrouping(undefined, "status")).toBe("none");
  });
});
