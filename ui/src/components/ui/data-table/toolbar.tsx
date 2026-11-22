import { useRef, useState } from "react";

import { DisplayPopover } from "@/components/ui/data-table/display-popover.tsx";
import { FiltersPopover } from "@/components/ui/data-table/filters-popover.tsx";
import { columnLabel, optionIcon, text } from "@/components/ui/data-table/labels.tsx";
import { SearchField } from "@/components/ui/search-field";
import { Tag, TagGroup } from "@/components/ui/tag-group";
import { m } from "@/paraglide/messages.js";

import type {
  DataTableColumn,
  DataTableGroupingOption,
  DataTableOption,
  DataTableTable,
} from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";
import type { Key, ReactNode, RefObject } from "react";

import styles from "@/components/ui/data-table/data-table.module.css";

type ToolbarProps<TData extends RowData> = {
  table: DataTableTable<TData>;
  // The part for the selected rows, while there are any.
  selectionBar?: ReactNode;
  groupingOptions: Array<DataTableGroupingOption>;
  grouping: string | undefined;
  onGroupingChange: (grouping: string | undefined) => void;
};

// The bar above the rows, which sticks to the top of the window: the search, a chip for each
// active filter, the selected rows' actions, and the Filters and Display popovers.
export function Toolbar<TData extends RowData>({
  table,
  selectionBar,
  groupingOptions,
  grouping,
  onGroupingChange,
}: ToolbarProps<TData>) {
  const searchRef = useRef<HTMLInputElement>(null);
  const filterColumns = table
    .getAllColumns()
    .filter((column) => column.getCanFilter() && column.columnDef.meta?.filterVariant);

  return (
    <div className={styles.toolbar}>
      <div className={styles.search}>
        <ToolbarSearch
          ref={searchRef}
          value={table.state.globalFilter ?? ""}
          onChange={(value) => {
            table.setGlobalFilter(value);
          }}
        />
      </div>
      <div className={styles.chips}>
        <FilterChips table={table} columns={filterColumns} searchRef={searchRef} />
      </div>
      <div className={styles.actions}>
        {selectionBar}
        {filterColumns.length > 0 && <FiltersPopover table={table} columns={filterColumns} />}
        <DisplayPopover
          table={table}
          groupingOptions={groupingOptions}
          grouping={grouping}
          onGroupingChange={onGroupingChange}
        />
      </div>
    </div>
  );
}

type ToolbarSearchProps = {
  value: string;
  onChange: (value: string) => void;
  ref: RefObject<HTMLInputElement | null>;
};

// The search, bound to the table's global filter. The value often comes from the URL, which
// catches up only once the navigation of a keystroke completes: while the field is focused, it
// shows what the user typed instead, so that no keystroke made in the meantime is lost.
function ToolbarSearch({ value, onChange, ref }: ToolbarSearchProps) {
  const [draft, setDraft] = useState<string | null>(null);

  return (
    <SearchField
      ref={ref}
      variant="bare"
      aria-label={m.dataTableSearch()}
      placeholder={m.dataTableSearchPlaceholder()}
      value={draft ?? value}
      onChange={(next) => {
        setDraft(next);
        onChange(next);
      }}
      onBlur={() => {
        setDraft(null);
      }}
    />
  );
}

// A chip of an active filter: a value of a select column, or what a text or number column filters
// by. Its key names the column, and the value of a select column.
type Chip = {
  key: string;
  property: string;
  value: string;
  icon?: DataTableOption["icon"];
};

function chipKey(columnId: string, value?: string): string {
  return JSON.stringify(value === undefined ? [columnId] : [columnId, value]);
}

function chipsOf<TData extends RowData>(column: DataTableColumn<TData>): Array<Chip> {
  const meta = column.columnDef.meta;
  const property = columnLabel(column);
  const value = column.getFilterValue();

  if (meta?.filterVariant === "select") {
    const selected = Array.isArray(value) ? value.map(String) : [];
    return selected.map((selectedValue) => {
      const option = meta.options?.find((candidate) => candidate.value === selectedValue);
      return {
        key: chipKey(column.id, selectedValue),
        property,
        value: option ? text(option.label) : selectedValue,
        icon: option?.icon,
      };
    });
  }

  if (typeof value !== "string" || !value.trim()) {
    return [];
  }
  return [{ key: chipKey(column.id), property, value }];
}

type FilterChipsProps<TData extends RowData> = {
  table: DataTableTable<TData>;
  columns: Array<DataTableColumn<TData>>;
  searchRef: RefObject<HTMLInputElement | null>;
};

// A tag for each active filter, whose button, or Delete, clears that filter.
function FilterChips<TData extends RowData>({
  table,
  columns,
  searchRef,
}: FilterChipsProps<TData>) {
  const chips = columns.flatMap((column) => chipsOf(column));
  if (chips.length === 0) {
    return null;
  }

  // Drops the values of the removed chips from the column filters, all at once.
  const remove = (keys: Set<Key>) => {
    table.setColumnFilters((filters) =>
      filters.flatMap((filter) => {
        if (Array.isArray(filter.value)) {
          const kept = filter.value.filter((value) => !keys.has(chipKey(filter.id, String(value))));
          return kept.length > 0 ? [{ ...filter, value: kept }] : [];
        }
        return keys.has(chipKey(filter.id)) ? [] : [filter];
      }),
    );
    // The tags are gone with the last one, and the focus with them: it moves to the search.
    if (chips.every((chip) => keys.has(chip.key))) {
      searchRef.current?.focus();
    }
  };

  return (
    <TagGroup aria-label={m.dataTableActiveFilters()} onRemove={remove}>
      {chips.map((chip) => (
        <Tag key={chip.key} id={chip.key} property={chip.property} icon={optionIcon(chip.icon)}>
          {chip.value}
        </Tag>
      ))}
    </TagGroup>
  );
}
