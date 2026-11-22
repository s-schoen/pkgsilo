import { functionalUpdate, useTable } from "@tanstack/react-table";
import { useTanStackTableDevtools } from "@tanstack/react-table-devtools";
import { useMemo, useRef, useState } from "react";

import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { withVariantFilterFns } from "@/components/ui/data-table/filter-fns.ts";
import {
  EMPTY_FILTER_STATE,
  filterVariants,
  fromColumnFilters,
  hasActiveFilters,
  isSameFilterState,
  toColumnFilters,
} from "@/components/ui/data-table/filter-state.ts";
import { orderGroups } from "@/components/ui/data-table/grouping.ts";
import { rowMenuItems } from "@/components/ui/data-table/row-menu-items.ts";
import { RowMenu } from "@/components/ui/data-table/row-menu.tsx";
import { rowText } from "@/components/ui/data-table/row-text.ts";
import { GroupedRows, Rows } from "@/components/ui/data-table/rows.tsx";
import { SelectionBar } from "@/components/ui/data-table/selection-bar.tsx";
import { LoadingRows, NoMatches, NothingYet } from "@/components/ui/data-table/states.tsx";
import { Toolbar } from "@/components/ui/data-table/toolbar.tsx";
import { dataTableFeatures } from "@/components/ui/data-table/types.ts";
import { m } from "@/paraglide/messages.js";
import { getLocale } from "@/paraglide/runtime.js";

import type { RowMenuItems } from "@/components/ui/data-table/row-menu-items.ts";
import type {
  DataTableBulkAction,
  DataTableColumnDef,
  DataTableFeatures,
  DataTableFilterState,
  DataTableGroupingOption,
  Href,
} from "@/components/ui/data-table/types.ts";
import type { ContextMenuPoint } from "@/components/ui/menu";
import type {
  ColumnVisibilityState,
  RowData,
  RowSelectionState,
  SortingState,
} from "@tanstack/react-table";
import type { KeyboardEvent, MouseEvent, ReactNode } from "react";
import type { Selection } from "react-aria-components/GridList";

import styles from "@/components/ui/data-table/data-table.module.css";

const noSorting: SortingState = [];
const allVisible: ColumnVisibilityState = {};
const noBulkActions: Array<never> = [];
const noSelection: RowSelectionState = {};
const noGroupingOptions: Array<never> = [];

// A row's context menu: the rows it acts on, its name and its items, which it keeps while it
// closes.
type OpenMenu<TData extends RowData> = {
  rows: Array<TData>;
  label: string;
  items: RowMenuItems<TData>;
  point: ContextMenuPoint | undefined;
};

// Shift+F10 or the context-menu key, which open the focused row's menu.
function isContextMenuKey(event: KeyboardEvent) {
  return (event.shiftKey && event.key === "F10") || event.key === "ContextMenu";
}

