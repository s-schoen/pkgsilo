import { Button as AriaButton } from "react-aria-components/Button";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactNode } from "react";
import type { ButtonProps as AriaButtonProps } from "react-aria-components/Button";

import styles from "@/components/ui/button/button.module.css";

type ButtonProps = Omit<AriaButtonProps, "children" | "className" | "style"> & {
  // How much the button stands out: primary for the main action of a view, secondary for the
  // others, tertiary and ghost for the least ones (ghost has no fill until it's hovered), danger for
  // an action that destroys data, and danger-outline for one that a view offers next to others.
  variant?: "primary" | "secondary" | "tertiary" | "ghost" | "danger" | "danger-outline";
  // The height, from the control sizes of the tokens: md unless a view needs another.
  size?: "sm" | "md" | "lg";
} & (
    | { children: ReactNode; icon?: TablerIcon }
    // A button with an icon alone has no text, so it names its action with aria-label.
    | { children?: undefined; icon: TablerIcon; "aria-label": string }
  );

// The class of each variant, whose names aren't all identifiers.
const variantClassNames = {
  primary: styles.primary,
  secondary: styles.secondary,
  tertiary: styles.tertiary,
  ghost: styles.ghost,
  danger: styles.danger,
  "danger-outline": styles.dangerOutline,
};

// A button of React Aria Components, which takes its props, such as onPress, type and isDisabled:
// <Button onPress={…}>{m.save()}</Button>, with an icon of @tabler/icons-react before the text as
// icon={IconPlus}, or alone as <Button icon={IconTrash} aria-label={m.delete()} />. It styles
// itself: its parent lays it out.
export function Button({
  variant = "primary",
  size = "md",
  icon: Icon,
  children,
  ...props
}: ButtonProps) {
  const classNames = [styles.button, variantClassNames[variant], styles[size]];
  if (children === undefined) {
    classNames.push(styles.iconOnly);
  }

  return (
    <AriaButton {...props} className={classNames.join(" ")}>
      {Icon && <Icon aria-hidden className={styles.icon} />}
      {children}
    </AriaButton>
  );
}
