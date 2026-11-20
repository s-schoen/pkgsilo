import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { ListBox, ListBoxItem } from "@/components/ui/list-box";
import { m } from "@/paraglide/messages.js";

import type { ComponentProps } from "react";

const groups = [
  { id: "admins", name: "Administrators" },
  { id: "maintainers", name: "Maintainers" },
  { id: "readers", name: "Readers" },
];

type Group = (typeof groups)[number];

// The ListBox of the groups: ListBox is generic over its items, which the stories fix.
function GroupListBox(props: Omit<ComponentProps<typeof ListBox<Group>>, "children">) {
  return (
    <ListBox {...props}>{(group) => <ListBoxItem id={group.id}>{group.name}</ListBoxItem>}</ListBox>
  );
}

const meta = preview.meta({
  component: GroupListBox,
  args: {
    "aria-label": "Groups",
    items: groups,
    selectionMode: "single",
    onSelectionChange: fn(),
  },
});

// A click selects an option, which shows its check.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const option = canvas.getByRole("option", { name: "Maintainers" });

    await userEvent.click(option);

    await expect(option).toHaveAttribute("aria-selected", "true");
    await expect(args.onSelectionChange).toHaveBeenCalledOnce();
  },
});

// The arrow keys move between the options, and Enter selects one.
export const SelectedWithKeyboard = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();
    await userEvent.keyboard("{ArrowDown}{Enter}");

    await expect(canvas.getByRole("option", { name: "Maintainers" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  },
});

// An option that can't be picked.
export const DisabledOption = meta.story({
  args: { disabledKeys: ["admins"] },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("option", { name: "Administrators" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  },
});

// A list without options says so.
export const Empty = meta.story({
  args: { items: [] },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(m.noResults())).toBeVisible();
  },
});
