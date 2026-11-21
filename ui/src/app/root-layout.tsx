import { Outlet, useRouter } from "@tanstack/react-router";
// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";

import { AppNavigation } from "@/app/app-navigation.tsx";
import { AppLayout } from "@/components/layouts/app-layout.tsx";
import { Toaster } from "@/components/ui/toast";
import { ColorSchemeMenu } from "@/features/appearance/components/color-scheme-menu.tsx";
import { RoutePreloadContext } from "@/hooks/use-route-preload.ts";

import type { NavigateOptions, ToOptions } from "@tanstack/react-router";

// The href of React Aria's links (Link, a MenuItem's or a Tab's href) is a route of the router, which
// checks it as it checks <Link to>, and routerOptions are the other options of router.navigate().
declare module "react-aria-components" {
  interface RouterConfig {
    href: ToOptions["to"];
    routerOptions: Omit<NavigateOptions, keyof ToOptions>;
  }
}

// The component of the root route: the frame of every page, with the app's navigation and the color
// scheme's menu, in which React Aria's links navigate through the router, without reloading the
// page, and the toasts of showToast(). A link that useRoutePreload() wires preloads its route
// through the router as well.
export function RootLayout() {
  const router = useRouter();

  return (
    <RouterProvider
      navigate={(to, options) => {
        void router.navigate({ ...options, to });
      }}
      useHref={(to) => router.buildLocation({ to }).href}
    >
      <RoutePreloadContext
        value={(href) => {
          void router.preloadRoute({ to: href });
        }}
      >
        <AppLayout navigation={AppNavigation} actions={<ColorSchemeMenu />}>
          <Outlet />
        </AppLayout>
        <Toaster />
      </RoutePreloadContext>
    </RouterProvider>
  );
}
