import type {
  DataTableColumn,
  DataTableFilterState,
  FilterVariant,
} from "@/components/ui/data-table/types.ts";
import type { ColumnFiltersState, RowData } from "@tanstack/react-table";

// No search and no filters.
export const EMPTY_FILTER_STATE: DataTableFilterState = {
  globalFilter: "",
  selectFilters: {},
  textFilters: {},
  numberFilters: {},
};

// The filter variant of each column that has one, by the id that TanStack gave the column: a nested
// accessor key such as "owner.name" becomes "owner_name", and a column without an id or accessor
// key takes its header's text.
export function filterVariants<TData extends RowData>(
  columns: Array<DataTableColumn<TData>>,
): Map<string, FilterVariant> {
  const variants = new Map<string, FilterVariant>();
  for (const column of columns) {
    const variant = column.columnDef.meta?.filterVariant;
    if (variant) {
      variants.set(column.id, variant);
    }
  }
  return variants;
}

// The column filters of a filter state, without the empty ones: a select filter without values,
// or a text or number filter of only whitespace.
export function toColumnFilters(state: DataTableFilterState): ColumnFiltersState {
  return [
    ...Object.entries(state.selectFilters).flatMap(([id, value]) =>
      Array.isArray(value) && value.length > 0 ? [{ id, value }] : [],
    ),
    ...Object.entries({ ...state.textFilters, ...state.numberFilters }).flatMap(([id, value]) =>
      typeof value === "string" && value.trim() ? [{ id, value }] : [],
    ),
  ];
}

// The filter state of TanStack's column filters, keeping the search of the current state. A filter
// of a column without a variant, or with a value that doesn't fit its variant, is dropped.
export function fromColumnFilters(
  current: DataTableFilterState,
  columnFilters: ColumnFiltersState,
  variants: Map<string, FilterVariant>,
): DataTableFilterState {
  const selectFilters: Record<string, Array<string>> = {};
  const textFilters: Record<string, string> = {};
  const numberFilters: Record<string, string> = {};

  for (const { id, value } of columnFilters) {
    const variant = variants.get(id);
    if (variant === "select") {
      if (Array.isArray(value) && value.length > 0) {
        selectFilters[id] = value.map(String);
      }
    } else if (variant && typeof value === "string" && value.trim()) {
      (variant === "text" ? textFilters : numberFilters)[id] = value;
    }
  }

  return { ...current, selectFilters, textFilters, numberFilters };
}

// Whether the state narrows the rows at all.
export function hasActiveFilters(state: DataTableFilterState): boolean {
  return state.globalFilter.length > 0 || toColumnFilters(state).length > 0;
}

// A key that two states share when they narrow the rows the same way.
function filterKey(state: DataTableFilterState): string {
  const filters = toColumnFilters(state)
    .map(({ id, value }) => [id, Array.isArray(value) ? value.map(String).toSorted() : value])
    .toSorted(([left], [right]) => String(left).localeCompare(String(right)));
  return JSON.stringify([state.globalFilter, filters]);
}

// Whether two filter states narrow the rows the same way, ignoring empty filters and the order of
// filters and values: the table counts a state equal to its default as unfiltered.
export function isSameFilterState(
  left: DataTableFilterState,
  right: DataTableFilterState,
): boolean {
  return filterKey(left) === filterKey(right);
}
