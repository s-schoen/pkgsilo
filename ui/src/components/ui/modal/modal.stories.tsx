import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";
import { DialogTrigger, Modal, ModalBody, ModalFooter } from "@/components/ui/modal";
import { TextField } from "@/components/ui/text-field";
import { m } from "@/paraglide/messages.js";

// The texts of the dialog, where a view passes messages.
const text = {
  open: "Create group",
  name: "Name",
  cancel: "Cancel",
  create: "Create",
};

const created = fn();

const meta = preview.meta({
  component: Modal,
  args: { title: "Create group", children: undefined },
  beforeEach: () => {
    created.mockClear();
  },
  render: (args) => (
    <DialogTrigger>
      <Button>{text.open}</Button>
      <Modal {...args}>
        {({ close }) => (
          <>
            <ModalBody>
              <TextField label={text.name} />
            </ModalBody>
            <ModalFooter>
              <Button variant="secondary" onPress={close}>
                {text.cancel}
              </Button>
              <Button
                onPress={() => {
                  created();
                  close();
                }}
              >
                {text.create}
              </Button>
            </ModalFooter>
          </>
        )}
      </Modal>
    </DialogTrigger>
  ),
});

// The trigger opens the dialog, named by its title, with the focus inside; an action closes it and
// the focus returns to the trigger.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: text.open });

    await userEvent.click(trigger);

    const dialog = await screen.findByRole("dialog", { name: args.title });
    await waitFor(async () => {
      await expect(dialog).toContainElement(document.activeElement as HTMLElement);
    });

    await userEvent.type(screen.getByRole("textbox", { name: text.name }), "maintainers");
    await userEvent.click(screen.getByRole("button", { name: text.create }));

    await expect(created).toHaveBeenCalledOnce();
    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    await expect(trigger).toHaveFocus();
  },
});

// Escape closes the dialog.
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
    await expect(created).not.toHaveBeenCalled();
  },
});

// Tab keeps the focus inside the dialog.
export const FocusStaysInside = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.open }));
    const dialog = await screen.findByRole("dialog");

    for (let i = 0; i < 5; i++) {
      await userEvent.tab();
      await expect(dialog).toContainElement(document.activeElement as HTMLElement);
    }
  },
});

// Open, in the small width.
export const Small = meta.story({
  args: { size: "sm", defaultOpen: true },
  play: async ({ args }) => {
    await expect(await screen.findByRole("dialog", { name: args.title })).toBeInTheDocument();
  },
});
