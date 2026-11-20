import { Dialog, DialogTrigger, Heading } from "react-aria-components/Dialog";
import { expect, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";
import { Popover } from "@/components/ui/popover";

const text = {
  open: "Details",
  heading: "Key ci-bot",
  body: "Created 4 days ago, expires in 26 days.",
};

const meta = preview.meta({
  component: Popover,
  args: { children: undefined },
  render: (args) => (
    <DialogTrigger>
      <Button variant="secondary">{text.open}</Button>
      <Popover {...args}>
        <Dialog aria-labelledby="popover-heading">
          <Heading id="popover-heading">{text.heading}</Heading>
          <p>{text.body}</p>
        </Dialog>
      </Popover>
    </DialogTrigger>
  ),
});

// A panel next to its trigger, which a click opens and Escape closes.
export const WithDialog = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.open }));

    await expect(await screen.findByRole("dialog", { name: text.heading })).toHaveTextContent(
      text.body,
    );

    await userEvent.keyboard("{Escape}");

    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  },
});
