import IconFilter from "@tabler/icons-react/icons/IconFilter";
import { Dialog, DialogTrigger } from "react-aria-components/Dialog";

import { Button } from "@/components/ui/button";
import { columnLabel, optionIcon, text } from "@/components/ui/data-table/labels.tsx";
import { FilterList, FilterListItem, FilterListSection } from "@/components/ui/filter-list";
import { Popover } from "@/components/ui/popover";
import { TextField } from "@/components/ui/text-field";
import { m } from "@/paraglide/messages.js";

import type { DataTableColumn, DataTableTable } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";
import type { Selection } from "react-aria-components";

import styles from "@/components/ui/data-table/data-table.module.css";

// The key of a select column's option in the filter list: option values repeat across columns.
function optionKey(columnId: string, value: string): string {
  return JSON.stringify([columnId, value]);
}

function selectedValues<TData extends RowData>(column: DataTableColumn<TData>): Array<string> {
  const value = column.getFilterValue();
  return Array.isArray(value) ? value.map(String) : [];
}

// How many filters narrow the rows: each value of a select column, and each text or number column
// with a value. The search isn't one.
function countActiveFilters<TData extends RowData>(columns: Array<DataTableColumn<TData>>): number {
  return columns.reduce((count, column) => {
    const value = column.getFilterValue();
    if (Array.isArray(value)) {
      return count + value.length;
    }
    return typeof value === "string" && value.trim() ? count + 1 : count;
  }, 0);
}

type FiltersPopoverProps<TData extends RowData> = {
  table: DataTableTable<TData>;
  // The columns with a filter variant.
  columns: Array<DataTableColumn<TData>>;
};

// Plane's filter menu: a button with the count of active filters, which opens one searchable list
// of every select column's options, then a field for each text or number column.
export function FiltersPopover<TData extends RowData>({
  table,
  columns,
}: FiltersPopoverProps<TData>) {
  const selectColumns = columns.filter(
    (column) => column.columnDef.meta?.filterVariant === "select",
  );
  const scalarColumns = columns.filter(
    (column) => column.columnDef.meta?.filterVariant !== "select",
  );
  const activeCount = countActiveFilters(columns);

  const selectedKeys = new Set(
    selectColumns.flatMap((column) =>
      selectedValues(column).map((value) => optionKey(column.id, value)),
    ),
  );

  // Replaces the filters of every select column with the checked options, at once.
  const changeSelection = (selection: Selection) => {
    const next = new Map(selectColumns.map((column) => [column.id, [] as Array<string>]));
    for (const column of selectColumns) {
      for (const option of column.columnDef.meta?.options ?? []) {
        if (selection === "all" || selection.has(optionKey(column.id, option.value))) {
          next.get(column.id)?.push(option.value);
        }
      }
    }
    table.setColumnFilters((filters) => [
      ...filters.filter((filter) => !next.has(filter.id)),
      ...[...next].flatMap(([id, value]) => (value.length > 0 ? [{ id, value }] : [])),
    ]);
  };

  return (
    <DialogTrigger>
      <Button variant="secondary" icon={IconFilter}>
        {activeCount > 0 ? m.dataTableFiltersCount({ count: activeCount }) : m.dataTableFilters()}
      </Button>
      <Popover placement="bottom end">
        <Dialog aria-label={m.dataTableFilters()} className={styles.filtersDialog}>
          {selectColumns.length > 0 && (
            <FilterList
              aria-label={m.dataTableFilterOptions()}
              searchLabel={m.dataTableSearchFilters()}
              placeholder={m.dataTableFilterBy()}
              // oxlint-disable-next-line jsx-a11y/no-autofocus -- The user opened the popover to filter: its search takes the focus, as WAI-ARIA's combobox-in-dialog pattern does.
              autoFocus
              selectedKeys={selectedKeys}
              onSelectionChange={changeSelection}
            >
              {selectColumns.map((column) => (
                <FilterListSection key={column.id} title={columnLabel(column)}>
                  {(column.columnDef.meta?.options ?? []).map((option) => (
                    <FilterListItem
                      key={option.value}
                      id={optionKey(column.id, option.value)}
                      icon={optionIcon(option.icon)}
                    >
                      {text(option.label)}
                    </FilterListItem>
                  ))}
                </FilterListSection>
              ))}
            </FilterList>
          )}
          {scalarColumns.length > 0 && (
            <div className={styles.scalarFilters}>
              {scalarColumns.map((column) => {
                const value = column.getFilterValue();
                return (
                  <TextField
                    key={column.id}
                    label={columnLabel(column)}
                    inputMode={
                      column.columnDef.meta?.filterVariant === "number" ? "decimal" : undefined
                    }
                    value={typeof value === "string" ? value : ""}
                    onChange={(next) => {
                      column.setFilterValue(next.trim() ? next : undefined);
                    }}
                  />
                );
              })}
            </div>
          )}
        </Dialog>
      </Popover>
    </DialogTrigger>
  );
}
