import IconPencil from "@tabler/icons-react/icons/IconPencil";
import IconTrash from "@tabler/icons-react/icons/IconTrash";
import { useRef, useState } from "react";
import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { ContextMenu, MenuItem, MenuSeparator } from "@/components/ui/menu";

import type { ContextMenuPoint, ContextMenuProps } from "@/components/ui/menu";
import type { KeyboardEvent, MouseEvent } from "react";

// The texts of the menu, where a view passes messages.
const text = {
  target: "Right-click me",
  label: "Actions for the file",
  rename: "Rename",
  delete: "Delete",
};

type Opened = { point?: ContextMenuPoint };

// An element with a context menu, as a view wires it: the right-click opens it at the pointer,
// Shift+F10 or the context-menu key below the element.
function FileWithMenu(
  props: Omit<ContextMenuProps<object>, "isOpen" | "onOpenChange" | "triggerRef">,
) {
  const ref = useRef<HTMLButtonElement>(null);
  const [opened, setOpened] = useState<Opened | undefined>();

  const onContextMenu = (event: MouseEvent<HTMLButtonElement>) => {
    event.preventDefault();
    event.currentTarget.focus();
    setOpened({ point: { x: event.clientX, y: event.clientY } });
  };
  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if ((event.shiftKey && event.key === "F10") || event.key === "ContextMenu") {
      event.preventDefault();
      setOpened({});
    }
  };

  return (
    <>
      <button
        ref={ref}
        type="button"
        onContextMenu={onContextMenu}
        onKeyDown={onKeyDown}
        style={{ padding: "var(--space-8)" }}
      >
        {text.target}
      </button>
      <ContextMenu
        {...props}
        isOpen={opened !== undefined}
        onOpenChange={(isOpen) => {
          if (!isOpen) {
            setOpened(undefined);
          }
        }}
        triggerRef={ref}
        point={opened?.point}
      />
    </>
  );
}

const meta = preview.meta({
  component: FileWithMenu,
  args: {
    "aria-label": text.label,
    onAction: fn(),
    children: [
      <MenuItem key="rename" id="rename" icon={IconPencil}>
        {text.rename}
      </MenuItem>,
      <MenuSeparator key="separator" />,
      <MenuItem key="delete" id="delete" icon={IconTrash} variant="danger">
        {text.delete}
      </MenuItem>,
    ],
  },
  argTypes: { children: { control: false } },
});

// A right-click opens the menu with its corner at the pointer, on its first item; a click on an
// item chooses it and closes the menu, and the focus returns to the element.
export const AtThePointer = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const target = canvas.getByRole("button", { name: text.target });
    const rect = target.getBoundingClientRect();
    const coords = { clientX: rect.x + 20, clientY: rect.y + 10 };

    await userEvent.pointer({ keys: "[MouseRight]", target, coords });

    const menu = await screen.findByRole("menu", { name: text.label });
    await waitFor(async () => {
      await expect(screen.getByRole("menuitem", { name: text.rename })).toHaveFocus();
    });
    const position = menu.getBoundingClientRect();
    // The popover's padding and border lie between its corner and the menu's.
    await expect(Math.abs(position.left - coords.clientX)).toBeLessThan(16);
    await expect(Math.abs(position.top - coords.clientY)).toBeLessThan(16);

    await userEvent.click(screen.getByRole("menuitem", { name: text.delete }));

    await expect(args.onAction).toHaveBeenCalledWith("delete", undefined);
    await waitFor(async () => {
      await expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });
    await expect(target).toHaveFocus();
  },
});

// Shift+F10 opens the menu below the focused element, the arrow keys move, and Escape closes it
// without choosing; the focus returns to the element.
export const FromTheKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const target = canvas.getByRole("button", { name: text.target });
    await userEvent.tab();
    await expect(target).toHaveFocus();

    await userEvent.keyboard("{Shift>}{F10}{/Shift}");

    const menu = await screen.findByRole("menu", { name: text.label });
    await waitFor(async () => {
      await expect(screen.getByRole("menuitem", { name: text.rename })).toHaveFocus();
    });
    await expect(menu.getBoundingClientRect().top).toBeGreaterThanOrEqual(
      target.getBoundingClientRect().bottom,
    );

    await userEvent.keyboard("{ArrowDown}");
    await expect(screen.getByRole("menuitem", { name: text.delete })).toHaveFocus();
    await userEvent.keyboard("{Escape}");

    await waitFor(async () => {
      await expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });
    await expect(target).toHaveFocus();
    await expect(args.onAction).not.toHaveBeenCalled();
  },
});

// The context-menu key opens it too, and Enter chooses the focused item.
export const ContextMenuKey = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.tab();
    await userEvent.keyboard("{ContextMenu}");

    await waitFor(async () => {
      await expect(screen.getByRole("menuitem", { name: text.rename })).toHaveFocus();
    });
    await userEvent.keyboard("{Enter}");

    await expect(args.onAction).toHaveBeenCalledWith("rename", undefined);
    await waitFor(async () => {
      await expect(canvas.getByRole("button", { name: text.target })).toHaveFocus();
    });
  },
});
