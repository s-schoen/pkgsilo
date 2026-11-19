import { Link } from "@tanstack/react-router";

import type { ReactNode } from "react";

import styles from "@/components/layouts/app-layout.module.css";

// The frame of every page: the header, then the page on its surface.
export function AppLayout({ children }: { children: ReactNode }) {
  return (
    <div className={styles.layout}>
      <header className={styles.header}>
        <h1 className={styles.title}>
          <Link to="/" className={styles.brand}>
            pkgsilo
          </Link>
        </h1>
      </header>
      <main className={styles.main}>{children}</main>
    </div>
  );
}
