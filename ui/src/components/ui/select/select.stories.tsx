import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { ListBoxItem } from "@/components/ui/list-box";
import { Select } from "@/components/ui/select";

import type { ComponentProps } from "react";

const roles = [
  { id: "reader", name: "Reader" },
  { id: "maintainer", name: "Maintainer" },
  { id: "admin", name: "Administrator" },
];

type Role = (typeof roles)[number];

// The Select of the roles: Select is generic over its items, which the stories fix.
function RoleSelect(props: Omit<ComponentProps<typeof Select<Role>>, "items" | "children">) {
  return (
    <Select {...props} items={roles}>
      {(role) => <ListBoxItem id={role.id}>{role.name}</ListBoxItem>}
    </Select>
  );
}

const meta = preview.meta({
  component: RoleSelect,
  args: { label: "Role", placeholder: "Pick a role", onChange: fn() },
});

// A click opens the list, and a click on an option picks it and closes the list.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: new RegExp(args.label) });

    await expect(trigger).toHaveTextContent(args.placeholder ?? "");

    await userEvent.click(trigger);
    await userEvent.click(await screen.findByRole("option", { name: "Maintainer" }));

    await expect(trigger).toHaveTextContent("Maintainer");
    await expect(args.onChange).toHaveBeenCalledWith("maintainer");
    // The list leaves with its animation.
    await waitFor(async () => {
      await expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    });
  },
});

// The keyboard opens the list with the arrow keys, moves through it and picks with Enter.
export const PickedWithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: new RegExp(args.label) });

    await userEvent.tab();
    await userEvent.keyboard("{ArrowDown}");
    await screen.findByRole("listbox");
    await userEvent.keyboard("{ArrowDown}{Enter}");

    await expect(trigger).toHaveTextContent("Maintainer");
    // The focus returns to the trigger once the list has left.
    await waitFor(async () => {
      await expect(trigger).toHaveFocus();
    });
  },
});

// The list opens with the picked option checked.
export const Open = meta.story({
  args: { defaultValue: "admin", defaultOpen: true },
  play: async () => {
    await expect(await screen.findByRole("option", { name: "Administrator" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  },
});

export const WithDescription = meta.story({
  args: { description: "Maintainers may change the group's members." },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByText(args.description ?? "")).toBeVisible();
  },
});

// An invalid select shows its error.
export const Invalid = meta.story({
  args: { isInvalid: true, errorMessage: "Pick a role." },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("alert")).toHaveTextContent(args.errorMessage ?? "");
  },
});

// A disabled select doesn't open.
export const Disabled = meta.story({
  args: { isDisabled: true, defaultValue: "reader" },
  play: async ({ args, canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: new RegExp(args.label) });

    await userEvent.click(trigger);

    await expect(trigger).toBeDisabled();
    await expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  },
});
