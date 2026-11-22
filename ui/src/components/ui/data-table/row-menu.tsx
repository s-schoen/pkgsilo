import IconArrowRight from "@tabler/icons-react/icons/IconArrowRight";
import IconExternalLink from "@tabler/icons-react/icons/IconExternalLink";
import IconTrash from "@tabler/icons-react/icons/IconTrash";

import { ContextMenu, MenuItem, MenuSection, MenuSeparator } from "@/components/ui/menu";
import { m } from "@/paraglide/messages.js";

import type { RowMenuItems } from "@/components/ui/data-table/row-menu-items.ts";
import type { ContextMenuPoint } from "@/components/ui/menu";
import type { RowData } from "@tanstack/react-table";
import type { ReactNode, RefObject } from "react";

type RowMenuProps<TData extends RowData> = {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  // The row the menu opened on, which has the focus.
  triggerRef: RefObject<HTMLElement | null>;
  point: ContextMenuPoint | undefined;
  // The menu's name: the row's for one row, the count for several, which its header shows too.
  label: string;
  rows: Array<TData>;
  items: RowMenuItems<TData>;
  // Asks before it deletes the rows.
  onDelete: (rows: Array<TData>) => void;
};

// The menu of a right-click, Shift+F10 or the context-menu key on a row: Open, Open in new tab, the
// bulk actions that apply, and Delete last. On several rows (the selection), a header names how
// many there are.
export function RowMenu<TData extends RowData>({
  isOpen,
  onOpenChange,
  triggerRef,
  point,
  label,
  rows,
  items,
  onDelete,
}: RowMenuProps<TData>) {
  const groups: Array<ReactNode> = [];
  if (items.open ?? items.href) {
    const { open, href } = items;
    groups.push(
      <MenuItem
        key="open"
        id="open"
        icon={IconArrowRight}
        // Without a row action, Open follows the row's link.
        href={open ? undefined : href}
        onAction={open}
      >
        {m.dataTableOpen()}
      </MenuItem>,
      href && (
        <MenuItem
          key="open-in-new-tab"
          id="open-in-new-tab"
          icon={IconExternalLink}
          href={href}
          target="_blank"
        >
          {m.dataTableOpenInNewTab()}
        </MenuItem>
      ),
    );
  }
  if (items.actions.length > 0) {
    if (groups.length > 0) {
      groups.push(<MenuSeparator key="actions-separator" />);
    }
    groups.push(
      ...items.actions.map(({ action, rows: applicable }) => (
        <MenuItem
          key={`action-${action.label()}`}
          id={`action-${action.label()}`}
          icon={action.icon}
          onAction={() => {
            void action.onAction(applicable);
          }}
        >
          {action.label()}
        </MenuItem>
      )),
    );
  }
  if (items.canDelete) {
    if (groups.length > 0) {
      groups.push(<MenuSeparator key="delete-separator" />);
    }
    groups.push(
      <MenuItem
        key="delete"
        id="delete"
        icon={IconTrash}
        variant="danger"
        onAction={() => {
          onDelete(rows);
        }}
      >
        {m.dataTableDelete()}
      </MenuItem>,
    );
  }

  return (
    <ContextMenu
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      triggerRef={triggerRef}
      point={point}
      aria-label={label}
    >
      {rows.length > 1 ? <MenuSection title={label}>{groups}</MenuSection> : groups}
    </ContextMenu>
  );
}
