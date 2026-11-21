import { expect } from "storybook/test";

import preview from "#.storybook/preview";
import { HomePage } from "@/features/home/components/home-page.tsx";
import { m } from "@/paraglide/messages.js";

const meta = preview.meta({
  component: HomePage,
});

// The page's title is its heading.
export const English = meta.story({
  globals: { locale: "en" },
  play: async ({ canvas }) => {
    await expect(
      canvas.getByRole("heading", { level: 1, name: m.welcome({}, { locale: "en" }) }),
    ).toBeVisible();
  },
});

// The locale of the toolbar reaches the messages.
export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(m.welcome({}, { locale: "de" }))).toBeVisible();
  },
});
