import { flexRender } from "@tanstack/react-table";
import { GridList, GridListItem } from "react-aria-components/GridList";
import { Link } from "react-aria-components/Link";

import { Checkbox } from "@/components/ui/checkbox";
import { useRoutePreload } from "@/hooks/use-route-preload.ts";

import type { DataTableRow } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";
import type { MouseEvent, ReactNode, Ref } from "react";
import type { Key, Selection } from "react-aria-components/GridList";
import type { LinkProps } from "react-aria-components/Link";

import styles from "@/components/ui/data-table/data-table.module.css";

// A route of the app, as React Aria's links take it in href.
export type Href = NonNullable<LinkProps["href"]>;

type RowsProps<TData extends RowData> = {
  label: string;
  rows: Array<DataTableRow<TData>>;
  getRowHref?: (row: TData) => Href;
  onRowAction?: (row: TData) => void;
  isRowActive?: (row: TData) => boolean;
  // Whether the rows have a selection box; the keys of the selected rows.
  isSelectable: boolean;
  selectedKeys: Set<Key>;
  onSelectionChange: (keys: Selection) => void;
  ref?: Ref<HTMLDivElement>;
};

// The rows in React Aria's GridList, which brings the keyboard: the arrow keys move between the
// rows, and into a row's link; Enter or a click opens the row. Without onRowAction, a row with an
// href is a link of React Aria, which follows the href, in a new tab with a modifier key.
//
// Selectable rows select as React Aria's grid list does: by their box, Space, Shift+click and
// Ctrl/Cmd+A; while any row is selected, a click on a row toggles it instead of opening it.
export function Rows<TData extends RowData>({
  label,
  rows,
  getRowHref,
  onRowAction,
  isRowActive,
  isSelectable,
  selectedKeys,
  onSelectionChange,
  ref,
}: RowsProps<TData>) {
  const byId = new Map(rows.map((row) => [row.id, row]));

  return (
    <GridList
      ref={ref}
      aria-label={label}
      layout="stack"
      className={styles.rows}
      selectionMode={isSelectable ? "multiple" : "none"}
      selectedKeys={selectedKeys}
      onSelectionChange={onSelectionChange}
      data-selecting={selectedKeys.size > 0 || undefined}
      onAction={
        onRowAction &&
        ((key) => {
          const row = byId.get(String(key));
          if (row) {
            onRowAction(row.original);
          }
        })
      }
    >
      {rows.map((row) => (
        <Row
          key={row.id}
          row={row}
          href={getRowHref?.(row.original)}
          onRowAction={onRowAction}
          isActive={isRowActive?.(row.original) ?? false}
          isSelectable={isSelectable}
        />
      ))}
    </GridList>
  );
}

type RowProps<TData extends RowData> = {
  row: DataTableRow<TData>;
  href: Href | undefined;
  onRowAction: ((row: TData) => void) | undefined;
  isActive: boolean;
  isSelectable: boolean;
};

// A row as Plane's list shows it: its selection box, the leading cells, the title, and the
// property chips at the end, which wrap below the title when the row is too narrow for them.
function Row<TData extends RowData>({
  row,
  href,
  onRowAction,
  isActive,
  isSelectable,
}: RowProps<TData>) {
  const cells = row.getVisibleCells();
  const cellsAt = (placement: string) =>
    cells.filter((cell) => cell.column.columnDef.meta?.placement === placement);
  const titleCell = cellsAt("title").at(0);
  const title = titleCell
    ? flexRender(titleCell.column.columnDef.cell, titleCell.getContext())
    : null;
  const titleValue = titleCell?.getValue();
  const properties = cellsAt("property");

  return (
    <GridListItem
      id={row.id}
      // What typeahead matches and a screen reader announces: the title's value.
      textValue={
        typeof titleValue === "string" || typeof titleValue === "number"
          ? String(titleValue)
          : row.id
      }
      href={onRowAction ? undefined : href}
      className={styles.row}
      data-active={isActive || undefined}
      data-actionable={Boolean(onRowAction ?? href) || undefined}
    >
      {isSelectable && (
        <span className={styles.selection}>
          <Checkbox slot="selection" />
        </span>
      )}
      {cellsAt("leading").map((cell) => (
        <span key={cell.id} className={styles.leading}>
          {flexRender(cell.column.columnDef.cell, cell.getContext())}
        </span>
      ))}
      <span className={styles.title}>
        {href ? (
          <TitleLink href={href} onPlainClick={onRowAction && (() => onRowAction(row.original))}>
            {title}
          </TitleLink>
        ) : (
          title
        )}
      </span>
      {properties.length > 0 && (
        <span className={styles.properties}>
          {properties.map((cell) => (
            <span key={cell.id} className={styles.property}>
              {flexRender(cell.column.columnDef.cell, cell.getContext())}
            </span>
          ))}
        </span>
      )}
    </GridListItem>
  );
}

type TitleLinkProps = {
  href: Href;
  // Runs instead of following the link on a plain click or Enter, as the row's action.
  onPlainClick: (() => void) | undefined;
  children: ReactNode;
};

// The row's title as a link, which preloads its route on the user's intent, and opens in a new tab
// on a click with a modifier key or the middle button, as a link does.
function TitleLink({ href, onPlainClick, children }: TitleLinkProps) {
  const preloadProps = useRoutePreload(href);

  return (
    <Link
      href={href}
      {...preloadProps}
      className={styles.titleLink}
      onClick={
        onPlainClick &&
        ((event: MouseEvent) => {
          if (isPlainClick(event)) {
            // React Aria follows the link only while the click's default isn't prevented.
            event.preventDefault();
            onPlainClick();
          }
        })
      }
    >
      {children}
    </Link>
  );
}

function isPlainClick(event: MouseEvent) {
  return event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;
}
