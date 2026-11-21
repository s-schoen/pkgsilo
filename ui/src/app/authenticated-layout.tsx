import { useDbClient } from "@tanstack/react-db";
import { useQueryClient } from "@tanstack/react-query";
import { Outlet, useNavigate } from "@tanstack/react-router";

import { AppNavigation } from "@/app/app-navigation.tsx";
import { AppLayout } from "@/components/layouts/app-layout.tsx";
import { ColorSchemeMenu } from "@/features/appearance/components/color-scheme-menu.tsx";
import { AccountMenu } from "@/features/session/components/account-menu.tsx";
import { useSessionExpiry } from "@/features/session/hooks/use-session-expiry.ts";
import { clearSessionData } from "@/features/session/utils/clear-session-data.ts";

// The component of the authenticated route: the app's frame around every page that needs a
// session, with the navigation, the color scheme's menu and the account. It checks the session
// again once it ends (useSessionExpiry). A sign-out leads to the sign-in page, which replaces the
// page in the history, since Back would only lead to the sign-in page again, and then removes what
// the clients hold of the session that ended (clearSessionData).
export function AuthenticatedLayout() {
  const queryClient = useQueryClient();
  const dbClient = useDbClient();
  const navigate = useNavigate();
  useSessionExpiry();

  return (
    <AppLayout
      navigation={AppNavigation}
      actions={<ColorSchemeMenu />}
      account={
        <AccountMenu
          onSignedOut={() => {
            void navigate({ to: "/signin", replace: true }).then(() =>
              clearSessionData({ queryClient, dbClient }),
            );
          }}
        />
      }
    >
      <Outlet />
    </AppLayout>
  );
}
