import IconChevronRight from "@tabler/icons-react/icons/IconChevronRight";
import { flexRender } from "@tanstack/react-table";
import { Button } from "react-aria-components/Button";
import { GridList, GridListItem } from "react-aria-components/GridList";
import { Link } from "react-aria-components/Link";
import { Tree, TreeItem, TreeItemContent } from "react-aria-components/Tree";

import { Checkbox } from "@/components/ui/checkbox";
import { PropertyPreview } from "@/components/ui/data-table/property-preview.tsx";
import { rowText } from "@/components/ui/data-table/row-text.ts";
import { useRoutePreload } from "@/hooks/use-route-preload.ts";

import type { Group } from "@/components/ui/data-table/grouping.ts";
import type { DataTableColumnDef, DataTableRow, Href } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";
import type { MouseEvent, ReactNode, Ref } from "react";
import type { Key, Selection } from "react-aria-components/GridList";

import styles from "@/components/ui/data-table/data-table.module.css";

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

type GroupedRowsProps<TData extends RowData> = Omit<RowsProps<TData>, "rows"> & {
  groups: Array<Group<DataTableRow<TData>>>;
  // A group's icon from its value.
  renderIcon?: (value: unknown) => ReactNode;
  // The ids of the groups that are collapsed.
  collapsedGroups: ReadonlySet<string>;
  onCollapsedGroupsChange: (collapsed: Set<string>) => void;
};

// The rows in their groups, in React Aria's Tree: a group is a row that expands to its rows. A
// GridList's section header can't take the focus, so it couldn't hold a toggle that the keyboard
// reaches; a tree's row can. The arrow keys move between the groups and the rows: Right expands a
// group and Left collapses it (from one of its rows, it moves to the group first), as Enter and
// the group's chevron toggle it. A collapsed group's rows leave the tree, so the focus never lands
// in one. The groups take no part in the selection, which spans the groups: Shift+click selects
// across them, and Ctrl/Cmd+A selects the rows of collapsed groups too.
export function GroupedRows<TData extends RowData>({
  label,
  groups,
  renderIcon,
  collapsedGroups,
  onCollapsedGroupsChange,
  getRowHref,
  onRowAction,
  isRowActive,
  isSelectable,
  selectedKeys,
  onSelectionChange,
  ref,
}: GroupedRowsProps<TData>) {
  const groupIds = groups.map((group) => group.id);
  const byId = new Map(groups.flatMap((group) => group.rows.map((row) => [row.id, row])));

  const toggle = (id: string) => {
    const next = new Set(collapsedGroups);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    onCollapsedGroupsChange(next);
  };

  return (
    <Tree
      ref={ref}
      aria-label={label}
      className={styles.rows}
      selectionMode={isSelectable ? "multiple" : "none"}
      selectedKeys={selectedKeys}
      onSelectionChange={onSelectionChange}
      // The groups can't be selected, yet still take the focus and expand.
      disabledKeys={groupIds}
      disabledBehavior="selection"
      expandedKeys={groupIds.filter((id) => !collapsedGroups.has(id))}
      // The tree knows only the groups it shows: the groups that the filters hide keep their state.
      onExpandedChange={(expanded) => {
        const next = new Set(collapsedGroups);
        for (const id of groupIds) {
          if (expanded.has(id)) {
            next.delete(id);
          } else {
            next.add(id);
          }
        }
        onCollapsedGroupsChange(next);
      }}
      data-selecting={selectedKeys.size > 0 || undefined}
      // Enter or a click on a group toggles it; on a row, it opens the row. Without a row action,
      // the tree has none, so that a click on a row selects it (or follows its link). React Aria
      // then toggles a group itself as long as nothing is selectable; else the group's own action
      // does. Never both, which would toggle it twice.
      onAction={
        onRowAction &&
        ((key) => {
          const id = String(key);
          const row = byId.get(id);
          if (row) {
            onRowAction(row.original);
          } else {
            toggle(id);
          }
        })
      }
    >
      {groups.map((group) => (
        <TreeItem
          key={group.id}
          id={group.id}
          textValue={group.label}
          className={styles.group}
          onAction={
            isSelectable && !onRowAction
              ? () => {
                  toggle(group.id);
                }
              : undefined
          }
        >
          <TreeItemContent>
            {/* The chevron's name, Expand or Collapse, is React Aria's. */}
            <Button slot="chevron" className={styles.groupChevron}>
              <IconChevronRight aria-hidden />
            </Button>
            {renderIcon && <span className={styles.groupIcon}>{renderIcon(group.value)}</span>}
            <span className={styles.groupLabel}>{group.label}</span>
            <span className={styles.groupCount}>{group.rows.length}</span>
          </TreeItemContent>
          {group.rows.map((row) => (
            <TreeRow
              key={row.id}
              row={row}
              href={getRowHref?.(row.original)}
              onRowAction={onRowAction}
              isActive={isRowActive?.(row.original) ?? false}
              isSelectable={isSelectable}
            />
          ))}
        </TreeItem>
      ))}
    </Tree>
  );
}

