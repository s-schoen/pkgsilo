import IconX from "@tabler/icons-react/icons/IconX";
import { isValidElement } from "react";
import { Button as AriaButton } from "react-aria-components/Button";
import {
  Tag as AriaTag,
  TagGroup as AriaTagGroup,
  TagList as AriaTagList,
} from "react-aria-components/TagGroup";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactElement, ReactNode } from "react";
import type {
  TagGroupProps as AriaTagGroupProps,
  TagListProps as AriaTagListProps,
  TagProps as AriaTagProps,
} from "react-aria-components/TagGroup";

import styles from "@/components/ui/tag-group/tag-group.module.css";

type TagGroupProps<T> = Omit<AriaTagGroupProps, "children" | "className" | "style"> &
  Pick<AriaTagListProps<T>, "items"> & {
    // The name of the group, as a screen reader announces it: a message.
    "aria-label": string;
    // The Tags, or a function that renders one for each of items.
    children: ReactNode | ((item: T) => ReactNode);
  };

// A row of tags that wraps onto new lines, of React Aria Components' TagGroup, which takes its
// props, such as onRemove with the ids of the tags to remove:
// <TagGroup aria-label={m.activeFilters()} onRemove={…}>
//   <Tag id="status:active" property={m.status()}>{m.active()}</Tag>
// </TagGroup>.
// With onRemove, each tag shows a button that removes it, which React Aria names ("Remove", in the
// locale), and Delete or Backspace removes the focused tag. The group is one stop of the tab order,
// within which the arrow keys move. A status that is only text is a Badge instead.
export function TagGroup<T extends object>({ items, children, ...props }: TagGroupProps<T>) {
  return (
    <AriaTagGroup {...props} className={styles.tagGroup}>
      <AriaTagList items={items} className={styles.list}>
        {children}
      </AriaTagList>
    </AriaTagGroup>
  );
}

type TagProps = Omit<AriaTagProps, "children" | "className" | "style" | "textValue"> & {
  // The tag's value: a message, or a name of the API's data.
  children: string;
  // What the value belongs to, as a filter's property: shown before the value in quieter text, and
  // part of the tag's name for a screen reader.
  property?: string;
  // An icon of @tabler/icons-react, or an element such as a colored dot, before the value;
  // decorative.
  icon?: TablerIcon | ReactElement;
};

// A tag of a TagGroup, in the look of a property chip: its property, its icon and its value, then
// the button that removes it while the group takes onRemove.
export function Tag({ children, property, icon, ...props }: TagProps) {
  return (
    <AriaTag
      {...props}
      textValue={property ? `${property}: ${children}` : children}
      className={styles.tag}
    >
      {({ allowsRemoving }) => (
        <>
          {property && <span className={styles.property}>{property}</span>}
          {icon && <TagIcon icon={icon} />}
          <span className={styles.value}>{children}</span>
          {allowsRemoving && (
            <AriaButton slot="remove" className={styles.remove}>
              <IconX aria-hidden className={styles.removeIcon} />
            </AriaButton>
          )}
        </>
      )}
    </AriaTag>
  );
}

function TagIcon({ icon: Icon }: { icon: TablerIcon | ReactElement }) {
  if (isValidElement(Icon)) {
    return (
      <span aria-hidden className={styles.icon}>
        {Icon}
      </span>
    );
  }
  return <Icon aria-hidden className={styles.icon} />;
}
