import type { ReactNode } from "react";

import styles from "@/components/layouts/detail-layout.module.css";

type DetailLayoutProps = {
  // The object's title, the page's <h1>, at the top of the main column.
  title: ReactNode;
  // The object's properties, usually a PropertyList.
  sidebar: ReactNode;
  // The main column's body below the title, such as the object's tabs.
  children?: ReactNode;
};

// The body of a details page, below its PageHeader, as Plane's: from --lg the main column (the
// title, then the body) at the start and a fixed-width sidebar of properties at the end; below it
// one column, in the order title, sidebar, body, so that the properties show before a long body.
// --lg, not --md, since the app's side menu takes room from --md.
export function DetailLayout({ title, sidebar, children }: DetailLayoutProps) {
  return (
    <div className={styles.layout}>
      <div className={styles.title}>{title}</div>
      <div className={styles.sidebar}>{sidebar}</div>
      {children !== undefined && <div className={styles.body}>{children}</div>}
    </div>
  );
}