type RowProps<TData extends RowData> = {
  row: DataTableRow<TData>;
  href: Href | undefined;
  onRowAction: ((row: TData) => void) | undefined;
  isActive: boolean;
  isSelectable: boolean;
};

// The attributes of a row's item that its styles read, and its id, which finds the row of a
// right-click or of a key that opens its context menu.
function rowAttributes<TData extends RowData>({
  row,
  isActive,
  href,
  onRowAction,
}: RowProps<TData>) {
  return {
    "data-row-id": row.id,
    "data-active": isActive || undefined,
    "data-actionable": Boolean(onRowAction ?? href) || undefined,
  };
}

// A row in the GridList.
function Row<TData extends RowData>(props: RowProps<TData>) {
  const { row, href, onRowAction } = props;

  return (
    <GridListItem
      id={row.id}
      textValue={rowText(row)}
      href={onRowAction ? undefined : href}
      className={styles.row}
      {...rowAttributes(props)}
    >
      <RowContent {...props} />
    </GridListItem>
  );
}

// A row in a group of the Tree.
function TreeRow<TData extends RowData>(props: RowProps<TData>) {
  const { row, href, onRowAction } = props;

  return (
    <TreeItem
      id={row.id}
      textValue={rowText(row)}
      href={onRowAction ? undefined : href}
      className={styles.row}
      {...rowAttributes(props)}
    >
      <TreeItemContent>
        <RowContent {...props} />
      </TreeItemContent>
    </TreeItem>
  );
}

// A row as Plane's list shows it: its selection box, the leading cells, the title, and the
// property chips at the end, which wrap below the title when the row is too narrow for them, for
// the properties that have a value. A
// chip of a column with a label previews it on hover or focus (property-preview.tsx); Right and
// Left move to it from the row, in the Tree's rows too, since a row without children of the tree
// leaves those keys to its own focusable elements.
function RowContent<TData extends RowData>({
  row,
  href,
  onRowAction,
  isSelectable,
}: RowProps<TData>) {
  const cells = row.getVisibleCells();
  const cellsAt = (placement: string) =>
    cells.filter((cell) => cell.column.columnDef.meta?.placement === placement);
  const titleCell = cellsAt("title").at(0);
  const title = titleCell
    ? flexRender(titleCell.column.columnDef.cell, titleCell.getContext())
    : null;
  // A property whose value is missing, as a field that the API leaves out, shows no chip: an empty
  // one would still take the focus. A column without an accessor renders what its cell says.
  const properties = cellsAt("property").filter(
    (cell) => cell.column.accessorFn === undefined || cell.getValue() != null,
  );

  return (
    <>
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
          {properties.map((cell) => {
            const chip = flexRender(cell.column.columnDef.cell, cell.getContext());
            const label = cell.column.columnDef.meta?.label;
            return (
              <span key={cell.id} className={styles.property}>
                {label ? (
                  <PropertyPreview
                    label={label()}
                    preview={(cell.column.columnDef as DataTableColumnDef<TData>).preview}
                    row={row.original}
                  >
                    {chip}
                  </PropertyPreview>
                ) : (
                  chip
                )}
              </span>
            );
          })}
        </span>
      )}
    </>
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
