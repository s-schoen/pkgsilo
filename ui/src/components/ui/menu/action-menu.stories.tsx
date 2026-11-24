import IconPencil from "@tabler/icons-react/icons/IconPencil";
import IconTrash from "@tabler/icons-react/icons/IconTrash";
import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { ActionMenu } from "@/components/ui/menu";

import type { ActionMenuItem } from "@/components/ui/menu";

// The texts of the menu, where a view passes messages.
const text = {
  trigger: "Actions for the identity",
  rename: "Rename",
  delete: "Delete",
};

const rename = fn();
const remove = fn();

// Rename and Delete, both visible.
const items: Array<ActionMenuItem> = [
  { id: "rename", label: text.rename, icon: IconPencil, onAction: rename },
  { id: "delete", label: text.delete, icon: IconTrash, variant: "danger", onAction: remove },
];

const meta = preview.meta({
  component: ActionMenu,
  args: { "aria-label": text.trigger, items },
  beforeEach: () => {
    rename.mockClear();
    remove.mockClear();
  },
  // The menu opens below the trigger's end, as at the end of a header.
  decorators: [
    (Story) => (
      <div style={{ display: "flex", justifyContent: "flex-end", inlineSize: "20rem" }}>
        <Story />
      </div>
    ),
  ],
});

// The ⋯ button opens the menu of the items, and choosing one calls its onAction and closes it.
export const Default = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.trigger }));
    const renamed = await screen.findByRole("menuitem", { name: text.rename });
    // Once the menu has entered with its animation.
    await waitFor(async () => {
      await expect(renamed).toBeVisible();
    });

    await userEvent.click(screen.getByRole("menuitem", { name: text.delete }));

    await expect(remove).toHaveBeenCalledOnce();
    await expect(rename).not.toHaveBeenCalled();
    await waitFor(async () => {
      await expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });
  },
});

// An item that isn't visible, as an action the actor may not take, isn't in the menu.
export const HiddenItem = meta.story({
  args: {
    items: items.map((item) => (item.id === "delete" ? { ...item, isVisible: false } : item)),
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.trigger }));

    const renamed = await screen.findByRole("menuitem", { name: text.rename });
    // Once the menu has entered with its animation.
    await waitFor(async () => {
      await expect(renamed).toBeVisible();
    });
    await expect(screen.getAllByRole("menuitem")).toHaveLength(1);
  },
});

// Without a visible item, there's no menu and no button to open it.
export const NoVisibleItem = meta.story({
  args: {
    items: [{ id: "delete", label: text.delete, isVisible: false, onAction: remove }],
  },
  play: async ({ canvas }) => {
    await expect(canvas.queryByRole("button", { name: text.trigger })).not.toBeInTheDocument();
  },
});
