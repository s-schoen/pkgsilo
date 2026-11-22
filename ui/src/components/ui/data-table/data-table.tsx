import { functionalUpdate, useTable } from "@tanstack/react-table";
import { useTanStackTableDevtools } from "@tanstack/react-table-devtools";
import { useMemo, useRef, useState } from "react";

import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import {
  EMPTY_FILTER_STATE,
  filterVariants,
  fromColumnFilters,
  hasActiveFilters,
  isSameFilterState,
  toColumnFilters,
} from "@/components/ui/data-table/filter-state.ts";
import { Rows } from "@/components/ui/data-table/rows.tsx";
import { SelectionBar } from "@/components/ui/data-table/selection-bar.tsx";
import { LoadingRows, NoMatches, NothingYet } from "@/components/ui/data-table/states.tsx";
import { Toolbar } from "@/components/ui/data-table/toolbar.tsx";
import { dataTableFeatures } from "@/components/ui/data-table/types.ts";
import { m } from "@/paraglide/messages.js";

import type { Href } from "@/components/ui/data-table/rows.tsx";
import type {
  DataTableBulkAction,
  DataTableColumnDef,
  DataTableFeatures,
  DataTableFilterState,
} from "@/components/ui/data-table/types.ts";
import type {
  ColumnVisibilityState,
  RowData,
  RowSelectionState,
  SortingState,
} from "@tanstack/react-table";
import type { ReactNode } from "react";
import type { Selection } from "react-aria-components/GridList";

import styles from "@/components/ui/data-table/data-table.module.css";

const noSorting: SortingState = [];
const allVisible: ColumnVisibilityState = {};
const noBulkActions: Array<never> = [];
const noSelection: RowSelectionState = {};

export type DataTableProps<TData extends RowData> = {
  // The name of the list, as a screen reader announces it: a message.
  label: string;
  // The columns, which place their cells in the row (meta.placement) and filter, sort or hide.
  // Keep them stable, as a constant of the module.
  columns: Array<DataTableColumnDef<TData>>;
  // Every row, as useLiveQuery returns them: the table filters and sorts them itself.
  rows: Array<TData>;
  // While the rows load: the table shows placeholders.
  isLoading?: boolean;
  // The row's id, which keeps its focus (and its selection) while the rows change.
  getRowId: (row: TData) => string;
  // Makes the row's title a link to this route, which also opens in a new tab.
  getRowHref?: (row: TData) => Href;
  // Opens the row on a click or Enter, as a peek: instead of following getRowHref.
  onRowAction?: (row: TData) => void;
  // Tints the row that's open, as the one of a peek.
  isRowActive?: (row: TData) => boolean;
  // The order the rows start in; sorting with onSortingChange controls it instead.
  initialSorting?: SortingState;
  sorting?: SortingState;
  onSortingChange?: (sorting: SortingState) => void;
  // The properties hidden at first, as { owner: false }.
  initialColumnVisibility?: ColumnVisibilityState;
  // The search and the filters; filterState with onFilterStateChange controls them, else they
  // start at defaultFilterState.
  filterState?: DataTableFilterState;
  onFilterStateChange?: (state: DataTableFilterState) => void;
  // The filters the page opens with, when they define what it lists: rows that none of them match
  // show emptyState, and Clear filters goes back to them.
  defaultFilterState?: DataTableFilterState;
  // What the table shows without rows; a line with an icon by default.
  emptyState?: ReactNode;
  // Actions on the selected rows, such as revoking API keys. Rows are selectable only when there's
  // something to do with them: bulkActions or onDelete.
  bulkActions?: Array<DataTableBulkAction<TData>>;
  // Deletes the selected rows once the user confirms it, the last of the selection's actions. It
  // reports its outcome itself, such as with a toast; while its promise runs, the dialog waits,
  // and if it rejects, the dialog stays open.
  onDelete?: (rows: Array<TData>) => Promise<void> | void;
};

