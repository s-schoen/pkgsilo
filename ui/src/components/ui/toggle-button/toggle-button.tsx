import { ToggleButton as AriaToggleButton } from "react-aria-components/ToggleButton";
import { ToggleButtonGroup as AriaToggleButtonGroup } from "react-aria-components/ToggleButtonGroup";

import type { TablerIcon } from "@tabler/icons-react";
import type { Ref } from "react";
import type { ToggleButtonProps as AriaToggleButtonProps } from "react-aria-components/ToggleButton";
import type { ToggleButtonGroupProps as AriaToggleButtonGroupProps } from "react-aria-components/ToggleButtonGroup";

import styles from "@/components/ui/toggle-button/toggle-button.module.css";

type ToggleButtonProps = Omit<AriaToggleButtonProps, "children" | "className" | "style"> & {
  // The text names what the button turns on: a message, or a name of the API's data.
  children: string;
  // An icon of @tabler/icons-react before the text, decorative.
  icon?: TablerIcon;
  // For a parent that focuses the button or anchors an overlay to it.
  ref?: Ref<HTMLButtonElement>;
};

// A button that stays pressed until it's pressed again, of React Aria Components, which takes its
// props, such as isSelected, onChange and isDisabled: <ToggleButton isSelected={…} onChange={…}>
// {m.archived()}</ToggleButton>. Inside a ToggleButtonGroup it takes an id instead, and the group
// holds which are pressed. A long text is cut with an ellipsis.
export function ToggleButton({ children, icon: Icon, ...props }: ToggleButtonProps) {
  return (
    <AriaToggleButton {...props} className={styles.toggleButton}>
      {Icon && <Icon aria-hidden className={styles.icon} />}
      <span className={styles.label}>{children}</span>
    </AriaToggleButton>
  );
}

type ToggleButtonGroupProps = Omit<
  AriaToggleButtonGroupProps,
  "className" | "style" | "selectionMode" | "orientation"
> & {
  // A name for the group, as a screen reader announces it: a message.
  "aria-label": string;
};

// A row of ToggleButtons, any of which can be pressed, of React Aria Components, which takes its
// props, such as selectedKeys and onSelectionChange with the ids of the pressed buttons:
// <ToggleButtonGroup aria-label={m.properties()} selectedKeys={…} onSelectionChange={…}>
//   <ToggleButton id="owner">{m.owner()}</ToggleButton>
// </ToggleButtonGroup>.
// It's one stop of the tab order, within which the arrow keys move; the buttons wrap onto a new
// line when the row is full.
export function ToggleButtonGroup(props: ToggleButtonGroupProps) {
  return <AriaToggleButtonGroup {...props} selectionMode="multiple" className={styles.group} />;
}
