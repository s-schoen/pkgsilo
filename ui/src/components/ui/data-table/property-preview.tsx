import { Button } from "react-aria-components/Button";
import { Link } from "react-aria-components/Link";
import { VisuallyHidden } from "react-aria-components/VisuallyHidden";

import { PreviewCard, PreviewTrigger } from "@/components/ui/preview-card";
import { useRoutePreload } from "@/hooks/use-route-preload.ts";

import type { DataTablePreview, Href } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";
import type { ReactNode } from "react";
import type { PressEvent } from "react-aria-components/Button";

import styles from "@/components/ui/data-table/data-table.module.css";

type PropertyPreviewProps<TData extends RowData> = {
  // The column's label, which names the chip and heads its card.
  label: string;
  preview: DataTablePreview<TData> | undefined;
  row: TData;
  // The chip, as the column's cell renders it.
  children: ReactNode;
};

// A property chip of a row, which the arrow keys reach inside the row, with a card that names its
// column (and shows the column's preview.content) while the pointer rests on it or it has the focus.
// A screen reader reads the chip as "<label>: <chip>", as the card's heading reads.
export function PropertyPreview<TData extends RowData>({
  label,
  preview,
  row,
  children,
}: PropertyPreviewProps<TData>) {
  // As a filter's chip names its property (tag-group.tsx).
  const prefix = `${label}: `;
  const name = (
    <>
      <VisuallyHidden>{prefix}</VisuallyHidden>
      {children}
    </>
  );

  return (
    <PreviewTrigger>
      {preview?.href ? (
        <PropertyLink href={preview.href(row)}>{name}</PropertyLink>
      ) : preview?.onAction ? (
        <Button
          className={styles.propertyTrigger}
          data-actionable
          onPress={() => {
            preview.onAction?.(row);
          }}
        >
          {name}
        </Button>
      ) : (
        <Button className={styles.propertyTrigger} onPress={pressRow}>
          {name}
        </Button>
      )}
      <PreviewCard heading={label}>{preview?.content?.(row)}</PreviewCard>
    </PreviewTrigger>
  );
}

// A press on a chip without an action of its own presses its row, which opens it, selects it or
// extends the selection with Shift, as a click beside the chip does. The chip can't just let the
// press through: the preview's trigger handles a long press, which keeps the end of every press
// from its row. The click names no pointer, so React Aria takes it as a virtual one.
function pressRow(event: PressEvent) {
  event.target.closest("[data-row-id]")?.dispatchEvent(
    new MouseEvent("click", {
      bubbles: true,
      cancelable: true,
      shiftKey: event.shiftKey,
      ctrlKey: event.ctrlKey,
      metaKey: event.metaKey,
      altKey: event.altKey,
    }),
  );
}

type PropertyLinkProps = {
  href: Href;
  children: ReactNode;
};

// The chip as a link of its own, such as to the row's owner, which preloads its route on the
// user's intent, as the row's title does.
function PropertyLink({ href, children }: PropertyLinkProps) {
  const preloadProps = useRoutePreload(href);

  return (
    <Link href={href} {...preloadProps} className={styles.propertyTrigger} data-actionable>
      {children}
    </Link>
  );
}
