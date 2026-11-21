import IconMenu2 from "@tabler/icons-react/icons/IconMenu2";
import { Link } from "react-aria-components/Link";

import { Button } from "@/components/ui/button";
import { DialogTrigger, Drawer } from "@/components/ui/drawer";
import { m } from "@/paraglide/messages.js";

import type { ReactNode } from "react";

import styles from "@/components/layouts/app-layout.module.css";

type AppLayoutProps = {
  // The navigation of the app, which the layout renders in the side menu and, on a small screen, in
  // a drawer: there it passes onNavigate, which closes the drawer once the user follows a link.
  navigation: (props: { onNavigate?: () => void }) => ReactNode;
  // The controls at the end of the header, such as the color scheme's menu.
  actions?: ReactNode;
  // The page.
  children: ReactNode;
};

// The frame of every page: the header across the top, the navigation in a side menu at its start,
// and the page on its surface. On a small screen, a button in the header opens the navigation in a
// drawer instead of the side menu.
export function AppLayout({ navigation, actions, children }: AppLayoutProps) {
  return (
    <div className={styles.layout}>
      <header className={styles.header}>
        <div className={styles.menuButton}>
          <DialogTrigger>
            <Button variant="ghost" icon={IconMenu2} aria-label={m.openNavigation()} />
            <Drawer title={m.navigation()}>
              {({ close }) => navigation({ onNavigate: close })}
            </Drawer>
          </DialogTrigger>
        </div>
        <Link href="/" className={styles.brand}>
          pkgsilo
        </Link>
        <div className={styles.actions}>{actions}</div>
      </header>
      <aside className={styles.sidebar}>{navigation({})}</aside>
      <main className={styles.main}>{children}</main>
    </div>
  );
}