export type DataTableProps<TData extends RowData> = {
  // The name of the list, as a screen reader announces it: a message.
  label: string;
  // The columns, which place their cells in the row (meta.placement) and filter, sort or hide.
  // Keep them stable, as a constant of the module. A column with meta.filterVariant and no
  // filterFn filters by its variant's function (filter-fns.ts).
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
  // The columns that the Display popover's Group by offers, besides None. An option whose id
  // names no column of the table isn't offered.
  groupingOptions?: Array<DataTableGroupingOption>;
  // The id of the grouping option the rows are grouped by, or none. With onGroupingChange, it
  // controls the grouping; without, it's the grouping the table starts with. An id that isn't
  // among groupingOptions groups by nothing.
  grouping?: string;
  onGroupingChange?: (grouping: string | undefined) => void;
  // What the table shows without rows; a line with an icon by default.
  emptyState?: ReactNode;
  // Actions on the selected rows, such as revoking API keys. Rows are selectable only when there's
  // something to do with them: bulkActions or onDelete. While one runs, its button is pending, and
  // no other action or delete starts.
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
  groupingOptions = noGroupingOptions,
  grouping: groupingProp,
  onGroupingChange,
}: DataTableProps<TData>) {
  const [localSorting, setLocalSorting] = useState(initialSorting);
  const [rowSelection, setRowSelection] = useState(noSelection);
  const [gridSelection, setGridSelection] = useState<Selection>(() => new Set());
  // The rows that the delete dialog asks about, which it keeps while it closes.
  const [deleting, setDeleting] = useState<Array<TData>>([]);
  const [isDeleteOpen, setDeleteOpen] = useState(false);
  const rowsRef = useRef<HTMLDivElement>(null);
  const [menu, setMenu] = useState<OpenMenu<TData>>();
  const [isMenuOpen, setMenuOpen] = useState(false);
  // The row that the menu opened on, which has the focus and gets it back.
  const menuRowRef = useRef<HTMLElement>(null);
  const isSelectable = Boolean(onDelete) || bulkActions.length > 0;
  const [localFilterState, setLocalFilterState] = useState(defaultFilterState);
  const [columnVisibility, setColumnVisibility] = useState(initialColumnVisibility);
  const [localGrouping, setLocalGrouping] = useState(groupingProp);
  // The groups the user collapsed, by their row's id, which holds the column and the value: a
  // group that comes later starts expanded.
  const [collapsedGroups, setCollapsedGroups] = useState<ReadonlySet<string>>(() => new Set());
  // The label of the bulk action that runs, from the selection bar or a row's menu.
  const [runningAction, setRunningAction] = useState<string>();

  const sorting = controlledSorting ?? localSorting;
  const groupingId = onGroupingChange ? groupingProp : localGrouping;
  const grouping = useMemo(
    () =>
      groupingId !== undefined && groupingOptions.some((option) => option.id === groupingId)
        ? [groupingId]
        : [],
    [groupingOptions, groupingId],
  );
  const filterState = controlledFilterState ?? localFilterState;
  const columnFilters = useMemo(() => toColumnFilters(filterState), [filterState]);
  const resolvedColumns = useMemo(() => withVariantFilterFns(columns), [columns]);

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
    columns: resolvedColumns,
    getRowId: (row) => getRowId(row),
    // The grouped column stays in its place in the rows, instead of moving to the front.
    groupedColumnMode: false,
    state: {
      grouping,
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
    // The filters by the ids that TanStack gave the columns, which the changes name.
    onColumnFiltersChange: (updater) => {
      changeFilterState(
        fromColumnFilters(
          filterState,
          functionalUpdate(updater, columnFilters),
          filterVariants(table.getAllLeafColumns()),
        ),
      );
    },
    globalFilterFn: "includesString",
  });
  useTanStackTableDevtools(table);

  // The grouping options whose column the table has, by the id TanStack gave it: TanStack ignores
  // a grouping by any other id, and leaves the rows as they are.
  const columnsById = table.getAllFlatColumnsById();
  const availableGroupingOptions = groupingOptions.filter((option) =>
    Object.hasOwn(columnsById, option.id),
  );
  const groupingOption = availableGroupingOptions.find((option) => option.id === groupingId);

  // Without grouping, the rows in their order. With it, the groups, whose rows the grouped row
  // model holds, sorted in their order too: the table shows them in its own Tree, so it doesn't
  // need TanStack's expanded row model.
  const visibleRows = table.getPreExpandedRowModel().rows;
  const groups = groupingOption ? orderGroups(visibleRows, groupingOption, getLocale()) : [];

  const changeGrouping = (next: string | undefined) => {
    if (onGroupingChange) {
      onGroupingChange(next);
    } else {
      setLocalGrouping(next);
    }
  };

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

  // Runs a bulk action, unless one runs already. A rejection leaves the rows selected: onAction has
  // reported it, as ConfirmDialog's onConfirm does.
  const runAction = async (action: DataTableBulkAction<TData>, targets: Array<TData>) => {
    if (runningAction !== undefined) {
      return;
    }
    setRunningAction(action.label());
    try {
      await action.onAction(targets);
    } catch {
      // onAction has reported the error, such as with a toast; the user can try again.
    } finally {
      setRunningAction(undefined);
    }
  };

  const askDelete = (targets: Array<TData>) => {
    setDeleting(targets);
    setDeleteOpen(true);
  };

  // Opens the context menu of the row that holds target, unless it isn't a row (a group) or there's
  // nothing to do with it. On a selected row, the menu acts on the selection; on another one, it
  // selects only that row (while rows are selectable) and acts on it alone.
  const openRowMenu = (target: EventTarget, point?: ContextMenuPoint) => {
    const element = target instanceof Element ? target.closest<HTMLElement>("[data-row-id]") : null;
    const row = filteredRows.find((candidate) => candidate.id === element?.dataset.rowId);
    if (!element || !row) {
      return false;
    }
    const isSelected = isSelectable && Object.hasOwn(rowSelection, row.id);
    const targets = isSelected ? selectedRows : [row];
    const items = rowMenuItems(
      targets.map((selected) => selected.original),
      { getRowHref, onRowAction, bulkActions, canDelete: Boolean(onDelete) },
    );
    if (!items) {
      return false;
    }
    if (isSelectable && !isSelected) {
      changeSelection(new Set([row.id]));
    }
    // The menu gives the focus back to the element that had it: the row, or its link.
    if (!element.contains(document.activeElement)) {
      element.focus();
    }
    menuRowRef.current = element;
    setMenu({
      rows: targets.map((selected) => selected.original),
      label:
        targets.length > 1
          ? m.dataTableSelected({ count: targets.length })
          : m.dataTableRowActions({ name: rowText(row) }),
      items,
      point,
    });
    setMenuOpen(true);
    return true;
  };

  // A right-click opens the menu at the pointer. A contextmenu event of the keyboard, which some
  // browsers send with a point outside the row, opens it at the row.
  const onContextMenu = (event: MouseEvent) => {
    const row = event.target instanceof Element ? event.target.closest("[data-row-id]") : null;
    const rect = row?.getBoundingClientRect();
    const isInRow =
      rect !== undefined &&
      event.clientX >= rect.left &&
      event.clientX <= rect.right &&
      event.clientY >= rect.top &&
      event.clientY <= rect.bottom;
    if (openRowMenu(event.target, isInRow ? { x: event.clientX, y: event.clientY } : undefined)) {
      event.preventDefault();
    }
  };

  const onKeyDown = (event: KeyboardEvent) => {
    if (isContextMenuKey(event) && openRowMenu(event.target)) {
      // Keeps the browser from sending its own contextmenu event, and its menu.
      event.preventDefault();
      event.stopPropagation();
    }
  };

  function body() {
    if (isLoading) {
      return <LoadingRows label={label} />;
    }
    if (groupingOption && groups.length > 0) {
      return (
        <GroupedRows
          label={label}
          groups={groups}
          renderIcon={groupingOption.renderIcon}
          collapsedGroups={collapsedGroups}
          onCollapsedGroupsChange={setCollapsedGroups}
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
        groupingOptions={availableGroupingOptions}
        grouping={groupingOption?.id}
        onGroupingChange={changeGrouping}
        selectionBar={
          selectedRows.length > 0 && (
            <SelectionBar<TData>
              rows={selectedRows.map((row) => row.original)}
              bulkActions={bulkActions}
              runningAction={runningAction}
              onAction={runAction}
              onDelete={onDelete && askDelete}
              onClear={clearSelection}
            />
          )
        }
      />
      {/* Catches the context menu's pointer and keys of the rows inside, which name themselves
          with data-row-id; the rows' grid stays the element that takes the focus. */}
      <div className={styles.body} onContextMenu={onContextMenu} onKeyDownCapture={onKeyDown}>
        {body()}
      </div>
      {menu && (
        <RowMenu
          isOpen={isMenuOpen}
          onOpenChange={setMenuOpen}
          triggerRef={menuRowRef}
          point={menu.point}
          label={menu.label}
          rows={menu.rows}
          items={menu.items}
          onAction={runAction}
          onDelete={askDelete}
        />
      )}
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
