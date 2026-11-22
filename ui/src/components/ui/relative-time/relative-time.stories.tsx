import { expect, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { RelativeTime } from "@/components/ui/relative-time";

const day = 24 * 60 * 60 * 1000;

// Three days ago and in two days, from when the story renders.
const threeDaysAgo = new Date(Date.now() - 3 * day);
const inTwoDays = new Date(Date.now() + 2 * day + 60 * 60 * 1000);

const meta = preview.meta({
  component: RelativeTime,
  args: { date: threeDaysAgo.toISOString() },
});

// A time in the past, as how long ago it was; its exact time is the <time>'s dateTime.
export const Past = meta.story({
  play: async ({ canvas }) => {
    const time = canvas.getByRole("img", { name: "3 days ago" });
    await expect(time).toHaveTextContent("3 days ago");
    await expect(time).toHaveAttribute("datetime", threeDaysAgo.toISOString());
  },
});

// A time to come, as how long until it does.
export const Future = meta.story({
  args: { date: inTwoDays },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("img", { name: "in 2 days" })).toBeVisible();
  },
});

// The keyboard focuses the time, which shows its date and time in a tooltip that describes it.
export const FocusedWithKeyboard = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();
    const time = canvas.getByRole("img", { name: "3 days ago" });
    await expect(time).toHaveFocus();

    const tooltip = await screen.findByRole("tooltip");
    await waitFor(async () => {
      await expect(tooltip).toBeVisible();
    });
    await expect(tooltip).toHaveTextContent(String(threeDaysAgo.getFullYear()));
    await expect(time).toHaveAccessibleDescription(tooltip.textContent);
  },
});

// In German, as the locale of the messages says it.
export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("img", { name: "vor 3 Tagen" })).toBeVisible();
  },
});
