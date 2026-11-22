import {
  columnFilteringFeature,
  columnGroupingFeature,
  columnVisibilityFeature,
  createExpandedRowModel,
  createFilteredRowModel,
  createGroupedRowModel,
  createSortedRowModel,
  filterFn_includesString,
  globalFilteringFeature,
  metaHelper,
  rowExpandingFeature,
  rowSelectionFeature,
  rowSortingFeature,
  sortFn_alphanumeric,
  sortFn_basic,
  sortFn_datetime,
  sortFn_text,
  tableFeatures,
} from "@tanstack/react-table";

import type { TablerIcon } from "@tabler/icons-react";
import type { CellData, Column, ColumnDef, ReactTable, Row, RowData } from "@tanstack/react-table";
import type { ReactNode } from "react";

// A message of paraglide, such as m.status, which the table calls as it renders, so that a change
// of locale renders the text anew.
export type Message = () => string;

// How the Filters popover filters a column: by a choice of its options, by text or by a number.
export type FilterVariant = "select" | "text" | "number";

// Where a row shows a column's cell: before the title (an icon), as the title, or as a property
// chip at the end. A column without a placement only filters, sorts or groups.
export type DataTablePlacement = "leading" | "title" | "property";

// A value that a select column filters by.
export type DataTableOption = {
  value: string;
  // A message, or a string for data such as a group's name, which needs no translation.
  label: Message | string;
  // Shown before the label: an icon of @tabler/icons-react, or a node such as a colored dot.
  icon?: TablerIcon | ReactNode;
};

// What a column tells the table besides TanStack's own options.
export type DataTableColumnMeta = {
  // The column's name in the Filters and Display popovers.
  label?: Message;
  placement?: DataTablePlacement;
  filterVariant?: FilterVariant;
  // The choices of a select column.
  options?: Array<DataTableOption>;
};

// The state of the toolbar's search and the Filters popover. A select filter holds the values it
// keeps, a text or number filter what the user typed.
export type DataTableFilterState = {
  globalFilter: string;
  selectFilters: Partial<Record<string, Array<string>>>;
  textFilters?: Partial<Record<string, string>>;
  numberFilters?: Partial<Record<string, string>>;
};

// The features of every data table: TanStack Table v9 bundles only what a table names, so this
// list is what the data table costs. Filtering, sorting and grouping run on the client, since a
// collection holds every row already.
export const dataTableFeatures = tableFeatures({
  columnFilteringFeature,
  columnGroupingFeature,
  columnVisibilityFeature,
  globalFilteringFeature,
  rowExpandingFeature,
  rowSelectionFeature,
  rowSortingFeature,
  expandedRowModel: createExpandedRowModel(),
  filteredRowModel: createFilteredRowModel(),
  groupedRowModel: createGroupedRowModel(),
  sortedRowModel: createSortedRowModel(),
  // The search matches a row when one of its cells contains the text, ignoring case.
  filterFns: {
    includesString: filterFn_includesString,
  },
  // The "auto" sort function picks one of these by the column's values.
  sortFns: {
    alphanumeric: sortFn_alphanumeric,
    basic: sortFn_basic,
    datetime: sortFn_datetime,
    text: sortFn_text,
  },
  columnMeta: metaHelper<DataTableColumnMeta>(),
});

export type DataTableFeatures = typeof dataTableFeatures;

// A column of a data table: TanStack's ColumnDef with the table's features and meta.
export type DataTableColumnDef<
  TData extends RowData,
  TValue extends CellData = CellData,
> = ColumnDef<DataTableFeatures, TData, TValue>;

export type DataTableColumn<TData extends RowData, TValue extends CellData = CellData> = Column<
  DataTableFeatures,
  TData,
  TValue
>;

export type DataTableRow<TData extends RowData> = Row<DataTableFeatures, TData>;

export type DataTableTable<TData extends RowData> = ReactTable<DataTableFeatures, TData>;
