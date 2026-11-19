import IconPlus from "@tabler/icons-react/icons/IconPlus";
import IconTrash from "@tabler/icons-react/icons/IconTrash";
import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";

const variants = ["primary", "secondary", "tertiary", "ghost", "danger", "danger-outline"] as const;

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

// The least actions of a view: tertiary has a fill, ghost only when it's hovered.
export const Tertiary = meta.story({
  args: { variant: "tertiary", children: "Cancel" },
});

export const Ghost = meta.story({
  args: { variant: "ghost", children: "Cancel" },
});

// An action that destroys data.
export const Danger = meta.story({
  args: { variant: "danger", children: "Delete" },
});

// An action that destroys data, next to others in a view.
export const DangerOutline = meta.story({
  args: { variant: "danger-outline", children: "Delete" },
});

// Each size next to the others, so that their heights compare.
export const Sizes = meta.story({
  render: (args) => (
    <div style={{ display: "flex", alignItems: "center", gap: "var(--space-2)" }}>
      <Button {...args} size="sm" />
      <Button {...args} size="md" />
      <Button {...args} size="lg" />
    </div>
  ),
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

// Every variant disabled, each in its own colors.
export const DisabledVariants = meta.story({
  args: { isDisabled: true },
  render: (args) => (
    <div style={{ display: "flex", gap: "var(--space-2)" }}>
      {variants.map((variant) => (
        <Button key={variant} {...args} variant={variant} />
      ))}
    </div>
  ),
  play: async ({ canvas }) => {
    for (const button of canvas.getAllByRole("button", { name: "Save" })) {
      await expect(button).toBeDisabled();
    }
  },
});

// Every variant in the dark color scheme, so that the accessibility check tests its contrast too.
export const Dark = meta.story({
  globals: { colorScheme: "dark" },
  render: (args) => (
    <div style={{ display: "flex", gap: "var(--space-2)" }}>
      {variants.map((variant) => (
        <Button key={variant} {...args} variant={variant} />
      ))}
    </div>
  ),
});
