import IconMenu2 from "@tabler/icons-react/icons/IconMenu2";
import { expect, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";
import { DialogTrigger, Drawer } from "@/components/ui/drawer";
import { m } from "@/paraglide/messages.js";

// The texts of the drawer, where a view passes messages.
const text = {
  open: "Open the menu",
  content: "The navigation of the app.",
  done: "Done",
};

const meta = preview.meta({
  component: Drawer,
  args: { title: "Menu", children: undefined },
  render: (args) => (
    <DialogTrigger>
      <Button variant="ghost" icon={IconMenu2} aria-label={text.open} />
      <Drawer {...args}>
        {({ close }) => (
          <>
            <p>{text.content}</p>
            <Button variant="secondary" onPress={close}>
              {text.done}
            </Button>
          </>
        )}
      </Drawer>
    </DialogTrigger>
  ),
});

// The trigger opens the drawer, named by its title, with the focus inside; its content can close it,
// and the focus returns to the trigger.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: text.open });

    await userEvent.click(trigger);

    const dialog = await screen.findByRole("dialog", { name: args.title });
    await waitFor(async () => {
      await expect(dialog).toContainElement(document.activeElement as HTMLElement);
    });

    await userEvent.click(screen.getByRole("button", { name: text.done }));

    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    await expect(trigger).toHaveFocus();
  },
});

// Escape closes the drawer.
export const ClosedWithEscape = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.open }));
    await screen.findByRole("dialog");

    await userEvent.keyboard("{Escape}");

    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  },
});

// The close button in the heading closes it too.
export const ClosedWithButton = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.open }));

    await userEvent.click(await screen.findByRole("button", { name: m.close() }));

    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  },
});

// Open, against the start of the screen.
export const Open = meta.story({
  args: { defaultOpen: true },
  play: async ({ args }) => {
    const dialog = await screen.findByRole("dialog", { name: args.title });
    await waitFor(async () => {
      await expect(dialog.getBoundingClientRect().left).toBe(0);
    });
  },
});
