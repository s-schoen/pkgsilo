// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";
import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { getListIdentitiesMockHandler } from "@/api/gen/client/identities/identities.msw.ts";
import { IdentityReference } from "@/features/identities/components/identity-reference.tsx";
import { PKGSILO_ID } from "@/features/identities/utils/identity-name.ts";
import { m } from "@/paraglide/messages.js";

const ada = {
  id: "00000000-0000-4000-8000-000000000002",
  username: "ada",
  displayName: "Ada Lovelace",
};

// The router that React Aria's links navigate through, as root-layout.tsx provides it.
const navigate = fn();

const meta = preview.meta({
  component: IdentityReference,
  args: { id: ada.id },
  beforeEach: ({ msw }) => {
    navigate.mockClear();
    msw.use(
      getListIdentitiesMockHandler({
        data: [ada],
        pagination: { offset: 0, limit: 100, nextOffset: null },
      }),
    );
  },
  decorators: [
    (Story) => (
      <RouterProvider navigate={navigate}>
        <Story />
      </RouterProvider>
    ),
  ],
});

// An identity that the actor reads: its avatar and its name, a link to its page.
export const Identity = meta.story({
  play: async ({ canvas, userEvent }) => {
    const link = await canvas.findByRole("link", { name: ada.displayName });
    await expect(link).toHaveAttribute("href", `/identities/${ada.id}`);

    await userEvent.click(link);
    await expect(navigate).toHaveBeenCalledWith(`/identities/${ada.id}`, undefined);
  },
});

// pkgsilo itself, for the nil UUID, with the app's icon and no link.
export const Pkgsilo = meta.story({
  args: { id: PKGSILO_ID },
  play: async ({ canvas }) => {
    await expect(canvas.getByText("pkgsilo")).toBeVisible();
    await expect(canvas.queryByRole("link")).not.toBeInTheDocument();
  },
});

// An identity that the actor can't read, without a link.
export const Unknown = meta.story({
  args: { id: "00000000-0000-4000-8000-000000000099" },
  play: async ({ canvas }) => {
    await expect(await canvas.findByText(m.identityUnknown())).toBeVisible();
    await expect(canvas.queryByRole("link")).not.toBeInTheDocument();
  },
});

// The unknown identity in German.
export const German = meta.story({
  args: { id: "00000000-0000-4000-8000-000000000099" },
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(await canvas.findByText(m.identityUnknown({}, { locale: "de" }))).toBeVisible();
  },
});
