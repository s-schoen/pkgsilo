import { Children, createContext, isValidElement, use, useId } from "react";
import { Link as AriaLink } from "react-aria-components/Link";

import { useRoutePreload } from "@/hooks/use-route-preload.ts";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactNode } from "react";
import type { LinkProps as AriaLinkProps } from "react-aria-components/Link";

import styles from "@/components/ui/nav-list/nav-list.module.css";

type NavListProps = {
  // The name of the navigation landmark: a message.
  "aria-label": string;
  // The NavItems and NavSections.
  children: ReactNode;
  // Called when the user follows any of the items on this page, as a drawer closes then; not for a
  // click with a modifier key, which opens the item elsewhere.
  onNavigate?: () => void;
  // The actions that the signed-in identity may take: an item with an action that isn't among them
  // hides. Without it, as in a story, every item shows.
  allowedActions?: ReadonlyArray<string>;
};

// The onNavigate of the NavList around an item.
const NavigateContext = createContext<(() => void) | undefined>(undefined);

// The allowedActions of the NavList around an item.
const AllowedActionsContext = createContext<ReadonlyArray<string> | undefined>(undefined);

// Whether an item that needs action shows under these allowedActions.
function isAllowed(action: string | undefined, allowedActions: ReadonlyArray<string> | undefined) {
  return action === undefined || allowedActions === undefined || allowedActions.includes(action);
}

// The links between the pages of the app, as a navigation landmark, whose NavItems may be grouped
// in NavSections:
// <NavList aria-label={m.mainNavigation()}>
//   <NavItem href="/" icon={IconHome} isCurrent>{m.home()}</NavItem>
//   <NavSection title={m.administration()}>…</NavSection>
// </NavList>.
// With allowedActions, an item that names an action hides unless the list holds it.
export function NavList({ children, onNavigate, allowedActions, ...props }: NavListProps) {
  return (
    <nav {...props} className={styles.navList}>
      <NavigateContext value={onNavigate}>
        <AllowedActionsContext value={allowedActions}>
          <ul className={styles.list}>{children}</ul>
        </AllowedActionsContext>
      </NavigateContext>
    </nav>
  );
}

type NavSectionProps = {
  // The section's heading: a message.
  title: string;
  // The section's NavItems.
  children: ReactNode;
};

// A group of a NavList's items under a heading, which names their list for a screen reader. It
// doesn't collapse, so its items are always one Tab away:
// <NavSection title={m.administration()}>
//   <NavItem href="/identities" icon={IconUsers} action="identity:read">{m.identities()}</NavItem>
// </NavSection>.
// A section none of whose items shows, as its actions aren't allowed, hides with its heading. It
// tells from its children, so pass the NavItems themselves: any other child counts as shown.
export function NavSection({ title, children }: NavSectionProps) {
  const headingId = useId();
  const allowedActions = use(AllowedActionsContext);
  const hasVisibleItem = Children.toArray(children).some(
    (child) =>
      !isValidElement<NavItemProps>(child) ||
      child.type !== NavItem ||
      isAllowed(child.props.action, allowedActions),
  );

  if (!hasVisibleItem) {
    return null;
  }

  return (
    <li className={styles.section}>
      <h2 id={headingId} className={styles.sectionHeading}>
        {title}
      </h2>
      <ul aria-labelledby={headingId} className={styles.list}>
        {children}
      </ul>
    </li>
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
  // The action that the page needs, such as identity:read: the item hides unless the NavList's
  // allowedActions hold it.
  action?: string;
};

// A link of a NavList, a row that navigates through the router and preloads its route once the
// user means to follow it; the current page's is marked with aria-current="page". With action, it
// shows only if the NavList's allowedActions hold it.
export function NavItem({
  children,
  icon: Icon,
  isCurrent = false,
  action,
  onPress,
  ...props
}: NavItemProps) {
  const onNavigate = use(NavigateContext);
  const allowedActions = use(AllowedActionsContext);
  const preloadProps = useRoutePreload(props.href);

  if (!isAllowed(action, allowedActions)) {
    return null;
  }

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
