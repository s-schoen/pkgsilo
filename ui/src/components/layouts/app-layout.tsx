import IconMenu2 from "@tabler/icons-react/icons/IconMenu2";
import { useEffect, useState } from "react";
import { Link } from "react-aria-components/Link";

import { Button } from "@/components/ui/button";
import { DialogTrigger, Drawer } from "@/components/ui/drawer";
import { useRoutePreload } from "@/hooks/use-route-preload.ts";
import { m } from "@/paraglide/messages.js";

import type { ComponentType, ReactNode } from "react";

import styles from "@/components/layouts/app-layout.module.css";

type AppLayoutProps = {
  // The navigation of the app, which the layout renders in the side menu and, on a small screen, in
  // a drawer: there it passes onNavigate, which closes the drawer once the user follows a link.
  navigation: ComponentType<{ onNavigate?: () => void }>;
  // The controls at the end of the header, such as the color scheme's menu.
  actions?: ReactNode;
  // The page.
  children: ReactNode;
};

// The width from which the side menu shows the navigation: --md of src/styles/media.css.
const sideMenuQuery = "(width >= 48rem)";

// The frame of every page: the header across the top, the navigation in a side menu at its start,
// and the page on its surface. On a small screen, a button in the header opens the navigation in a
// drawer instead of the side menu.
export function AppLayout({ navigation: Navigation, actions, children }: AppLayoutProps) {
  const [isDrawerOpen, setDrawerOpen] = useState(false);
  const brandPreloadProps = useRoutePreload("/");

  // The drawer closes once the screen is wide enough for the side menu, as when a tablet turns:
  // its overlay would cover the side menu, which shows the same navigation.
  useEffect(() => {
    const sideMenu = matchMedia(sideMenuQuery);
    function closeDrawer() {
      if (sideMenu.matches) {
        setDrawerOpen(false);
      }
    }
    sideMenu.addEventListener("change", closeDrawer);
    return () => {
      sideMenu.removeEventListener("change", closeDrawer);
    };
  }, []);

  return (
    <div className={styles.layout}>
      <header className={styles.header}>
        <div className={styles.menuButton}>
          <DialogTrigger isOpen={isDrawerOpen} onOpenChange={setDrawerOpen}>
            <Button variant="ghost" icon={IconMenu2} aria-label={m.openNavigation()} />
            <Drawer title={m.navigation()}>
              <Navigation
                onNavigate={() => {
                  setDrawerOpen(false);
                }}
              />
            </Drawer>
          </DialogTrigger>
        </div>
        <Link href="/" {...brandPreloadProps} className={styles.brand}>
          pkgsilo
        </Link>
        <div className={styles.actions}>{actions}</div>
      </header>
      <aside className={styles.sidebar}>
        <Navigation />
      </aside>
      <main className={styles.main}>{children}</main>
    </div>
  );
}
