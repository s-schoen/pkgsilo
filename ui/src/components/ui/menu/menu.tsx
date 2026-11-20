import { Header } from "react-aria-components/Header";
import {
  Menu as AriaMenu,
  MenuItem as AriaMenuItem,
  MenuSection as AriaMenuSection,
} from "react-aria-components/Menu";
import { Separator } from "react-aria-components/Separator";

import { Popover } from "@/components/ui/popover";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactNode } from "react";
import type {
  MenuItemProps as AriaMenuItemProps,
  MenuProps as AriaMenuProps,
} from "react-aria-components/Menu";
import type { PopoverProps } from "react-aria-components/Popover";

import styles from "@/components/ui/menu/menu.module.css";

// The trigger of a menu: it pairs a button (usually a ghost Button with an icon alone) with the Menu
// that the button opens.
export { MenuTrigger } from "react-aria-components/Menu";

type MenuProps<T> = Omit<AriaMenuProps<T>, "className" | "style"> & {
  // Where the menu opens, next to its trigger: below its start unless a view needs another side.
  placement?: PopoverProps["placement"];
};

// A list of actions in a popover, of React Aria Components, which takes its props, such as items
// and onAction (with the id of the chosen item):
// <MenuTrigger>
//   <Button variant="ghost" icon={IconDots} aria-label={m.actions()} />
//   <Menu onAction={(id) => …}>
//     <MenuItem id="rename">{m.rename()}</MenuItem>
//     <MenuItem id="delete" variant="danger">{m.delete()}</MenuItem>
//   </Menu>
// </MenuTrigger>.
// The arrow keys move through the items, Enter chooses one, and Escape closes the menu.
export function Menu<T extends object>({ placement = "bottom start", ...props }: MenuProps<T>) {
  return (
    <Popover placement={placement}>
      <AriaMenu {...props} className={styles.menu} />
    </Popover>
  );
}

type MenuItemProps = Omit<AriaMenuItemProps, "children" | "className" | "style"> & {
  // The action's text: a message.
  children: string;
  // An icon of @tabler/icons-react before the text, decorative.
  icon?: TablerIcon;
  // danger for an action that destroys data.
  variant?: "neutral" | "danger";
};

// An action of a Menu. With href, it's a link that navigates through the router.
export function MenuItem({ children, icon: Icon, variant = "neutral", ...props }: MenuItemProps) {
  return (
    <AriaMenuItem
      {...props}
      textValue={props.textValue ?? children}
      className={`${styles.item} ${styles[variant]}`}
    >
      {Icon && <Icon aria-hidden className={styles.icon} />}
      <span className={styles.label}>{children}</span>
    </AriaMenuItem>
  );
}

// A group of a Menu's items, under an optional heading: a message.
export function MenuSection({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <AriaMenuSection className={styles.section}>
      {title && <Header className={styles.header}>{title}</Header>}
      {children}
    </AriaMenuSection>
  );
}

// A line between the items or sections of a Menu.
export function MenuSeparator() {
  return <Separator className={styles.separator} />;
}
