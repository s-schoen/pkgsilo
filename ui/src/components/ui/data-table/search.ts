import { z } from "zod/v4";

import { EMPTY_FILTER_STATE } from "@/components/ui/data-table/filter-state.ts";

import type { DataTableFilterState } from "@/components/ui/data-table/types.ts";
import type { SortingState } from "@tanstack/react-table";

// The value of `group` that turns off a page's default grouping.
export const NO_GROUPING = "none";

// The router's parser turns a value such as "42" or "true" into a number or a boolean, and a
// repeated key into an array: a param takes each and gives a string.
const primitive = z.union([z.string(), z.number(), z.boolean()]);

const textParam = primitive.transform(String).optional().catch(undefined);

// A sort: a column's id, ascending, or "-" and the id, descending. An empty one turns off a
// page's default order.
const sortParam = primitive
  .transform(String)
  .pipe(z.string().regex(/^(?:-?[^-].*)?$/u))
  .optional()
  .catch(undefined);

// The values of a select filter, comma-joined, as "active,expired", without blanks and repeats.
// An empty one, "?status=", keeps no value where the page filters by default.
const listParam = z
  .union([primitive, z.array(primitive)])
  .transform((value) => joinList(splitList(Array.isArray(value) ? value.join(",") : String(value))))
  .optional()
  .catch(undefined);

function splitList(value: string): Array<string> {
  return [
    ...new Set(
      value
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  ];
}

function joinList(values: Array<string>): string {
  return values.join(",");
}

// The search params of a data table, with one param per select filter of `filters`.
export type DataTableSearch<TFilter extends string = string> = {
  q?: string;
  group?: string;
  sort?: string;
} & Partial<Record<TFilter, string>>;

// The schema of a data table's search params, for the route's validateSearch: `q` (the search),
// `group` (a column's id, or "none"), `sort` (a column's id, "-" before it for descending) and one
// param per select filter, whose values are comma-joined, so a value must not hold a comma. A
// value that doesn't parse falls back to none instead of failing the route. Spread its shape into
// a z.object to add the route's own params.
export function dataTableSearchSchema<const TFilter extends string>({
  filters,
}: {
  filters: ReadonlyArray<TFilter>;
}) {
  const filterParams = Object.fromEntries(filters.map((id) => [id, listParam])) as Record<
    TFilter,
    typeof listParam
  >;
  return z.object({ q: textParam, group: textParam, sort: sortParam, ...filterParams });
}

// What a page shows when its URL has no params: the filters it lists by, its order and its
// grouping. Only text and select filters of `filterState` count; text and number filters stay out
// of the URL.
export type DataTableSearchDefaults = {
  filterState?: DataTableFilterState;
  sorting?: SortingState;
  grouping?: string;
};

function sameValues(left: Array<string>, right: Array<string>): boolean {
  return left.length === right.length && left.every((value) => right.includes(value));
}

// The search and the select filters of the params, each param left out taking its default.
export function filterStateFromSearch<TFilter extends string>(
  search: DataTableSearch<TFilter>,
  filters: ReadonlyArray<TFilter>,
  defaults: DataTableFilterState = EMPTY_FILTER_STATE,
): DataTableFilterState {
  const selectFilters: Record<string, Array<string>> = {};
  for (const id of filters) {
    const param: string | undefined = search[id];
    const values = param === undefined ? (defaults.selectFilters[id] ?? []) : splitList(param);
    if (values.length > 0) {
      selectFilters[id] = values;
    }
  }
  return {
    globalFilter: search.q ?? defaults.globalFilter,
    selectFilters,
  };
}

// The params of the search and the select filters, leaving out each that equals its default.
export function searchFromFilterState<TFilter extends string>(
  state: DataTableFilterState,
  filters: ReadonlyArray<TFilter>,
  defaults: DataTableFilterState = EMPTY_FILTER_STATE,
): Record<string, string | undefined> {
  const search: Record<string, string | undefined> = {
    q: state.globalFilter === defaults.globalFilter ? undefined : state.globalFilter,
  };
  for (const id of filters) {
    const values = state.selectFilters[id] ?? [];
    const defaultValues = defaults.selectFilters[id] ?? [];
    search[id] = sameValues(values, defaultValues) ? undefined : joinList(values);
  }
  return search;
}

function sortParamOf(sorting: SortingState): string {
  const first = sorting.at(0);
  if (!first) {
    return "";
  }
  return first.desc ? `-${first.id}` : first.id;
}

// The order of the `sort` param, or the default without one. The table sorts by one column.
export function sortingFromSearch(
  search: DataTableSearch,
  defaults: SortingState = [],
): SortingState {
  const { sort } = search;
  if (sort === undefined) {
    return defaults;
  }
  if (!sort) {
    return [];
  }
  return sort.startsWith("-") ? [{ id: sort.slice(1), desc: true }] : [{ id: sort, desc: false }];
}

// The `sort` param of the order: none for the default, empty to turn the default off.
export function searchFromSorting(
  sorting: SortingState,
  defaults: SortingState = [],
): string | undefined {
  const sort = sortParamOf(sorting);
  return sort === sortParamOf(defaults) ? undefined : sort;
}

// The grouping column of the `group` param, or the default without one.
export function groupingFromSearch(search: DataTableSearch, defaults?: string): string | undefined {
  const { group } = search;
  if (group === undefined || group === "") {
    return defaults;
  }
  return group === NO_GROUPING ? undefined : group;
}

// The `group` param of the grouping: none for the default, "none" to turn the default off.
export function searchFromGrouping(
  grouping: string | undefined,
  defaults?: string,
): string | undefined {
  if (grouping === defaults) {
    return undefined;
  }
  return grouping ?? NO_GROUPING;
}
