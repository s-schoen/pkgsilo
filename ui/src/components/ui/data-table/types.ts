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
import type { LinkProps } from "react-aria-components/Link";

// A route of the app, as React Aria's links take it in href.
export type Href = NonNullable<LinkProps["href"]>;

// A message of paraglide, such as m.status, which the table calls as it renders, so that a change
// of locale renders the text anew.
export type Message = () => string;

// How the Filters popover filters a column: by a choice of its options, by text or by a number.
// Without a filterFn of its own, the column keeps a row whose value is one of the checked options
// (compared as text; a list value when one of its items is), contains the text (ignoring case), or
// equals the number.
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

// What a property's preview card shows below the column's label, and what a click on its chip does.
// Every property chip of a column with a label has a card that names the column; without
// preview.href or preview.onAction, a click on the chip opens the row, as a click beside it does.
export type DataTablePreview<TData extends RowData> = {
  // More about the row's value, such as its owner's name and picture. It renders only while the
  // card is open, so it may load what it shows.
  content?: (row: TData) => ReactNode;
  // A click or Enter on the chip follows this link instead of opening the row, such as to the
  // owner's page; a modifier key opens it in a new tab. A row without one (undefined), such as
  // one whose owner the actor can't read, falls back to onAction, else opens the row.
  href?: (row: TData) => Href | undefined;
  // A click or Enter on the chip runs this instead of opening the row.
  onAction?: (row: TData) => void;
};

// A column of a data table: TanStack's ColumnDef with the table's features and meta, and the
// preview of its property chips. The preview sits beside meta, not in it, since TanStack's meta
// doesn't know the row's type.
export type DataTableColumnDef<
  TData extends RowData,
  TValue extends CellData = CellData,
> = ColumnDef<DataTableFeatures, TData, TValue> & { preview?: DataTablePreview<TData> };

export type DataTableColumn<TData extends RowData, TValue extends CellData = CellData> = Column<
  DataTableFeatures,
  TData,
  TValue
>;

export type DataTableRow<TData extends RowData> = Row<DataTableFeatures, TData>;

export type DataTableTable<TData extends RowData> = ReactTable<DataTableFeatures, TData>;

// A column that the Display popover's Group by offers. The table groups by the column's value, or
// by its getGroupingValue when that differs from what the cell shows (two assets of the same name,
// say): the groups stay apart even when their labels are equal. The functions below get that value
// as the column gives it, a Date or null as well as text.
export type DataTableGroupingOption = {
  // The column's id, as TanStack gives it: its id, else its accessor key with "_" for each "."
  // ("owner.name" is "owner_name").
  id: string;
  // The column's name in the Display popover: a message.
  label: Message;
  // A group's label from its value; the value as text when it's missing.
  formatValue?: (value: unknown) => string;
  // A group's icon from its value, such as the status's icon.
  renderIcon?: (value: unknown) => ReactNode;
  // Orders the groups by their values; else they're ordered by their labels, in the locale's
  // collation.
  compareValues?: (left: unknown, right: unknown) => number;
};

// An action on the selected rows, besides delete, such as revoking API keys: the toolbar offers it
// while one of the selected rows qualifies.
export type DataTableBulkAction<TData extends RowData> = {
  // The button's text: a message.
  label: Message;
  icon: TablerIcon;
  // The rows the action can change; every row when it's missing.
  appliesTo?: (row: TData) => boolean;
  // Runs on the selected rows that the action applies to, and on no other. It reports its outcome
  // itself, such as with a toast; while its promise runs, its button is pending, and if it rejects,
  // the rows stay selected.
  onAction: (rows: Array<TData>) => Promise<void> | void;
};
