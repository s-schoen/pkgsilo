import IconDots from "@tabler/icons-react/icons/IconDots";
import IconPencil from "@tabler/icons-react/icons/IconPencil";
import IconTrash from "@tabler/icons-react/icons/IconTrash";
import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";
import { Menu, MenuItem, MenuSection, MenuSeparator, MenuTrigger } from "@/components/ui/menu";

// The texts of the menu, where a view passes messages.
const text = {
  trigger: "Actions",
  rename: "Rename",
  members: "Members",
  delete: "Delete",
  group: "Group",
};

const meta = preview.meta({
  component: Menu,
  args: { onAction: fn() },
  render: (args) => (
    <MenuTrigger>
      <Button variant="ghost" icon={IconDots} aria-label={text.trigger} />
      <Menu {...args}>
        <MenuItem id="rename" icon={IconPencil}>
          {text.rename}
        </MenuItem>
        <MenuItem id="members">{text.members}</MenuItem>
        <MenuSeparator />
        <MenuItem id="delete" icon={IconTrash} variant="danger">
          {text.delete}
        </MenuItem>
      </Menu>
    </MenuTrigger>
  ),
});

// A click opens the menu, and a click on an item chooses it and closes the menu.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.trigger }));
    await userEvent.click(await screen.findByRole("menuitem", { name: text.rename }));

    // With the item's value, which a static item lacks.
    await expect(args.onAction).toHaveBeenCalledWith("rename", undefined);
    await waitFor(async () => {
      await expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });
  },
});

// Enter opens the menu on its first item, the arrow keys move, and Enter chooses; the focus returns
// to the trigger.
export const ChosenWithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: text.trigger });

    await userEvent.tab();
    await userEvent.keyboard("{Enter}");

    await waitFor(async () => {
      await expect(screen.getByRole("menuitem", { name: text.rename })).toHaveFocus();
    });

    await userEvent.keyboard("{ArrowDown}{Enter}");

    await expect(args.onAction).toHaveBeenCalledWith("members", undefined);
    await waitFor(async () => {
      await expect(trigger).toHaveFocus();
    });
  },
});

// Escape closes the menu without choosing.
export const ClosedWithEscape = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.trigger }));
    await screen.findByRole("menu");

    await userEvent.keyboard("{Escape}");

    await waitFor(async () => {
      await expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });
    await expect(args.onAction).not.toHaveBeenCalled();
  },
});

// An item that can't be chosen.
export const DisabledItem = meta.story({
  args: { disabledKeys: ["delete"] },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.trigger }));

    await expect(await screen.findByRole("menuitem", { name: text.delete })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  },
});

// Items grouped under a heading.
export const WithSection = meta.story({
  render: (args) => (
    <MenuTrigger defaultOpen>
      <Button variant="ghost" icon={IconDots} aria-label={text.trigger} />
      <Menu {...args}>
        <MenuSection title={text.group}>
          <MenuItem id="rename">{text.rename}</MenuItem>
          <MenuItem id="members">{text.members}</MenuItem>
        </MenuSection>
      </Menu>
    </MenuTrigger>
  ),
  play: async () => {
    // Once the menu has entered with its animation.
    await waitFor(async () => {
      await expect(screen.getByText(text.group)).toBeVisible();
    });
    await expect(screen.getAllByRole("menuitem")).toHaveLength(2);
  },
});

// The options of a menu with one of them selected.
const schemes = { system: "System", light: "Light", dark: "Dark" };

const selectionChanged = fn();

// A menu whose items are options, of which the selected one shows a check: Enter on another one
// selects it instead. The menu leaves its popover when it closes, so a view keeps the selection in
// selectedKeys.
export const SingleSelection = meta.story({
  args: {
    selectionMode: "single",
    defaultSelectedKeys: ["system"],
    onSelectionChange: selectionChanged,
  },
  beforeEach: () => {
    selectionChanged.mockClear();
  },
  render: (args) => (
    <MenuTrigger>
      <Button variant="ghost" icon={IconDots} aria-label={text.trigger} />
      <Menu {...args}>
        <MenuItem id="system">{schemes.system}</MenuItem>
        <MenuItem id="light">{schemes.light}</MenuItem>
        <MenuItem id="dark">{schemes.dark}</MenuItem>
      </Menu>
    </MenuTrigger>
  ),
  play: async ({ userEvent }) => {
    // Enter opens the menu on its first item.
    await userEvent.tab();
    await userEvent.keyboard("{Enter}");
    const system = await screen.findByRole("menuitemradio", { name: schemes.system });
    await expect(system).toHaveAttribute("aria-checked", "true");
    await expect(screen.getByRole("menuitemradio", { name: schemes.dark })).toHaveAttribute(
      "aria-checked",
      "false",
    );
    await waitFor(async () => {
      await expect(system).toHaveFocus();
    });

    await userEvent.keyboard("{ArrowDown}{ArrowDown}{Enter}");

    // React Aria's Selection, a Set of the selected ids.
    await expect(selectionChanged).toHaveBeenCalledOnce();
    await expect([...(selectionChanged.mock.calls[0]?.[0] ?? [])]).toEqual(["dark"]);
    await waitFor(async () => {
      await expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });
  },
});
