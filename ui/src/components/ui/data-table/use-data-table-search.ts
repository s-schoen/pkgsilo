import { useNavigate } from "@tanstack/react-router";
import { useMemo, useState } from "react";

import { EMPTY_FILTER_STATE } from "@/components/ui/data-table/filter-state.ts";
import {
  filterStateFromSearch,
  groupingFromSearch,
  searchFromFilterState,
  searchFromGrouping,
  searchFromSorting,
  sortingFromSearch,
} from "@/components/ui/data-table/search.ts";

import type {
  DataTableSearch,
  DataTableSearchDefaults,
} from "@/components/ui/data-table/search.ts";
import type { DataTableFilterState } from "@/components/ui/data-table/types.ts";
import type { SortingState } from "@tanstack/react-table";

const noSorting: SortingState = [];

// A route whose validateSearch holds dataTableSearchSchema({ filters }), as the Route of a route
// file.
type DataTableSearchRoute<TFilter extends string> = {
  useSearch: () => DataTableSearch<TFilter>;
};

// The table's props that the URL controls, to spread into DataTable.
export type DataTableSearchProps = {
  filterState: DataTableFilterState;
  onFilterStateChange: (state: DataTableFilterState) => void;
  defaultFilterState: DataTableFilterState;
  sorting: SortingState;
  onSortingChange: (sorting: SortingState) => void;
  grouping: string | undefined;
  onGroupingChange: (grouping: string | undefined) => void;
};

// Keeps a data table's search, select filters, order and grouping in the route's search params,
// leaving out what equals `defaults`. `filters` are the select filters of the route's
// dataTableSearchSchema. A change replaces the history entry, keeping the route's other params, so
// Back leaves the page instead of undoing a keystroke. Text and number filters stay in the page's
// state.
export function useDataTableSearch<const TFilter extends string>(
  route: DataTableSearchRoute<TFilter>,
  {
    filters,
    defaults = {},
  }: {
    filters: ReadonlyArray<TFilter>;
    defaults?: DataTableSearchDefaults;
  },
): DataTableSearchProps {
  const search = route.useSearch();
  const navigate = useNavigate();
  const [localFilters, setLocalFilters] = useState<
    Pick<DataTableFilterState, "textFilters" | "numberFilters">
  >({});

  const defaultFilterState = defaults.filterState ?? EMPTY_FILTER_STATE;
  const defaultSorting = defaults.sorting ?? noSorting;
  const defaultGrouping = defaults.grouping;

  // The params as one key, so that the state keeps its identity while they don't change.
  const filterKey = JSON.stringify([search.q, ...filters.map((id) => search[id])]);
  const filterState = useMemo(
    () => ({
      ...filterStateFromSearch(search, filters, defaultFilterState),
      ...localFilters,
    }),
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- filterKey stands for the params of search and filters
    [filterKey, defaultFilterState, localFilters],
  );
  const sorting = useMemo(
    () => sortingFromSearch(search, defaultSorting),
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- only the sort param matters
    [search.sort, defaultSorting],
  );

  // Merges the params into the URL, without scrolling the page to the top.
  function update(params: Record<string, string | undefined>) {
    void navigate({
      to: ".",
      // The route is generic, so its search can't be typed here.
      search: ((previous: Record<string, unknown>) => ({ ...previous, ...params })) as never,
      replace: true,
      resetScroll: false,
    });
  }

  return {
    filterState,
    onFilterStateChange: (next) => {
      setLocalFilters({ textFilters: next.textFilters, numberFilters: next.numberFilters });
      update(searchFromFilterState(next, filters, defaultFilterState));
    },
    defaultFilterState,
    sorting,
    onSortingChange: (next) => {
      update({ sort: searchFromSorting(next, defaultSorting) });
    },
    grouping: groupingFromSearch(search, defaultGrouping),
    onGroupingChange: (next) => {
      update({ group: searchFromGrouping(next, defaultGrouping) });
    },
  };
}
