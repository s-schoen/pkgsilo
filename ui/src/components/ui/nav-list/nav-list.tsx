import { createContext, use } from "react";
import { Link as AriaLink } from "react-aria-components/Link";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactNode } from "react";
import type { LinkProps as AriaLinkProps } from "react-aria-components/Link";

import styles from "@/components/ui/nav-list/nav-list.module.css";

type NavListProps = {
  // The name of the navigation landmark: a message.
  "aria-label": string;
  // The NavItems.
  children: ReactNode;
  // Called when the user follows any of the items, as a drawer closes then.
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

type NavItemProps = Omit<
  AriaLinkProps,
  "children" | "className" | "style" | "aria-current" | "href"
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

// A link of a NavList, a row that navigates through the router; the current page's is marked with
// aria-current="page".
export function NavItem({
  children,
  icon: Icon,
  isCurrent = false,
  onPress,
  ...props
}: NavItemProps) {
  const onNavigate = use(NavigateContext);

  return (
    <li className={styles.listItem}>
      <AriaLink
        {...props}
        onPress={(event) => {
          onPress?.(event);
          onNavigate?.();
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
