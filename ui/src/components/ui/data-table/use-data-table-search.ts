import { useNavigate } from "@tanstack/react-router";
import { useMemo, useState } from "react";

import { EMPTY_FILTER_STATE } from "@/components/ui/data-table/filter-state.ts";
import {
  assertFilterParams,
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

// The filters that stay out of the URL: the text and number filters, and the select filters of the
// columns that `filters` doesn't list.
type LocalFilters = Required<Omit<DataTableFilterState, "globalFilter">>;

function localFiltersOf(state: DataTableFilterState, filters: ReadonlyArray<string>): LocalFilters {
  return {
    selectFilters: Object.fromEntries(
      Object.entries(state.selectFilters).filter(([id]) => !filters.includes(id)),
    ),
    textFilters: state.textFilters ?? {},
    numberFilters: state.numberFilters ?? {},
  };
}

// The state of the last change, while the URL it navigated from is still the route's search: the
// router updates the search only once the navigation completes, which a slow loader delays. A
// change made in the meantime builds on this one instead of on the search, which would undo it.
type PendingChange = {
  urlKey: string;
  state: DataTableFilterState;
};

// Keeps a data table's search, select filters, order and grouping in the route's search params,
// leaving out what equals `defaults`. `filters` are the select filters of the route's
// dataTableSearchSchema. A change replaces the history entry, keeping the route's other params, so
// Back leaves the page instead of undoing a keystroke. Text and number filters, and the select
// filters of columns that `filters` doesn't list, stay in the page's state.
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
  assertFilterParams(filters);
  const search = route.useSearch();
  const navigate = useNavigate();

  const defaultFilterState = defaults.filterState ?? EMPTY_FILTER_STATE;
  const defaultSorting = defaults.sorting ?? noSorting;
  const defaultGrouping = defaults.grouping;

  const [localFilters, setLocalFilters] = useState(() =>
    localFiltersOf(defaultFilterState, filters),
  );
  const [pending, setPending] = useState<PendingChange>();

  // The params as one key, so that the state keeps its identity while they don't change.
  const urlKey = JSON.stringify([search.q, ...filters.map((id) => search[id])]);
  const urlFilterState = useMemo(() => {
    const fromUrl = filterStateFromSearch(search, filters, defaultFilterState);
    return {
      ...fromUrl,
      ...localFilters,
      selectFilters: { ...localFilters.selectFilters, ...fromUrl.selectFilters },
    };
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- urlKey stands for the params of search and filters
  }, [urlKey, defaultFilterState, localFilters]);
  // Once the URL changes, by the change's navigation or another, it's the state again.
  if (pending !== undefined && pending.urlKey !== urlKey) {
    setPending(undefined);
  }
  const filterState = pending?.urlKey === urlKey ? pending.state : urlFilterState;

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
      setLocalFilters(localFiltersOf(next, filters));
      setPending({ urlKey, state: next });
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
