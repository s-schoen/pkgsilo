import IconPlus from "@tabler/icons-react/icons/IconPlus";
import IconTrash from "@tabler/icons-react/icons/IconTrash";
import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";

const meta = preview.meta({
  component: Button,
  args: { children: "Save", onPress: fn() },
  // An icon is a component, which the controls can't edit.
  argTypes: { icon: { control: false } },
});

// The main action of a view.
export const Primary = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: "Save" }));

    await expect(args.onPress).toHaveBeenCalledOnce();
  },
});

// The keyboard presses the button too, and its focus shows the ring.
export const PressedWithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const button = canvas.getByRole("button", { name: "Save" });

    await userEvent.tab();
    await userEvent.keyboard("{Enter}");

    await expect(button).toHaveFocus();
    await expect(button).toHaveAttribute("data-focus-visible");
    await expect(args.onPress).toHaveBeenCalledOnce();
  },
});

export const Secondary = meta.story({
  args: { variant: "secondary", children: "Cancel" },
});

// An action that destroys data.
export const Danger = meta.story({
  args: { variant: "danger", children: "Delete" },
});

export const WithIcon = meta.story({
  args: { icon: IconPlus, children: "Add" },
});

// An icon alone, so the button is named by its aria-label.
export const IconOnly = meta.story({
  args: { icon: IconTrash, children: undefined, "aria-label": "Delete", variant: "secondary" },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: "Delete" }));

    await expect(args.onPress).toHaveBeenCalledOnce();
  },
});

// A disabled button can't be pressed.
export const Disabled = meta.story({
  args: { isDisabled: true },
  play: async ({ args, canvas, userEvent }) => {
    const button = canvas.getByRole("button", { name: "Save" });

    await userEvent.click(button);

    await expect(button).toBeDisabled();
    await expect(args.onPress).not.toHaveBeenCalled();
  },
});

// Every variant in the dark color scheme, so that the accessibility check tests its contrast too.
export const Dark = meta.story({
  globals: { colorScheme: "dark" },
  render: (args) => (
    <div style={{ display: "flex", gap: "var(--space-2)" }}>
      <Button {...args} variant="primary" />
      <Button {...args} variant="secondary" />
      <Button {...args} variant="danger" />
    </div>
  ),
});
