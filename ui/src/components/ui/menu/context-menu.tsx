import { Menu as AriaMenu } from "react-aria-components/Menu";

import { Popover } from "@/components/ui/popover";

import type { RefObject } from "react";
import type { MenuProps as AriaMenuProps } from "react-aria-components/Menu";

import styles from "@/components/ui/menu/menu.module.css";

// A point in the window, as a MouseEvent's clientX and clientY.
export type ContextMenuPoint = { x: number; y: number };

export type ContextMenuProps<T> = Omit<AriaMenuProps<T>, "className" | "style" | "onClose"> & {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  // The element that the menu belongs to, such as a list's row. Focus it before the menu opens:
  // the focus returns to it when the menu closes.
  triggerRef: RefObject<Element | null>;
  // Where the pointer opened the menu; without one, as from the keyboard, the menu opens below the
  // start of triggerRef.
  point?: ContextMenuPoint;
};

// A Menu that opens on its own, at the pointer or at an element, rather than from a button: the
// menu of a right-click, Shift+F10 or the context-menu key. The view decides when it opens and
// what it's about, and renders it once, not per element:
// const [menu, setMenu] = useState<ContextMenuPoint | undefined>();
// <div ref={ref} onContextMenu={(event) => { event.preventDefault(); ref.current.focus(); … }}>
// <ContextMenu isOpen={…} onOpenChange={…} triggerRef={ref} point={menu} aria-label={…}>
//   <MenuItem id="rename">{m.rename()}</MenuItem>
// </ContextMenu>.
// It focuses its first item, the arrow keys move, Enter chooses an item and closes the menu, as
// Escape or a click outside does, and the focus returns to the element that had it.
export function ContextMenu<T extends object>({
  isOpen,
  onOpenChange,
  triggerRef,
  point,
  ...props
}: ContextMenuProps<T>) {
  return (
    <Popover
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      triggerRef={triggerRef}
      placement="bottom start"
      // At the pointer, the menu's corner touches it, as the system's menus do.
      offset={point ? 0 : undefined}
      getTargetRect={point && (() => new DOMRect(point.x, point.y, 0, 0))}
      // A second right-click on the menu keeps the browser's own menu away.
      onContextMenu={(event) => {
        event.preventDefault();
      }}
    >
      <AriaMenu
        // React Aria's prop, not the DOM's: the menu focuses its first item once it opens, as a
        // menu that the keyboard opens does.
        // oxlint-disable-next-line jsx-a11y/no-autofocus
        autoFocus="first"
        {...props}
        onClose={() => {
          onOpenChange(false);
        }}
        className={styles.menu}
      />
    </Popover>
  );
}
