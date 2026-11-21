import { createContext, use, useEffect, useRef } from "react";

import type { PressEvent } from "react-aria-components";
import type { LinkProps } from "react-aria-components/Link";

// A route of the app, as React Aria's links take it in href.
type Href = NonNullable<LinkProps["href"]>;

// Loads a route's code and data before the user navigates to it: src/app/root-layout.tsx provides
// the router's preloadRoute(). Without it, as in a story, nothing preloads.
export const RoutePreloadContext = createContext<((href: Href) => void) | undefined>(undefined);

// How long the pointer rests on a link before its route preloads, so that a pointer that only
// crosses it loads nothing: the delay of TanStack's <Link>, the router's defaultPreloadDelay, which
// src/app/router.tsx leaves at its default.
const hoverDelay = 50;

// The handlers that preload the route of a React Aria link on the user's intent: once the pointer
// rests on it, it's focused, or a touch starts on it. TanStack's <Link> does so by itself
// (defaultPreload: "intent"); React Aria's links navigate through the router, but don't preload.
// Spread them onto the link: <AriaLink {...props} {...useRoutePreload(props.href)} />.
export function useRoutePreload(href: Href) {
  const preload = use(RoutePreloadContext);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

  // A link that unmounts while the pointer rests on it preloads nothing.
  useEffect(
    () => () => {
      clearTimeout(timer.current);
    },
    [],
  );

  return {
    onHoverStart: () => {
      clearTimeout(timer.current);
      timer.current = setTimeout(() => preload?.(href), hoverDelay);
    },
    onHoverEnd: () => {
      clearTimeout(timer.current);
    },
    onFocus: () => {
      preload?.(href);
    },
    // A touch has no hover: it preloads as it starts, before the press ends and navigates.
    onPressStart: (event: PressEvent) => {
      if (event.pointerType === "touch") {
        preload?.(href);
      }
    },
  };
}
