import { expect } from "storybook/test";

import preview from "#.storybook/preview";
import { Spinner } from "@/components/ui/spinner";
import { m } from "@/paraglide/messages.js";

const meta = preview.meta({
  component: Spinner,
});

// Says "Loading" to a screen reader.
export const Default = meta.story({
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("progressbar", { name: m.loading() })).toBeVisible();
  },
});

// A view names what's loading.
export const WithLabel = meta.story({
  args: { label: "Loading groups" },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("progressbar", { name: args.label })).toBeVisible();
  },
});

export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(
      canvas.getByRole("progressbar", { name: m.loading({}, { locale: "de" }) }),
    ).toBeVisible();
  },
});
