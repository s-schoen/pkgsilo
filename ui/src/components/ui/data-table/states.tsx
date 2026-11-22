import IconInbox from "@tabler/icons-react/icons/IconInbox";
import IconSearchOff from "@tabler/icons-react/icons/IconSearchOff";

import { Button } from "@/components/ui/button";
import { m } from "@/paraglide/messages.js";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactNode } from "react";

import styles from "@/components/ui/data-table/data-table.module.css";

// The rows while they load: placeholders in the rows' shape, which a screen reader skips, in a
// grid that says it's busy.
export function LoadingRows({ label }: { label: string }) {
  return (
    <div role="grid" aria-label={label} aria-busy="true" className={styles.rows}>
      {[0, 1, 2].map((index) => (
        <div key={index} aria-hidden className={`${styles.row} ${styles.skeletonRow}`}>
          <span className={styles.skeleton} />
        </div>
      ))}
    </div>
  );
}

export type EmptyStateProps = {
  icon: TablerIcon;
  // One short line: a message.
  message: string;
  // At most one button.
  children?: ReactNode;
};

// What the table shows instead of rows: an icon, a line, and the button that helps, if any. A page
// passes one as the table's emptyState, as DataTableEmptyState, to offer the button that creates
// the first item.
export function EmptyState({ icon: Icon, message, children }: EmptyStateProps) {
  return (
    <div className={styles.empty}>
      <Icon aria-hidden className={styles.emptyIcon} />
      <p className={styles.emptyMessage}>{message}</p>
      {children}
    </div>
  );
}

// No rows at all.
export function NothingYet() {
  return <EmptyState icon={IconInbox} message={m.dataTableEmpty()} />;
}

// Rows, but none that pass the filters: the button restores the default filters.
export function NoMatches({ onClear }: { onClear: () => void }) {
  return (
    <EmptyState icon={IconSearchOff} message={m.dataTableNoMatches()}>
      <Button variant="secondary" onPress={onClear}>
        {m.dataTableClearFilters()}
      </Button>
    </EmptyState>
  );
}
