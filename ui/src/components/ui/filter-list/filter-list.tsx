import IconCheck from "@tabler/icons-react/icons/IconCheck";
import { isValidElement } from "react";
import { Autocomplete, useFilter } from "react-aria-components/Autocomplete";
import { Header } from "react-aria-components/Header";
import {
  ListBox as AriaListBox,
  ListBoxItem as AriaListBoxItem,
  ListBoxSection as AriaListBoxSection,
} from "react-aria-components/ListBox";

import { SearchField } from "@/components/ui/search-field";
import { m } from "@/paraglide/messages.js";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactElement, ReactNode } from "react";
import type { AutocompleteProps } from "react-aria-components/Autocomplete";
import type {
  ListBoxItemProps as AriaListBoxItemProps,
  ListBoxProps as AriaListBoxProps,
} from "react-aria-components/ListBox";

import styles from "@/components/ui/filter-list/filter-list.module.css";

type FilterListProps<T> = Omit<
  AriaListBoxProps<T>,
  | "className"
  | "style"
  | "renderEmptyState"
  | "selectionMode"
  | "layout"
  | "orientation"
  | "aria-label"
  | "autoFocus"
  | "escapeKeyBehavior"
> &
  Pick<AutocompleteProps<T>, "inputValue" | "defaultInputValue" | "onInputChange"> & {
    // The name of the list, as a screen reader announces it: a message.
    "aria-label": string;
    // The name of the search above the list: a message.
    searchLabel: string;
    // A sample of what to type, while the search is empty: a message.
    placeholder?: string;
    // Focuses the search once it renders, as when a popover opens with the list.
    autoFocus?: boolean;
  };

// A list of options to check any of, with a search above it that narrows the list as the user
// types, of React Aria Components' Autocomplete, ListBox and the library's SearchField, which takes
// the ListBox's props, such as items, selectedKeys and onSelectionChange with the keys of the
// checked options:
// <FilterList aria-label={m.filters()} searchLabel={m.searchFilters()} selectedKeys={…}
//   onSelectionChange={…}>
//   <FilterListSection title={m.status()}>
//     <FilterListItem id="status:active" icon={IconCircleCheck}>{m.active()}</FilterListItem>
//   </FilterListSection>
// </FilterList>.
// The search matches the options' text, ignoring case and accents, and hides a section with no
// match. The focus stays in the search: the arrow keys move through the options, and Enter checks
// or unchecks one; Escape clears the search, then closes the popover the list is in. It says so
// when nothing matches. The parent sets the width; the list scrolls below the search.
export function FilterList<T extends object>({
  searchLabel,
  placeholder,
  autoFocus,
  inputValue,
  defaultInputValue,
  onInputChange,
  ...props
}: FilterListProps<T>) {
  const { contains } = useFilter({ sensitivity: "base" });

  return (
    <div className={styles.filterList}>
      <Autocomplete
        filter={contains}
        inputValue={inputValue}
        defaultInputValue={defaultInputValue}
        onInputChange={onInputChange}
      >
        <div className={styles.search}>
          <SearchField
            variant="bare"
            aria-label={searchLabel}
            placeholder={placeholder}
            // oxlint-disable-next-line jsx-a11y/no-autofocus -- The caller opts in, as a popover that the user opened to search in focuses its search (WAI-ARIA's combobox-in-dialog pattern).
            autoFocus={autoFocus}
          />
        </div>
        <AriaListBox
          {...props}
          selectionMode="multiple"
          // Escape clears the search, then closes the popover, but never unchecks every option.
          escapeKeyBehavior="none"
          className={styles.list}
          renderEmptyState={() => <div className={styles.empty}>{m.noMatchingOptions()}</div>}
        />
      </Autocomplete>
    </div>
  );
}

// A group of a FilterList's options, under its heading: a message, or a name of the API's data.
export function FilterListSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <AriaListBoxSection className={styles.section}>
      <Header className={styles.header}>{title}</Header>
      {children}
    </AriaListBoxSection>
  );
}

type FilterListItemProps = Omit<AriaListBoxItemProps, "children" | "className" | "style"> & {
  // The option's text: a message, or a name of the API's data. The search matches it.
  children: string;
  // An icon of @tabler/icons-react, or an element such as a colored dot, after the box; decorative.
  icon?: TablerIcon | ReactElement;
};

// An option of a FilterList: a box that shows whether it's checked, its icon and its text.
export function FilterListItem({ children, icon, ...props }: FilterListItemProps) {
  return (
    <AriaListBoxItem {...props} textValue={props.textValue ?? children} className={styles.item}>
      <span className={styles.box}>
        <IconCheck aria-hidden className={styles.check} />
      </span>
      {icon && <ItemIcon icon={icon} />}
      <span className={styles.label}>{children}</span>
    </AriaListBoxItem>
  );
}

function ItemIcon({ icon: Icon }: { icon: TablerIcon | ReactElement }) {
  if (isValidElement(Icon)) {
    return (
      <span aria-hidden className={styles.icon}>
        {Icon}
      </span>
    );
  }
  return <Icon aria-hidden className={styles.icon} />;
}