// A page's list, as Plane's: a toolbar with the search, a chip for each filter, and the Filters and
// Display popovers, then a row for each item, with its leading icon, its title and its properties.
// TanStack Table holds the rows and every state (the search, the filters, the order, the shown
// properties); React Aria's GridList renders the rows. It fills its page from edge to edge, as
// AppLayout's page frame leaves room for, and its toolbar sticks to the top of the window.
export function DataTable<TData extends RowData>({
  label,
  columns,
  rows,
  isLoading = false,
  getRowId,
  getRowHref,
  onRowAction,
  isRowActive,
  initialSorting = noSorting,
  sorting: controlledSorting,
  onSortingChange,
  initialColumnVisibility = allVisible,
  filterState: controlledFilterState,
  onFilterStateChange,
  defaultFilterState = EMPTY_FILTER_STATE,
  emptyState,
  bulkActions = noBulkActions,
  onDelete,
}: DataTableProps<TData>) {
  const [localSorting, setLocalSorting] = useState(initialSorting);
  const [rowSelection, setRowSelection] = useState(noSelection);
  const [gridSelection, setGridSelection] = useState<Selection>(() => new Set());
  // The rows that the delete dialog asks about, which it keeps while it closes.
  const [deleting, setDeleting] = useState<Array<TData>>([]);
  const [isDeleteOpen, setDeleteOpen] = useState(false);
  const rowsRef = useRef<HTMLDivElement>(null);
  const isSelectable = Boolean(onDelete) || bulkActions.length > 0;
  const [localFilterState, setLocalFilterState] = useState(defaultFilterState);
  const [columnVisibility, setColumnVisibility] = useState(initialColumnVisibility);

  const sorting = controlledSorting ?? localSorting;
  const filterState = controlledFilterState ?? localFilterState;
  const variants = useMemo(() => filterVariants(columns), [columns]);
  const columnFilters = useMemo(() => toColumnFilters(filterState), [filterState]);

  const changeFilterState = (next: DataTableFilterState) => {
    if (controlledFilterState === undefined) {
      setLocalFilterState(next);
    }
    onFilterStateChange?.(next);
  };

  const table = useTable<DataTableFeatures, TData>({
    key: label,
    features: dataTableFeatures,
    data: rows,
    columns,
    getRowId: (row) => getRowId(row),
    state: {
      sorting,
      columnVisibility,
      globalFilter: filterState.globalFilter,
      columnFilters,
      rowSelection,
    },
    enableRowSelection: isSelectable,
    onRowSelectionChange: setRowSelection,
    onSortingChange: (updater) => {
      const next = functionalUpdate(updater, sorting);
      if (controlledSorting === undefined) {
        setLocalSorting(next);
      }
      onSortingChange?.(next);
    },
    onColumnVisibilityChange: setColumnVisibility,
    onGlobalFilterChange: (updater) => {
      const next: unknown = functionalUpdate(updater, filterState.globalFilter);
      changeFilterState({ ...filterState, globalFilter: typeof next === "string" ? next : "" });
    },
    onColumnFiltersChange: (updater) => {
      changeFilterState(
        fromColumnFilters(filterState, functionalUpdate(updater, columnFilters), variants),
      );
    },
    globalFilterFn: "includesString",
  });
  useTanStackTableDevtools(table);

  const visibleRows = table.getRowModel().rows;

  // Only rows that the data and the filters keep stay selected: a row that leaves them leaves the
  // selection too, and doesn't come back with it.
  const filteredRows = table.getFilteredRowModel().rows;
  const selectedRows = filteredRows.filter((row) => Object.hasOwn(rowSelection, row.id));
  if (selectedRows.length !== Object.keys(rowSelection).length) {
    setRowSelection(Object.fromEntries(selectedRows.map((row) => [row.id, true])));
  }
  // GridList's own selection, as long as it holds the same rows: it keeps the anchor that
  // Shift+click selects from, which a new set would lose.
  const selectedKeys =
    gridSelection !== "all" &&
    gridSelection.size === selectedRows.length &&
    selectedRows.every((row) => gridSelection.has(row.id))
      ? gridSelection
      : new Set(selectedRows.map((row) => row.id));

  // GridList's "all", as Ctrl/Cmd+A selects it, is every row that the filters keep.
  const changeSelection = (keys: Selection) => {
    setGridSelection(keys);
    const ids = keys === "all" ? filteredRows.map((row) => row.id) : [...keys].map(String);
    table.setRowSelection(Object.fromEntries(ids.map((id) => [id, true])));
  };

  // Ends the selection and moves the focus from the toolbar's part, which goes with it, to the rows.
  const clearSelection = () => {
    table.resetRowSelection();
    rowsRef.current?.focus();
  };

  function body() {
    if (isLoading) {
      return <LoadingRows label={label} />;
    }
    if (visibleRows.length > 0) {
      return (
        <Rows
          label={label}
          rows={visibleRows}
          getRowHref={getRowHref}
          onRowAction={onRowAction}
          isRowActive={isRowActive}
          isSelectable={isSelectable}
          selectedKeys={selectedKeys}
          onSelectionChange={changeSelection}
          ref={rowsRef}
        />
      );
    }
    // The filters hide every row, unless they are the page's own: then the page has nothing.
    const isFiltered =
      rows.length > 0 &&
      hasActiveFilters(filterState) &&
      !isSameFilterState(filterState, defaultFilterState);
    if (isFiltered) {
      return (
        <NoMatches
          onClear={() => {
            changeFilterState(defaultFilterState);
          }}
        />
      );
    }
    return emptyState ?? <NothingYet />;
  }

  return (
    <div className={styles.dataTable}>
      <Toolbar
        table={table}
        selectionBar={
          selectedRows.length > 0 && (
            <SelectionBar<TData>
              rows={selectedRows.map((row) => row.original)}
              bulkActions={bulkActions}
              onDelete={
                onDelete &&
                ((targets) => {
                  setDeleting(targets);
                  setDeleteOpen(true);
                })
              }
              onClear={clearSelection}
            />
          )
        }
      />
      {body()}
      {onDelete && (
        <ConfirmDialog
          isOpen={isDeleteOpen}
          onOpenChange={setDeleteOpen}
          title={m.dataTableDeleteTitle({ count: deleting.length })}
          description={m.dataTableDeleteDescription()}
          confirmLabel={m.dataTableDelete()}
          onConfirm={async () => {
            await onDelete(deleting);
            table.resetRowSelection();
          }}
        />
      )}
    </div>
  );
}
