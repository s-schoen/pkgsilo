import IconHome from "@tabler/icons-react/icons/IconHome";
import { useMatchRoute } from "@tanstack/react-router";

import { NavItem, NavList } from "@/components/ui/nav-list";
import { m } from "@/paraglide/messages.js";

// The navigation of the app, in the side menu and the drawer of AppLayout: an entry for each page
// that a feature's route adds. The entry of the page that's shown is current: exactly for the start
// page, and with { fuzzy: true } for a section whose pages lie below its path.
export function AppNavigation({ onNavigate }: { onNavigate?: () => void }) {
  const matchRoute = useMatchRoute();

  return (
    <NavList aria-label={m.mainNavigation()} onNavigate={onNavigate}>
      <NavItem href="/" icon={IconHome} isCurrent={matchRoute({ to: "/" }) !== false}>
        {m.home()}
      </NavItem>
    </NavList>
  );
}
