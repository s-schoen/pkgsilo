import { useId } from "react";
import { Popover as AriaPopover } from "react-aria-components/Popover";

import type { ReactNode } from "react";
import type { PopoverProps as AriaPopoverProps } from "react-aria-components/Popover";

import styles from "@/components/ui/preview-card/preview-card.module.css";

// The trigger of a preview card, of React Aria Components: it pairs a focusable element, such as a
// Link, with its PreviewCard, which opens after the pointer has rested on the element for delay ms,
// once the keyboard's focus has stayed on it as long, or on a long press.
export { PreviewTrigger } from "react-aria-components/PreviewTrigger";

type PreviewCardProps = Omit<
  AriaPopoverProps,
  "children" | "className" | "style" | "aria-label" | "aria-labelledby"
> & {
  // What the element is, which names the card: a message.
  heading: string;
  // More about it, below the heading, such as an owner's name and picture. It renders only while
  // the card is open, so it may load what it shows.
  children?: ReactNode;
};

// A card with more about an element, which floats next to it while the pointer rests on it, inside
// a PreviewTrigger:
// <PreviewTrigger>
//   <Link href="/identities/ada">ada</Link>
//   <PreviewCard heading={m.owner()}>…</PreviewCard>
// </PreviewTrigger>.
// Unlike a tooltip it may hold links or buttons, which Tab reaches from the trigger. It never
// holds what the user needs to act: a touch screen opens it only on a long press.
export function PreviewCard({
  heading,
  children,
  placement = "bottom",
  offset = 6,
  ...props
}: PreviewCardProps) {
  const headingId = useId();

  return (
    <AriaPopover
      {...props}
      placement={placement}
      offset={offset}
      aria-labelledby={headingId}
      className={styles.card}
    >
      <span id={headingId} className={styles.heading}>
        {heading}
      </span>
      {children !== undefined && <div className={styles.body}>{children}</div>}
    </AriaPopover>
  );
}
