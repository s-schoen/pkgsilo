import IconUser from "@tabler/icons-react/icons/IconUser";
import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { ToggleButton } from "@/components/ui/toggle-button";

const meta = preview.meta({
  component: ToggleButton,
  args: { children: "Archived", onChange: fn() },
  // An icon is a component, which the controls can't edit.
  argTypes: { icon: { control: false } },
});

// A click presses the button, and another releases it.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const button = canvas.getByRole("button", { name: args.children });

    await expect(button).toHaveAttribute("aria-pressed", "false");

    await userEvent.click(button);

    await expect(button).toHaveAttribute("aria-pressed", "true");
    await expect(args.onChange).toHaveBeenLastCalledWith(true);

    await userEvent.click(button);

    await expect(button).toHaveAttribute("aria-pressed", "false");
    await expect(args.onChange).toHaveBeenLastCalledWith(false);
  },
});

// Space and Enter toggle it, and its focus shows the ring.
export const WithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const button = canvas.getByRole("button", { name: args.children });

    await userEvent.tab();
    await userEvent.keyboard(" ");

    await expect(button).toHaveAttribute("aria-pressed", "true");
    await expect(button).toHaveAttribute("data-focus-visible");

    await userEvent.keyboard("{Enter}");

    await expect(button).toHaveAttribute("aria-pressed", "false");
  },
});

export const Selected = meta.story({
  args: { defaultSelected: true },
});

export const WithIcon = meta.story({
  args: { icon: IconUser, children: "Owner" },
});

// A text longer than the button's limit is cut with an ellipsis, and still names it.
export const LongText = meta.story({
  args: { children: "Last updated by a maintainer" },
  play: async ({ args, canvas }) => {
    const button = canvas.getByRole("button", { name: args.children });
    const label = button.lastElementChild;

    await expect(label?.scrollWidth).toBeGreaterThan(label?.clientWidth ?? 0);
  },
});

// A disabled toggle can't be pressed.
export const Disabled = meta.story({
  args: { isDisabled: true },
  play: async ({ args, canvas, userEvent }) => {
    const button = canvas.getByRole("button", { name: args.children });

    await userEvent.click(button);

    await expect(button).toBeDisabled();
    await expect(args.onChange).not.toHaveBeenCalled();
  },
});

export const DisabledSelected = meta.story({
  args: { isDisabled: true, defaultSelected: true },
});
