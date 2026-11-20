import { Popover as AriaPopover } from "react-aria-components/Popover";

import type { ReactNode } from "react";
import type { PopoverProps as AriaPopoverProps } from "react-aria-components/Popover";

import styles from "@/components/ui/popover/popover.module.css";

type PopoverProps = Omit<AriaPopoverProps, "children" | "className" | "style"> & {
  children: ReactNode;
  // trigger for a list that picks the trigger's value (a select's, a combobox's), at least as wide
  // as the trigger; content for a menu or a panel, as wide as its content within the popup sizes.
  width?: "trigger" | "content";
};

// A surface that floats next to its trigger, of React Aria Components, which places it and closes it
// on Escape or a click outside. A Select, a ComboBox or a Menu renders one around its list; a panel
// of other content puts a Dialog in it, inside a DialogTrigger with the trigger.
export function Popover({ children, width = "content", offset = 4, ...props }: PopoverProps) {
  return (
    <AriaPopover {...props} offset={offset} className={`${styles.popover} ${styles[width]}`}>
      {children}
    </AriaPopover>
  );
}
