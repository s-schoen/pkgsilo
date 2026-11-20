import { Button as AriaButton } from "react-aria-components/Button";

import { Spinner } from "@/components/ui/spinner";

import type { TablerIcon } from "@tabler/icons-react";
import type { Ref } from "react";
import type { ButtonProps as AriaButtonProps } from "react-aria-components/Button";

import styles from "@/components/ui/button/button.module.css";

type ButtonProps = Omit<AriaButtonProps, "children" | "className" | "style"> & {
  // How much the button stands out: primary for the main action of a view, secondary for the
  // others, tertiary and ghost for the least ones (ghost has no fill until it's hovered), danger for
  // an action that destroys data, and danger-outline for one that a view offers next to others.
  variant?: "primary" | "secondary" | "tertiary" | "ghost" | "danger" | "danger-outline";
  // The height, from the control sizes of the tokens: md unless a view needs another.
  size?: "sm" | "md" | "lg";
  // For a parent that focuses the button or anchors an overlay to it.
  ref?: Ref<HTMLButtonElement>;
} & (
    | {
        // The text names the action, so it's a string (a message): null, false or a missing text
        // would leave the button without a name.
        children: string;
        icon?: TablerIcon;
      }
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
//
// isPending, while the action runs, shows a spinner over the content, which keeps the button's width,
// and ignores presses while the button keeps its focus.
export function Button({
  variant = "primary",
  size = "md",
  icon: Icon,
  children,
  isPending,
  ...props
}: ButtonProps) {
  const classNames = [styles.button, variantClassNames[variant], styles[size]];
  // An empty text shows nothing either, so the icon stands alone.
  if (!children) {
    classNames.push(styles.iconOnly);
  }

  return (
    <AriaButton {...props} isPending={isPending} className={classNames.join(" ")}>
      <span className={styles.content}>
        {Icon && <Icon aria-hidden className={styles.icon} />}
        {children}
      </span>
      {isPending && (
        <span className={styles.pending}>
          <Spinner />
        </span>
      )}
    </AriaButton>
  );
}
