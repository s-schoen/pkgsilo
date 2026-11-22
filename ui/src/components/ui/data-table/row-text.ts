import type { DataTableRow } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";

// What typeahead matches and a screen reader announces of a row: its title's value.
export function rowText<TData extends RowData>(row: DataTableRow<TData>): string {
  const titleCell = row
    .getVisibleCells()
    .find((cell) => cell.column.columnDef.meta?.placement === "title");
  const value = titleCell?.getValue();
  return typeof value === "string" || typeof value === "number" ? String(value) : row.id;
}
