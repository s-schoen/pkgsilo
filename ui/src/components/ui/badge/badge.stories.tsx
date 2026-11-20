import IconCheck from "@tabler/icons-react/icons/IconCheck";
import { expect } from "storybook/test";

import preview from "#.storybook/preview";
import { Badge } from "@/components/ui/badge";

const meta = preview.meta({
  component: Badge,
  args: { children: "Member" },
  // An icon is a component, which the controls can't edit.
  argTypes: { icon: { control: false } },
});

// A plain label.
export const Neutral = meta.story({
  play: async ({ args, canvas }) => {
    await expect(canvas.getByText(args.children)).toBeVisible();
  },
});

// What's selected or new.
export const Accent = meta.story({
  args: { variant: "accent", children: "New" },
});

// What works, such as an active key.
export const Success = meta.story({
  args: { variant: "success", children: "Active" },
});

// What needs attention, such as a key that expires soon.
export const Warning = meta.story({
  args: { variant: "warning", children: "Expires soon" },
});

// What failed or ended, such as an expired key.
export const Danger = meta.story({
  args: { variant: "danger", children: "Expired" },
});

// The icon is decorative: the text alone names the status.
export const WithIcon = meta.story({
  args: { variant: "success", children: "Active", icon: IconCheck },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByText(args.children)).toBeVisible();
    await expect(canvas.getByText(args.children).querySelector("svg")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  },
});
