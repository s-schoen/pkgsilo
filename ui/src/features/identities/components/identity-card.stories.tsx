import { expect } from "storybook/test";

import preview from "#.storybook/preview";
import { IdentityCard } from "@/features/identities/components/identity-card.tsx";
import { m } from "@/paraglide/messages.js";

const ada = { id: "id-ada", username: "ada", displayName: "Ada Lovelace" };

const meta = preview.meta({
  component: IdentityCard,
  args: { identity: ada },
});

// An identity with its initials, its name and its handle.
export const Default = meta.story({
  play: async ({ canvas }) => {
    await expect(canvas.getByText(ada.displayName)).toBeVisible();
    await expect(canvas.getByText(`@${ada.username}`)).toBeVisible();
  },
});

// Without a display name, the username is the name, which the handle doesn't repeat.
export const UsernameOnly = meta.story({
  args: { identity: { id: ada.id, username: ada.username } },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(ada.username)).toBeVisible();
    await expect(canvas.queryByText(`@${ada.username}`)).not.toBeInTheDocument();
  },
});

// pkgsilo itself, for the nil UUID, with the app's icon.
export const Pkgsilo = meta.story({
  args: { identity: undefined, isPkgsilo: true },
  play: async ({ canvas }) => {
    await expect(canvas.getByText("pkgsilo")).toBeVisible();
  },
});

// An identity that the actor can't read.
export const Unknown = meta.story({
  args: { identity: undefined },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(m.identityUnknown())).toBeVisible();
  },
});
