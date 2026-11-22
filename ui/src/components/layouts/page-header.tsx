import IconChevronRight from "@tabler/icons-react/icons/IconChevronRight";
import { Breadcrumb, Breadcrumbs } from "react-aria-components/Breadcrumbs";
import { Link } from "react-aria-components/Link";

import { useRoutePreload } from "@/hooks/use-route-preload.ts";
import { m } from "@/paraglide/messages.js";

import type { ReactNode } from "react";
import type { LinkProps } from "react-aria-components/Link";

import styles from "@/components/layouts/page-header.module.css";

// A route of the app, as React Aria's links take it in href.
type Href = NonNullable<LinkProps["href"]>;

// A step of the breadcrumbs: its name (a message, or a name of the API's data) and the page it
// links to. The last one is the page that's shown, which links nowhere.
export type PageHeaderCrumb = {
  label: string;
  href?: Href;
};

type PageHeaderProps = {
  // The way to the page, from its section: Identities › Ada Lovelace.
  breadcrumbs?: Array<PageHeaderCrumb>;
  // The page's title, its <h1>: a list page passes one. A details page leaves it out, since the
  // title of its main column is its <h1>.
  title?: string;
  // The page's actions, at the end, such as the button that creates an item: one primary at most.
  actions?: ReactNode;
};

// The top of a page: its breadcrumbs, whose links preload their routes, its title, and its actions
// at the end, which wrap below the rest when the page is narrow.
export function PageHeader({ breadcrumbs, title, actions }: PageHeaderProps) {
  return (
    <header className={styles.header}>
      <div className={styles.start}>
        {breadcrumbs !== undefined && breadcrumbs.length > 0 && (
          <nav aria-label={m.breadcrumbs()}>
            <Breadcrumbs className={styles.breadcrumbs}>
              {breadcrumbs.map((crumb, index) => (
                <Crumb
                  // A path's links differ, and only its last crumb, the page that's shown, has none.
                  key={crumb.href ?? crumb.label}
                  crumb={crumb}
                  isLast={index === breadcrumbs.length - 1}
                />
              ))}
            </Breadcrumbs>
          </nav>
        )}
        {title !== undefined && <h1 className={styles.title}>{title}</h1>}
      </div>
      {actions !== undefined && <div className={styles.actions}>{actions}</div>}
    </header>
  );
}

// A step of the breadcrumbs, with a chevron after it unless it's the last one. React Aria marks
// the last one as the current page and keeps it from linking.
function Crumb({ crumb, isLast }: { crumb: PageHeaderCrumb; isLast: boolean }) {
  return (
    <Breadcrumb className={styles.crumb}>
      {crumb.href === undefined ? (
        <Link className={styles.crumbLink}>{crumb.label}</Link>
      ) : (
        <CrumbLink href={crumb.href}>{crumb.label}</CrumbLink>
      )}
      {!isLast && <IconChevronRight aria-hidden className={styles.separator} />}
    </Breadcrumb>
  );
}

// A crumb's link, which preloads its route on the user's intent.
function CrumbLink({ href, children }: { href: Href; children: string }) {
  const preloadProps = useRoutePreload(href);

  return (
    <Link href={href} {...preloadProps} className={styles.crumbLink}>
      {children}
    </Link>
  );
}
