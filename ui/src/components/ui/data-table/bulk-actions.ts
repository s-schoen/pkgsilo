import type { DataTableBulkAction } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";

// A bulk action with the rows it acts on.
export type ApplicableAction<TData extends RowData> = {
  action: DataTableBulkAction<TData>;
  rows: Array<TData>;
};

// The bulk actions that apply to one of the rows at least, each with the rows it applies to, in the
// order the page gave them.
export function applicableActions<TData extends RowData>(
  actions: Array<DataTableBulkAction<TData>>,
  rows: Array<TData>,
): Array<ApplicableAction<TData>> {
  return actions.flatMap((action) => {
    const applicable = action.appliesTo ? rows.filter(action.appliesTo) : rows;
    return applicable.length > 0 ? [{ action, rows: applicable }] : [];
  });
}
