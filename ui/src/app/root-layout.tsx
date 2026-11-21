import { Outlet, useRouter } from "@tanstack/react-router";
// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";

import { Toaster } from "@/components/ui/toast";
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

// The component of the root route, around every page, the sign-in page too: React Aria's links
// navigate through the router, without reloading the page, and the toasts of showToast() show. A
// link that useRoutePreload() wires preloads its route through the router as well. The app's frame
// is the authenticated route's (authenticated-layout.tsx).
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
        <Outlet />
        <Toaster />
      </RoutePreloadContext>
    </RouterProvider>
  );
}
