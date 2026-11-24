import IconHome from "@tabler/icons-react/icons/IconHome";
import IconUsers from "@tabler/icons-react/icons/IconUsers";
import { useMatchRoute } from "@tanstack/react-router";

import { NavItem, NavList, NavSection } from "@/components/ui/nav-list";
import { usePermissions } from "@/features/permissions/hooks/use-permissions.ts";
import { m } from "@/paraglide/messages.js";

// The navigation of the app, in the side menu and the drawer of AppLayout: an entry for each page
// that a feature's route adds. The entry of the page that's shown is current: exactly for the start
// page, and with { fuzzy: true } for a section whose pages lie below its path. An entry whose
// action the signed-in identity's permissions don't list hides, and so does a section without an
// entry left.
export function AppNavigation({ onNavigate }: { onNavigate?: () => void }) {
  const matchRoute = useMatchRoute();
  const { allowedActions } = usePermissions();

  return (
    <NavList
      aria-label={m.mainNavigation()}
      onNavigate={onNavigate}
      allowedActions={allowedActions}
    >
      <NavItem href="/" icon={IconHome} isCurrent={matchRoute({ to: "/" }) !== false}>
        {m.home()}
      </NavItem>
      <NavSection title={m.administration()}>
        <NavItem
          href="/identities"
          icon={IconUsers}
          action="identity:read"
          isCurrent={matchRoute({ to: "/identities", fuzzy: true }) !== false}
        >
          {m.identities()}
        </NavItem>
      </NavSection>
    </NavList>
  );
}
