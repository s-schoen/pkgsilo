import { delay } from "msw";
import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { DialogTrigger } from "@/components/ui/modal";
import { m } from "@/paraglide/messages.js";

const text = { open: "Delete group" };

const meta = preview.meta({
  component: ConfirmDialog,
  args: {
    title: "Delete the group maintainers?",
    description: "Its members lose the permissions that the group grants them.",
    confirmLabel: "Delete",
    onConfirm: fn(),
  },
  render: (args) => (
    <DialogTrigger>
      <Button variant="danger-outline">{text.open}</Button>
      <ConfirmDialog {...args} />
    </DialogTrigger>
  ),
});

// The dialog asks, with the focus on Cancel; confirming runs the action and closes it.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.open }));

    const dialog = await screen.findByRole("alertdialog", { name: args.title });
    await expect(dialog).toHaveAccessibleDescription(args.description);
    await waitFor(async () => {
      await expect(screen.getByRole("button", { name: m.cancel() })).toHaveFocus();
    });

    await userEvent.click(screen.getByRole("button", { name: args.confirmLabel }));

    await expect(args.onConfirm).toHaveBeenCalledOnce();
    await waitFor(async () => {
      await expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
  },
});

// Enter on the focused Cancel cancels: nothing runs.
export const CancelledWithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.open }));
    await screen.findByRole("alertdialog");
    await waitFor(async () => {
      await expect(screen.getByRole("button", { name: m.cancel() })).toHaveFocus();
    });

    await userEvent.keyboard("{Enter}");

    await waitFor(async () => {
      await expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    await expect(args.onConfirm).not.toHaveBeenCalled();
  },
});

// While the action runs, the confirm button is pending and Cancel is disabled.
export const Pending = meta.story({
  args: {
    onConfirm: fn(async () => {
      await delay(500);
    }),
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.open }));
    const confirm = await screen.findByRole("button", { name: args.confirmLabel });

    await userEvent.click(confirm);

    await expect(confirm).toHaveAttribute("data-pending");
    await expect(screen.getByRole("button", { name: m.cancel() })).toBeDisabled();
    await waitFor(async () => {
      await expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
  },
});

// A failed action keeps the dialog open, so that the user can try again.
export const Failed = meta.story({
  args: {
    onConfirm: fn(() => Promise.reject(new Error("The API failed."))),
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.open }));
    const confirm = await screen.findByRole("button", { name: args.confirmLabel });

    await userEvent.click(confirm);

    await expect(args.onConfirm).toHaveBeenCalledOnce();
    await waitFor(async () => {
      await expect(confirm).not.toHaveAttribute("data-pending");
    });
    await expect(screen.getByRole("alertdialog")).toBeInTheDocument();
  },
});

// An action that doesn't destroy data, without the warning.
export const Primary = meta.story({
  args: {
    variant: "primary",
    title: "Publish the policy?",
    description: "It applies to every request from now on.",
    confirmLabel: "Publish",
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.open }));

    await expect(await screen.findByRole("alertdialog", { name: args.title })).toBeInTheDocument();
  },
});
