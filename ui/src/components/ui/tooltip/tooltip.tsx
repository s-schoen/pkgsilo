import {
  Tooltip as AriaTooltip,
  TooltipTrigger as AriaTooltipTrigger,
} from "react-aria-components/Tooltip";

import type {
  TooltipProps as AriaTooltipProps,
  TooltipTriggerComponentProps,
} from "react-aria-components/Tooltip";

import styles from "@/components/ui/tooltip/tooltip.module.css";

// The trigger of a tooltip: it pairs a focusable control, such as a Button, with its Tooltip, which
// shows after the pointer has rested on the control for delay ms, or at once when the keyboard
// focuses it.
export function TooltipTrigger({ delay = 600, ...props }: TooltipTriggerComponentProps) {
  return <AriaTooltipTrigger {...props} delay={delay} />;
}

type TooltipProps = Omit<AriaTooltipProps, "children" | "className" | "style"> & {
  // The tooltip's text: a message.
  children: string;
};

// A short text about a control, of React Aria Components:
// <TooltipTrigger>
//   <Button variant="ghost" icon={IconTrash} aria-label={m.delete()} />
//   <Tooltip>{m.delete()}</Tooltip>
// </TooltipTrigger>.
// It names an icon-only button for those who see it; the button's aria-label still names it for a
// screen reader, which the tooltip only describes. It never holds what the user needs to act.
export function Tooltip({ children, placement = "top", offset = 6, ...props }: TooltipProps) {
  return (
    <AriaTooltip {...props} placement={placement} offset={offset} className={styles.tooltip}>
      {children}
    </AriaTooltip>
  );
}
