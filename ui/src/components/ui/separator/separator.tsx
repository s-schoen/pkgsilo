import { Separator as AriaSeparator } from "react-aria-components/Separator";

import styles from "@/components/ui/separator/separator.module.css";

type SeparatorProps = {
  // horizontal between blocks stacked on top of each other, vertical between items side by side,
  // where it's as tall as its row.
  orientation?: "horizontal" | "vertical";
};

// A line that separates content, with the separator role: <Separator /> between two blocks.
export function Separator({ orientation = "horizontal" }: SeparatorProps) {
  return (
    <AriaSeparator
      orientation={orientation}
      className={`${styles.separator} ${styles[orientation]}`}
    />
  );
}
