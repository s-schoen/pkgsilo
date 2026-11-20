import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { ComboBox } from "@/components/ui/combo-box";
import { ListBoxItem } from "@/components/ui/list-box";
import { m } from "@/paraglide/messages.js";

import type { ComponentProps } from "react";

const identities = [
  { id: "alice", name: "alice" },
  { id: "bob", name: "bob" },
  { id: "ci-bot", name: "ci-bot" },
  { id: "mallory", name: "mallory" },
];

type Identity = (typeof identities)[number];

// The ComboBox of the identities: ComboBox is generic over its items, which the stories fix.
function IdentityComboBox(
  props: Omit<ComponentProps<typeof ComboBox<Identity>>, "items" | "children">,
) {
  return (
    <ComboBox {...props} items={identities}>
      {(identity) => <ListBoxItem id={identity.id}>{identity.name}</ListBoxItem>}
    </ComboBox>
  );
}

const meta = preview.meta({
  component: IdentityComboBox,
  args: { label: "Member", placeholder: "Search identities", onChange: fn() },
});

// Typing filters the options, and a click picks one.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const input = canvas.getByRole("combobox", { name: args.label });

    await userEvent.type(input, "al");

    // Once the list has entered with its animation.
    await waitFor(async () => {
      await expect(screen.getByRole("option", { name: "alice" })).toBeVisible();
    });
    await expect(screen.queryByRole("option", { name: "bob" })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("option", { name: "alice" }));

    await expect(input).toHaveValue("alice");
    await expect(args.onChange).toHaveBeenCalledWith("alice");
  },
});

// The keyboard opens the list from the input, moves through it and picks with Enter.
export const PickedWithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const input = canvas.getByRole("combobox", { name: args.label });

    await userEvent.tab();
    await userEvent.keyboard("{ArrowDown}");
    await screen.findByRole("listbox");
    await userEvent.keyboard("{ArrowDown}{Enter}");

    await expect(input).toHaveValue("bob");
    await expect(input).toHaveFocus();
  },
});

// The button opens the whole list.
export const OpenedWithButton = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button"));

    await expect(await screen.findAllByRole("option")).toHaveLength(identities.length);
  },
});

// A text that matches nothing says so.
export const NoResults = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("combobox", { name: args.label }), "zz");

    // Once the list has entered with its animation.
    await waitFor(async () => {
      await expect(screen.getByText(m.noResults())).toBeVisible();
    });
  },
});

// An invalid combobox shows its error.
export const Invalid = meta.story({
  args: { isInvalid: true, errorMessage: "Pick an identity." },
  play: async ({ args, canvas }) => {
    const input = canvas.getByRole("combobox", { name: args.label });

    await expect(canvas.getByRole("alert")).toHaveTextContent(args.errorMessage ?? "");
    await expect(input).toBeInvalid();
  },
});

export const Disabled = meta.story({
  args: { isDisabled: true },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("combobox", { name: args.label })).toBeDisabled();
  },
});

// React Aria's own strings, such as the button's name, follow the locale.
export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("button")).toHaveAccessibleName(/Empfehlungen anzeigen/);
  },
});
