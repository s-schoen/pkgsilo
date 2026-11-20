import { Link as AriaLink } from "react-aria-components/Link";

import type { LinkProps as AriaLinkProps } from "react-aria-components/Link";

import styles from "@/components/ui/link/link.module.css";

type LinkProps = Omit<AriaLinkProps, "children" | "className" | "style"> & {
  // The link's text: a message, or a name of the API's data.
  children: string;
  // primary for a link in or after text, in the accent and underlined; secondary for a quiet link
  // among others, such as in a list of metadata, underlined while hovered.
  variant?: "primary" | "secondary";
};

// A link of React Aria Components, which navigates through the router: <Link href="/groups">
// {m.groups()}</Link>, with href checked against the routes. Without href it's a link that runs
// onPress.
export function Link({ variant = "primary", ...props }: LinkProps) {
  return <AriaLink {...props} className={`${styles.link} ${styles[variant]}`} />;
}
