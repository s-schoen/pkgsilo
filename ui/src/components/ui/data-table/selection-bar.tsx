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
  // The label of the bulk action that runs, whose button is pending; while one runs, the other
  // actions and delete are disabled.
  runningAction: string | undefined;
  // Runs an action on the rows it applies to.
  onAction: (action: DataTableBulkAction<TData>, rows: Array<TData>) => Promise<void>;
  // Asks before it deletes the rows; missing when the page deletes nothing.
  onDelete: ((rows: Array<TData>) => void) | undefined;
  onClear: () => void;
};

// The toolbar's part for the selected rows: how many there are, a button that clears the selection,
// the bulk actions that apply to one of them, and delete last.
export function SelectionBar<TData extends RowData>({
  rows,
  bulkActions,
  runningAction,
  onAction,
  onDelete,
  onClear,
}: SelectionBarProps<TData>) {
  const isRunning = runningAction !== undefined;

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
      {applicableActions(bulkActions, rows).map(({ action, rows: applicable }) => {
        const label = action.label();
        return (
          <Button
            key={label}
            variant="tertiary"
            icon={action.icon}
            isPending={runningAction === label}
            isDisabled={isRunning && runningAction !== label}
            onPress={() => {
              void onAction(action, applicable);
            }}
          >
            {label}
          </Button>
        );
      })}
      {onDelete && (
        <Button
          variant="danger-outline"
          icon={IconTrash}
          isDisabled={isRunning}
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
