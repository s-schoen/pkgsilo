import type { TablerIcon } from "@tabler/icons-react";

import styles from "@/components/ui/badge/badge.module.css";

type BadgeProps = {
  // What the badge says, such as a status: a message.
  children: string;
  // The color of a status: neutral for a plain label, accent for what's selected or new, success,
  // warning and danger for what works, needs attention or failed. property is a quiet chip with a
  // frame, for a property of a row in a data table, as its owner or its expiry.
  variant?: "neutral" | "accent" | "success" | "warning" | "danger" | "property";
  // An icon of @tabler/icons-react before the text, decorative.
  icon?: TablerIcon;
};

// A short label that marks the state of an object, such as an API key's: <Badge variant="success">
// {m.active()}</Badge>. It's text, not a control.
export function Badge({ children, variant = "neutral", icon: Icon }: BadgeProps) {
  return (
    <span className={`${styles.badge} ${styles[variant]}`}>
      {Icon && <Icon aria-hidden className={styles.icon} />}
      {children}
    </span>
  );
}
