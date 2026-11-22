import IconAdjustmentsHorizontal from "@tabler/icons-react/icons/IconAdjustmentsHorizontal";
import IconSortAscending from "@tabler/icons-react/icons/IconSortAscending";
import IconSortDescending from "@tabler/icons-react/icons/IconSortDescending";
import { Dialog, DialogTrigger } from "react-aria-components/Dialog";

import { Button } from "@/components/ui/button";
import { columnLabel } from "@/components/ui/data-table/labels.tsx";
import { NO_GROUPING } from "@/components/ui/data-table/search.ts";
import { ListBoxItem } from "@/components/ui/list-box";
import { Popover } from "@/components/ui/popover";
import { Select } from "@/components/ui/select";
import { ToggleButton, ToggleButtonGroup } from "@/components/ui/toggle-button";
import { Tooltip, TooltipTrigger } from "@/components/ui/tooltip";
import { m } from "@/paraglide/messages.js";

import type { DataTableGroupingOption, DataTableTable } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";

import styles from "@/components/ui/data-table/data-table.module.css";

type DisplayPopoverProps<TData extends RowData> = {
  table: DataTableTable<TData>;
  groupingOptions: Array<DataTableGroupingOption>;
  grouping: string | undefined;
  onGroupingChange: (grouping: string | undefined) => void;
};

// Plane's Display menu: the grouping of the rows (a column, or none), their order (a column and
// its direction), and toggles for the properties that a row shows.
export function DisplayPopover<TData extends RowData>({
  table,
  groupingOptions,
  grouping,
  onGroupingChange,
}: DisplayPopoverProps<TData>) {
  const columns = table.getAllColumns();
  const sortColumns = columns
    .filter((column) => column.getCanSort() && column.columnDef.meta?.label)
    .map((column) => ({ id: column.id, label: columnLabel(column) }));
  const propertyColumns = columns.filter(
    (column) => column.columnDef.meta?.placement === "property" && column.getCanHide(),
  );
  const groupByItems = [
    { id: NO_GROUPING, label: m.dataTableGroupNone() },
    ...groupingOptions.map((option) => ({ id: option.id, label: option.label() })),
  ];
  const sort = table.state.sorting.at(0);
  const direction = sort?.desc ? m.dataTableDescending() : m.dataTableAscending();

  return (
    <DialogTrigger>
      <Button variant="secondary" icon={IconAdjustmentsHorizontal}>
        {m.dataTableDisplay()}
      </Button>
      <Popover placement="bottom end">
        <Dialog aria-label={m.dataTableDisplay()} className={styles.displayDialog}>
          {groupingOptions.length > 0 && (
            <Select
              label={m.dataTableGroupBy()}
              items={groupByItems}
              value={grouping ?? NO_GROUPING}
              onChange={(id) => {
                onGroupingChange(id === null || id === NO_GROUPING ? undefined : String(id));
              }}
            >
              {(item) => <ListBoxItem id={item.id}>{item.label}</ListBoxItem>}
            </Select>
          )}
          {sortColumns.length > 0 && (
            <div className={styles.orderBy}>
              <Select
                label={m.dataTableOrderBy()}
                placeholder={m.dataTableOrderDefault()}
                items={sortColumns}
                value={sort?.id ?? null}
                onChange={(id) => {
                  if (id !== null) {
                    table.setSorting([{ id: String(id), desc: sort?.desc ?? false }]);
                  }
                }}
              >
                {(column) => <ListBoxItem id={column.id}>{column.label}</ListBoxItem>}
              </Select>
              {/* The icon shows the direction, which the name says: pressing it reverses it. */}
              <TooltipTrigger>
                <Button
                  variant="secondary"
                  icon={sort?.desc ? IconSortDescending : IconSortAscending}
                  aria-label={direction}
                  isDisabled={!sort}
                  onPress={() => {
                    if (sort) {
                      table.setSorting([{ ...sort, desc: !sort.desc }]);
                    }
                  }}
                />
                <Tooltip>{direction}</Tooltip>
              </TooltipTrigger>
            </div>
          )}
          {propertyColumns.length > 0 && (
            <div className={styles.displaySection}>
              <span aria-hidden className={styles.displayHeading}>
                {m.dataTableProperties()}
              </span>
              <ToggleButtonGroup
                aria-label={m.dataTableProperties()}
                selectedKeys={propertyColumns
                  .filter((column) => column.getIsVisible())
                  .map((column) => column.id)}
                onSelectionChange={(keys) => {
                  for (const column of propertyColumns) {
                    column.toggleVisibility(keys.has(column.id));
                  }
                }}
              >
                {propertyColumns.map((column) => (
                  <ToggleButton key={column.id} id={column.id}>
                    {columnLabel(column)}
                  </ToggleButton>
                ))}
              </ToggleButtonGroup>
            </div>
          )}
        </Dialog>
      </Popover>
    </DialogTrigger>
  );
}
