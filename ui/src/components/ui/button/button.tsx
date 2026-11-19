import { Button as AriaButton } from "react-aria-components/Button";

import type { TablerIcon } from "@tabler/icons-react";
import type { ReactNode } from "react";
import type { ButtonProps as AriaButtonProps } from "react-aria-components/Button";

import styles from "@/components/ui/button/button.module.css";

type ButtonProps = Omit<AriaButtonProps, "children" | "className" | "style"> & {
  // How much the button stands out: primary for the main action of a view, secondary for the
  // others, danger for an action that destroys data.
  variant?: "primary" | "secondary" | "danger";
} & (
    | { children: ReactNode; icon?: TablerIcon }
    // A button with an icon alone has no text, so it names its action with aria-label.
    | { children?: undefined; icon: TablerIcon; "aria-label": string }
  );

// A button of React Aria Components, which takes its props, such as onPress, type and isDisabled:
// <Button onPress={…}>{m.save()}</Button>, with an icon of @tabler/icons-react before the text as
// icon={IconPlus}, or alone as <Button icon={IconTrash} aria-label={m.delete()} />. It styles
// itself: its parent lays it out.
export function Button({ variant = "primary", icon: Icon, children, ...props }: ButtonProps) {
  const classNames = [styles.button, styles[variant]];
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
