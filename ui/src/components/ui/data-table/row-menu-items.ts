import { applicableActions } from "@/components/ui/data-table/bulk-actions.ts";

import type { ApplicableAction } from "@/components/ui/data-table/bulk-actions.ts";
import type { DataTableBulkAction, Href } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";

// What a row's context menu offers for the rows it acts on.
export type RowMenuItems<TData extends RowData> = {
  // Open and Open in new tab, for a single row.
  open: (() => void) | undefined;
  href: Href | undefined;
  actions: Array<ApplicableAction<TData>>;
  // The rows that Delete deletes; none when the menu has no Delete.
  deletable: Array<TData>;
};

type RowMenuOptions<TData extends RowData> = {
  getRowHref: ((row: TData) => Href) | undefined;
  onRowAction: ((row: TData) => void) | undefined;
  bulkActions: Array<DataTableBulkAction<TData>>;
  // Picks the rows that Delete deletes, none when the page deletes nothing.
  deletableRows: (rows: Array<TData>) => Array<TData>;
};

// The items of the menu on these rows, or none when there's nothing to do with them: then the
// browser shows its own menu.
export function rowMenuItems<TData extends RowData>(
  rows: Array<TData>,
  { getRowHref, onRowAction, bulkActions, deletableRows }: RowMenuOptions<TData>,
): RowMenuItems<TData> | undefined {
  const single = rows.length === 1 ? rows[0] : undefined;
  const items = {
    open: single !== undefined && onRowAction ? () => onRowAction(single) : undefined,
    href: single !== undefined ? getRowHref?.(single) : undefined,
    actions: applicableActions(bulkActions, rows),
    deletable: deletableRows(rows),
  };
  const isEmpty =
    !items.open && !items.href && items.actions.length === 0 && items.deletable.length === 0;
  return isEmpty ? undefined : items;
}
