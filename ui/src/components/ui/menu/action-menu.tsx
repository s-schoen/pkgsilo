import IconDots from "@tabler/icons-react/icons/IconDots";

import { Button } from "@/components/ui/button";
import { Menu, MenuItem, MenuTrigger } from "@/components/ui/menu/menu.tsx";

import type { TablerIcon } from "@tabler/icons-react";
import type { PopoverProps } from "react-aria-components/Popover";

// An action of an ActionMenu.
export type ActionMenuItem = {
  id: string;
  // The action's text: a message.
  label: string;
  // An icon of @tabler/icons-react before the text, decorative.
  icon?: TablerIcon;
  // danger for an action that destroys data.
  variant?: "neutral" | "danger";
  // false hides the item, as for an action that the actor may not take; a named boolean of a
  // resource's permissions hook, such as canDelete.
  isVisible?: boolean;
  onAction: () => void;
};

type ActionMenuProps = {
  // The trigger's name, a message: "Actions for the identity".
  "aria-label": string;
  items: Array<ActionMenuItem>;
  // Where the menu opens: below the trigger's end, as at the end of a page's header.
  placement?: PopoverProps["placement"];
};

// The ⋯ menu of an object, such as a details page's header holds: a ghost button with the dots
// that opens a Menu of the visible items. Without a visible item, it renders nothing, not even the
// button:
// <ActionMenu aria-label={m.identityActions()} items={[
//   { id: "delete", label: m.identityDelete(), icon: IconTrash, variant: "danger",
//     isVisible: canDelete, onAction: () => setDeleteOpen(true) },
// ]} />.
export function ActionMenu({ items, placement = "bottom end", ...props }: ActionMenuProps) {
  const visible = items.filter((item) => item.isVisible !== false);

  if (visible.length === 0) {
    return null;
  }

  return (
    <MenuTrigger>
      <Button variant="ghost" icon={IconDots} aria-label={props["aria-label"]} />
      <Menu
        placement={placement}
        onAction={(id) => {
          visible.find((item) => item.id === id)?.onAction();
        }}
      >
        {visible.map(({ id, label, icon, variant }) => (
          <MenuItem key={id} id={id} icon={icon} variant={variant}>
            {label}
          </MenuItem>
        ))}
      </Menu>
    </MenuTrigger>
  );
}
