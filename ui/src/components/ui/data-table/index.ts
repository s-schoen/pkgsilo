// The public API of the data table: other code imports it as @/components/ui/data-table.
export { DataTable } from "@/components/ui/data-table/data-table.tsx";
export type { DataTableProps } from "@/components/ui/data-table/data-table.tsx";
export { EmptyState as DataTableEmptyState } from "@/components/ui/data-table/states.tsx";
export type { EmptyStateProps as DataTableEmptyStateProps } from "@/components/ui/data-table/states.tsx";
export type {
  DataTableBulkAction,
  DataTableColumn,
  DataTableColumnDef,
  DataTableColumnMeta,
  DataTableFilterState,
  DataTableGroupingOption,
  DataTableOption,
  DataTablePlacement,
  DataTablePreview,
  DataTableRow,
  DataTableTable,
  FilterVariant,
} from "@/components/ui/data-table/types.ts";
export { dataTableSearchSchema } from "@/components/ui/data-table/search.ts";
export type {
  DataTableSearch,
  DataTableSearchDefaults,
} from "@/components/ui/data-table/search.ts";
export { useDataTableSearch } from "@/components/ui/data-table/use-data-table-search.ts";
export type { DataTableSearchProps } from "@/components/ui/data-table/use-data-table-search.ts";
