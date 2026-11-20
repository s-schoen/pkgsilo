import IconCheck from "@tabler/icons-react/icons/IconCheck";
import {
  ListBox as AriaListBox,
  ListBoxItem as AriaListBoxItem,
} from "react-aria-components/ListBox";

import { m } from "@/paraglide/messages.js";

import type {
  ListBoxItemProps as AriaListBoxItemProps,
  ListBoxProps as AriaListBoxProps,
} from "react-aria-components/ListBox";

import styles from "@/components/ui/list-box/list-box.module.css";

type ListBoxProps<T> = Omit<AriaListBoxProps<T>, "className" | "style" | "renderEmptyState">;

// A list of options to pick from, of React Aria Components, which takes its props, such as items,
// selectionMode and onAction: <ListBox items={groups}>{(group) => <ListBoxItem id={group.id}>
// {group.name}</ListBoxItem>}</ListBox>. A Select and a ComboBox render one in their popover; it
// says "No results" while it's empty.
export function ListBox<T extends object>(props: ListBoxProps<T>) {
  return (
    <AriaListBox
      {...props}
      className={styles.listBox}
      renderEmptyState={() => <div className={styles.empty}>{m.noResults()}</div>}
    />
  );
}

type ListBoxItemProps = Omit<AriaListBoxItemProps, "children" | "className" | "style"> & {
  // The option's text: a name of the API's data, or a message.
  children: string;
};

// An option of a ListBox: its text, and a check while it's selected.
export function ListBoxItem({ children, ...props }: ListBoxItemProps) {
  return (
    <AriaListBoxItem {...props} textValue={props.textValue ?? children} className={styles.item}>
      <span className={styles.label}>{children}</span>
      <IconCheck aria-hidden className={styles.check} />
    </AriaListBoxItem>
  );
}
