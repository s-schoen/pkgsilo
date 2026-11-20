import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { Link } from "@/components/ui/link";

const sentence = "This page doesn't exist. ";

const meta = preview.meta({
  component: Link,
  args: { children: "Go to the start page", href: "/" as const },
});

// A link to a route, in the accent and underlined. The stories have no router: it's an <a>.
export const Primary = meta.story({
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("link", { name: args.children })).toHaveAttribute("href", "/");
  },
});

// A quiet link among others.
export const Secondary = meta.story({
  args: { variant: "secondary" },
});

// The keyboard reaches the link, whose focus shows the ring, and Enter follows it: without href,
// it runs onPress.
export const PressedWithKeyboard = meta.story({
  args: { href: undefined, onPress: fn() },
  play: async ({ args, canvas, userEvent }) => {
    const link = canvas.getByRole("link", { name: args.children });

    await userEvent.tab();
    await userEvent.keyboard("{Enter}");

    await expect(link).toHaveFocus();
    await expect(link).toHaveAttribute("data-focus-visible");
    await expect(args.onPress).toHaveBeenCalledOnce();
  },
});

// A link in a sentence, which its underline sets apart from the text.
export const InText = meta.story({
  render: (args) => (
    <p>
      {sentence}
      <Link {...args} />
    </p>
  ),
});

export const Disabled = meta.story({
  args: { isDisabled: true },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("link", { name: args.children })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  },
});
