import { Link } from "@tanstack/react-router";

import type { ReactNode } from "react";

// The frame of every page: the header, then the page.
export function AppLayout({ children }: { children: ReactNode }) {
  return (
    <>
      <header>
        <h1>
          <Link to="/">pkgsilo</Link>
        </h1>
      </header>
      <main>{children}</main>
    </>
  );
}
