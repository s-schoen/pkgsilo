import { expect, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";
import { Toaster, showToast, toastQueue } from "@/components/ui/toast";
import { m } from "@/paraglide/messages.js";

import type { ToastContent } from "@/components/ui/toast";

const text = { show: "Save" };

// A button that shows the story's toast, and the region that shows it, as the root route renders it.
function ToastDemo(content: ToastContent) {
  return (
    <>
      <Button
        onPress={() => {
          showToast(content);
        }}
      >
        {text.show}
      </Button>
      <Toaster />
    </>
  );
}

const meta = preview.meta({
  component: ToastDemo,
  args: { title: "Group created", variant: "success" as const },
  // The queue is the app's, which outlives a story.
  beforeEach: () => {
    toastQueue.clear();
  },
});

// An action that worked: the toast shows in the notifications region, as a status.
export const Success = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.show }));

    const toast = await screen.findByRole("alertdialog", { name: args.title });
    await expect(toast).toBeVisible();
    await expect(screen.getByRole("region")).toContainElement(toast);
  },
});

// More about it, below the title.
export const WithDescription = meta.story({
  args: {
    title: "Key created",
    description: "Copy it now: it won't be shown again.",
    variant: "neutral",
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.show }));

    await expect(await screen.findByRole("alertdialog", { name: args.title })).toHaveTextContent(
      args.description ?? "",
    );
  },
});

// An action that failed: the toast stays until the user dismisses it.
export const Danger = meta.story({
  args: { title: "The group couldn't be deleted.", variant: "danger" },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: text.show }));
    const toast = await screen.findByRole("alertdialog", { name: args.title });

    await userEvent.click(screen.getByRole("button", { name: m.dismiss() }));

    await waitFor(async () => {
      await expect(toast).not.toBeInTheDocument();
    });
  },
});

// Several toasts stack, the newest at the bottom.
export const Stacked = meta.story({
  play: async ({ canvas, userEvent }) => {
    const button = canvas.getByRole("button", { name: text.show });

    await userEvent.click(button);
    await userEvent.click(button);

    await waitFor(async () => {
      await expect(screen.getAllByRole("alertdialog")).toHaveLength(2);
    });
  },
});
