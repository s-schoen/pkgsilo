import { expect } from "storybook/test";

import preview from "#.storybook/preview";
import { Separator } from "@/components/ui/separator";

// The sample content on either side of the separator.
const before = "Identities";
const after = "Groups";

const meta = preview.meta({
  component: Separator,
});

// Between blocks stacked on top of each other.
export const Horizontal = meta.story({
  render: (args) => (
    <div>
      <p>{before}</p>
      <Separator {...args} />
      <p>{after}</p>
    </div>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("separator")).toBeVisible();
  },
});

// Between items side by side, as tall as their row, which the story lays out as a parent would.
export const Vertical = meta.story({
  args: { orientation: "vertical" },
  render: (args) => (
    <div style={{ display: "flex", gap: "var(--space-3)", alignItems: "center" }}>
      <span>{before}</span>
      <Separator {...args} />
      <span>{after}</span>
    </div>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("separator")).toHaveAttribute("aria-orientation", "vertical");
  },
});
