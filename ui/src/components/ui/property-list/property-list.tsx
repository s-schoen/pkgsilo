import { m } from "@/paraglide/messages.js";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactNode } from "react";

import styles from "@/components/ui/property-list/property-list.module.css";

type PropertyListProps = {
  // The rows: PropertyItems.
  children: ReactNode;
};

// An object's properties, as a details page's sidebar shows them, in a <dl>: <PropertyList>
// <PropertyItem icon={IconAt} label={m.username()}>ada</PropertyItem></PropertyList>.
export function PropertyList({ children }: PropertyListProps) {
  return <dl className={styles.list}>{children}</dl>;
}

type PropertyItemProps = {
  // An icon of @tabler/icons-react before the label, decorative.
  icon: TablerIcon;
  // The property's name: a message.
  label: string;
  // The value: text, a link, a RelativeTime. Without one, the row says that it's empty; a property
  // that the actor can't read is left out, not shown empty.
  children?: ReactNode;
};

// A row of a PropertyList: the icon and the label at the start, the value at the end, as Plane's
// sidebar.
export function PropertyItem({ icon: Icon, label, children }: PropertyItemProps) {
  const isEmpty = children === undefined || children === null || children === false;

  return (
    <div className={styles.item}>
      <dt className={styles.label}>
        <Icon aria-hidden className={styles.icon} />
        <span className={styles.labelText}>{label}</span>
      </dt>
      <dd className={isEmpty ? `${styles.value} ${styles.empty}` : styles.value}>
        {isEmpty ? m.propertyEmpty() : children}
      </dd>
    </div>
  );
}
