import IconTrash from "@tabler/icons-react/icons/IconTrash";
import IconX from "@tabler/icons-react/icons/IconX";

import { Button } from "@/components/ui/button";
import { applicableActions } from "@/components/ui/data-table/bulk-actions.ts";
import { Tooltip, TooltipTrigger } from "@/components/ui/tooltip";
import { m } from "@/paraglide/messages.js";

import type { DataTableBulkAction } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";

import styles from "@/components/ui/data-table/data-table.module.css";

type SelectionBarProps<TData extends RowData> = {
  // The selected rows, of which there's one at least.
  rows: Array<TData>;
  bulkActions: Array<DataTableBulkAction<TData>>;
  // Asks before it deletes the rows; missing when the page deletes nothing.
  onDelete: ((rows: Array<TData>) => void) | undefined;
  onClear: () => void;
};

// The toolbar's part for the selected rows: how many there are, a button that clears the selection,
// the bulk actions that apply to one of them, and delete last.
export function SelectionBar<TData extends RowData>({
  rows,
  bulkActions,
  onDelete,
  onClear,
}: SelectionBarProps<TData>) {
  return (
    <div className={styles.selectionBar}>
      <span className={styles.selectionCount}>{m.dataTableSelected({ count: rows.length })}</span>
      <TooltipTrigger>
        <Button
          variant="ghost"
          icon={IconX}
          aria-label={m.dataTableClearSelection()}
          onPress={onClear}
        />
        <Tooltip>{m.dataTableClearSelection()}</Tooltip>
      </TooltipTrigger>
      {applicableActions(bulkActions, rows).map(({ action, rows: applicable }) => (
        <Button
          key={action.label()}
          variant="tertiary"
          icon={action.icon}
          onPress={() => {
            void action.onAction(applicable);
          }}
        >
          {action.label()}
        </Button>
      ))}
      {onDelete && (
        <Button
          variant="danger-outline"
          icon={IconTrash}
          onPress={() => {
            onDelete(rows);
          }}
        >
          {m.dataTableDelete()}
        </Button>
      )}
    </div>
  );
}
