import { expect } from "storybook/test";

import preview from "#.storybook/preview";
import { IdentityTitle } from "@/features/identities/components/identity-title.tsx";
import { m } from "@/paraglide/messages.js";

const text = { name: "Ada Lovelace", long: "Ada Lovelace, Countess of Lovelace ".repeat(4) };

const meta = preview.meta({
  component: IdentityTitle,
  args: { name: text.name, isYou: false },
});

// Another identity's name.
export const Default = meta.story({
  play: async ({ canvas }) => {
    await expect(canvas.getByText(text.name)).toBeVisible();
    await expect(canvas.queryByText(m.identityYou())).not.toBeInTheDocument();
  },
});

// The signed-in identity's name, with the badge.
export const You = meta.story({
  args: { isYou: true },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(m.identityYou())).toBeVisible();
  },
});

// A long name ends in an ellipsis and keeps the badge in view.
export const LongName = meta.story({
  args: { name: text.long, isYou: true },
  decorators: [
    (Story) => (
      <div style={{ inlineSize: "16rem", overflow: "hidden" }}>
        <Story />
      </div>
    ),
  ],
  play: async ({ canvas }) => {
    await expect(canvas.getByText(m.identityYou())).toBeVisible();
    const name = canvas.getByText(text.long.trim(), { normalizer: (s) => s.trim() });
    await expect(name.scrollWidth).toBeGreaterThan(name.clientWidth);
  },
});

// The badge in German.
export const YouGerman = meta.story({
  args: { isYou: true },
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(m.identityYou({}, { locale: "de" }))).toBeVisible();
  },
});
