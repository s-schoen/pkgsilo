import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { Switch } from "@/components/ui/switch";

const meta = preview.meta({
  component: Switch,
  args: { children: "Sign-ups", onChange: fn() },
});

// A click on the text turns the setting on.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const toggle = canvas.getByRole("switch", { name: args.children });

    await userEvent.click(canvas.getByText(args.children));

    await expect(toggle).toBeChecked();
    await expect(args.onChange).toHaveBeenCalledWith(true);
  },
});

// Space toggles the focused switch, whose focus shows the ring.
export const ToggledWithKeyboard = meta.story({
  args: { defaultSelected: true },
  play: async ({ args, canvas, userEvent }) => {
    const toggle = canvas.getByRole("switch", { name: args.children });

    await userEvent.tab();
    await userEvent.keyboard(" ");

    await expect(toggle).toHaveFocus();
    await expect(toggle).not.toBeChecked();
    await expect(args.onChange).toHaveBeenCalledWith(false);
  },
});

export const On = meta.story({
  args: { defaultSelected: true },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("switch", { name: args.children })).toBeChecked();
  },
});

// A hint below the text describes the switch.
export const WithDescription = meta.story({
  args: { description: "Anyone may create an identity." },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("switch", { name: args.children })).toHaveAccessibleDescription(
      args.description ?? "",
    );
  },
});

// A disabled switch can't be toggled, on or off.
export const Disabled = meta.story({
  args: { isDisabled: true },
  render: (args) => (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-2)" }}>
      <Switch {...args} />
      <Switch {...args} defaultSelected />
    </div>
  ),
  play: async ({ args, canvas }) => {
    const [off, on] = canvas.getAllByRole("switch", { name: args.children });

    await expect(off).toBeDisabled();
    await expect(off).not.toBeChecked();
    await expect(on).toBeChecked();
  },
});
