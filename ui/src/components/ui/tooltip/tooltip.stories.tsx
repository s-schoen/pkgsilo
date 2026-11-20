import IconTrash from "@tabler/icons-react/icons/IconTrash";
import { expect, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipTrigger } from "@/components/ui/tooltip";

const text = { delete: "Delete group" };

const meta = preview.meta({
  component: Tooltip,
  args: { children: text.delete },
  render: (args) => (
    <TooltipTrigger>
      <Button variant="secondary" icon={IconTrash} aria-label={text.delete} />
      <Tooltip {...args} />
    </TooltipTrigger>
  ),
});

// The keyboard's focus shows the tooltip at once, which describes the button.
export const ShownOnFocus = meta.story({
  play: async ({ canvas, userEvent }) => {
    const button = canvas.getByRole("button", { name: text.delete });

    await userEvent.tab();

    await expect(await screen.findByRole("tooltip")).toHaveTextContent(text.delete);
    await expect(button).toHaveAccessibleDescription(text.delete);
  },
});

// The pointer shows it after a rest on the button, and hides it when it leaves.
export const ShownOnHover = meta.story({
  play: async ({ canvas, canvasElement, userEvent }) => {
    const button = canvas.getByRole("button", { name: text.delete });
    // React Aria shows a tooltip on hover only while the pointer is the user's last input, and the
    // earlier stories of the page used the keyboard.
    await userEvent.click(canvasElement);

    await userEvent.hover(button);

    await expect(await screen.findByRole("tooltip", {}, { timeout: 2000 })).toBeInTheDocument();

    await userEvent.unhover(button);

    await waitFor(async () => {
      await expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
    });
  },
});

// Escape hides it.
export const HiddenWithEscape = meta.story({
  play: async ({ userEvent }) => {
    await userEvent.tab();
    await screen.findByRole("tooltip");

    await userEvent.keyboard("{Escape}");

    await waitFor(async () => {
      await expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
    });
  },
});
