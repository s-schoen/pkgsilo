import { createContext, use } from "react";
import { Link as AriaLink } from "react-aria-components/Link";

import { useRoutePreload } from "@/hooks/use-route-preload.ts";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactNode } from "react";
import type { LinkProps as AriaLinkProps } from "react-aria-components/Link";

import styles from "@/components/ui/nav-list/nav-list.module.css";

type NavListProps = {
  // The name of the navigation landmark: a message.
  "aria-label": string;
  // The NavItems.
  children: ReactNode;
  // Called when the user follows any of the items on this page, as a drawer closes then; not for a
  // click with a modifier key, which opens the item elsewhere.
  onNavigate?: () => void;
};

// The onNavigate of the NavList around an item.
const NavigateContext = createContext<(() => void) | undefined>(undefined);

// The links between the pages of the app, as a navigation landmark:
// <NavList aria-label={m.mainNavigation()}>
//   <NavItem href="/" icon={IconHome} isCurrent>{m.home()}</NavItem>
// </NavList>.
export function NavList({ children, onNavigate, ...props }: NavListProps) {
  return (
    <nav {...props} className={styles.navList}>
      <NavigateContext value={onNavigate}>
        <ul className={styles.list}>{children}</ul>
      </NavigateContext>
    </nav>
  );
}

// The item preloads its route on onHoverStart, onHoverEnd, onFocus and onPressStart, so it takes
// none of its own.
type NavItemProps = Omit<
  AriaLinkProps,
  | "children"
  | "className"
  | "style"
  | "aria-current"
  | "href"
  | "onHoverStart"
  | "onHoverEnd"
  | "onFocus"
  | "onPressStart"
> & {
  // The route that the item links to.
  href: NonNullable<AriaLinkProps["href"]>;
  // The page's name: a message.
  children: string;
  // An icon of @tabler/icons-react before the name, decorative.
  icon?: TablerIcon;
  // The item of the page that's shown, which the caller finds through the router.
  isCurrent?: boolean;
};

// A link of a NavList, a row that navigates through the router and preloads its route once the
// user means to follow it; the current page's is marked with aria-current="page".
export function NavItem({
  children,
  icon: Icon,
  isCurrent = false,
  onPress,
  ...props
}: NavItemProps) {
  const onNavigate = use(NavigateContext);
  const preloadProps = useRoutePreload(props.href);

  return (
    <li className={styles.listItem}>
      <AriaLink
        {...props}
        {...preloadProps}
        onPress={(event) => {
          onPress?.(event);
          // With a modifier key, the browser opens the link in a new tab or window, or downloads it,
          // and this page stays: React Aria's router navigates only without one.
          if (!(event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)) {
            onNavigate?.();
          }
        }}
        aria-current={isCurrent ? "page" : undefined}
        className={styles.item}
      >
        {Icon && <Icon aria-hidden className={styles.icon} />}
        <span className={styles.label}>{children}</span>
      </AriaLink>
    </li>
  );
}
