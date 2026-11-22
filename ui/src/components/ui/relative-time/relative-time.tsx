import { Focusable } from "react-aria-components/Focusable";

import { formatDateTime, formatRelativeTime } from "@/components/ui/relative-time/format.ts";
import { Tooltip, TooltipTrigger } from "@/components/ui/tooltip";
import { getLocale } from "@/paraglide/runtime.js";

import styles from "@/components/ui/relative-time/relative-time.module.css";

type RelativeTimeProps = {
  // The time, as the API gives it (ISO 8601) or as a Date.
  date: string | Date;
};

// A time as how long ago it was, or how long until it comes ("3 days ago"), in the locale of the
// messages, with the date and the time in a tooltip: <RelativeTime date={identity.updatedAt} />.
// It's a <time>, whose dateTime gives the exact time to a machine. The keyboard reaches it, so that
// the tooltip shows without a pointer too: a tooltip's trigger needs a role that a screen reader
// reads its description on, so it's an image named by its text, as React Aria's Focusable asks.
// It renders the text once: it doesn't tick.
export function RelativeTime({ date }: RelativeTimeProps) {
  const time = typeof date === "string" ? new Date(date) : date;
  const locale = getLocale();

  const relative = formatRelativeTime(time, locale);

  return (
    <TooltipTrigger>
      <Focusable>
        <time
          dateTime={time.toISOString()}
          // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- an image named by its text, a role that a tooltip's description is read on; an <img> holds no text
          role="img"
          aria-label={relative}
          // oxlint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- the tooltip needs a focusable trigger, so that the keyboard shows the exact time too
          tabIndex={0}
          className={styles.time}
        >
          {relative}
        </time>
      </Focusable>
      <Tooltip>{formatDateTime(time, locale)}</Tooltip>
    </TooltipTrigger>
  );
}
