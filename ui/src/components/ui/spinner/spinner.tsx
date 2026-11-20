import { ProgressBar } from "react-aria-components/ProgressBar";

import { m } from "@/paraglide/messages.js";

import styles from "@/components/ui/spinner/spinner.module.css";

type SpinnerProps = {
  // What's loading, read by a screen reader: "Loading" unless a view says more.
  label?: string;
};

// A spinning ring that says something is loading, with the progressbar role and no value. It's as
// large as its text, so it follows the font size of its parent: <Spinner /> or
// <Spinner label={m.loadingGroups()} />.
export function Spinner({ label }: SpinnerProps) {
  return (
    <ProgressBar isIndeterminate aria-label={label ?? m.loading()} className={styles.spinner}>
      <svg viewBox="0 0 16 16" fill="none" aria-hidden className={styles.ring}>
        <circle cx="8" cy="8" r="6.5" strokeWidth="2" className={styles.track} />
        <path d="M8 1.5a6.5 6.5 0 0 1 6.5 6.5" strokeWidth="2" strokeLinecap="round" />
      </svg>
    </ProgressBar>
  );
}
