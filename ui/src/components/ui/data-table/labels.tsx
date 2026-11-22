import { isValidElement } from "react";

import type {
  DataTableColumn,
  DataTableOption,
  Message,
} from "@/components/ui/data-table/types.ts";
import type { TablerIcon } from "@tabler/icons-react";
import type { RowData } from "@tanstack/react-table";
import type { ReactElement } from "react";

// The text of a message, or a string of the API's data as it is.
export function text(label: Message | string): string {
  return typeof label === "function" ? label() : label;
}

// A column's name in the toolbar: its label, else its id.
export function columnLabel<TData extends RowData>(column: DataTableColumn<TData>): string {
  return column.columnDef.meta?.label?.() ?? column.id;
}

// An option's icon as the filter list and the tags take it: a Tabler icon, or an element.
export function optionIcon(icon: DataTableOption["icon"]): TablerIcon | ReactElement | undefined {
  if (icon === undefined || icon === null || typeof icon === "boolean") {
    return undefined;
  }
  if (isValidElement(icon)) {
    return icon;
  }
  // A Tabler icon is a component made with forwardRef: an object with a render function.
  if (typeof icon === "function" || (typeof icon === "object" && "render" in icon)) {
    return icon as TablerIcon;
  }
  return <>{icon}</>;
}
