import { HttpResponse, http } from "msw";
import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { getGetIdentityMockHandler } from "@/api/gen/client/identities/identities.msw.ts";
import {
  getDeleteCurrentSessionMockHandler,
  getGetCurrentSessionMockHandler,
} from "@/api/gen/client/sessions/sessions.msw.ts";
import { Toaster, toastQueue } from "@/components/ui/toast";
import { AccountMenu } from "@/features/session/components/account-menu.tsx";
import { m } from "@/paraglide/messages.js";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

const identityId = "00000000-0000-4000-8000-000000000002";
const ada: IdentityResponse = { id: identityId, username: "ada", displayName: "Ada Lovelace" };

// The session of the identity, which ends in a day.
const sessionHandler = getGetCurrentSessionMockHandler({
  data: {
    id: "00000000-0000-4000-8000-000000000001",
    identityId,
    createdAt: "2026-11-21T12:00:00Z",
    expiresAt: "2026-11-22T12:00:00Z",
  },
});

// The API's answer with status.
function problem(status: number, title: string) {
  return HttpResponse.json(
    { type: "about:blank", title, status },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

const meta = preview.meta({
  component: AccountMenu,
  args: { onSignedOut: fn() },
  // At the bottom of the side menu's width, with the Toaster of the root route.
  decorators: [
    (Story) => (
      <div style={{ inlineSize: "15rem" }}>
        <Story />
        <Toaster />
      </div>
    ),
  ],
  // The queue is the app's, which outlives a story.
  beforeEach: ({ msw }) => {
    toastQueue.clear();
    msw.use(sessionHandler, getGetIdentityMockHandler({ data: ada }));
  },
});

// The identity's initials and display name; the menu names its username and signs out.
export const SignedIn = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(getDeleteCurrentSessionMockHandler());
  },
  play: async ({ args, canvas, userEvent }) => {
    const trigger = await canvas.findByRole("button", { name: ada.displayName });
    await expect(trigger).toHaveTextContent("AL");

    await userEvent.click(trigger);
    const menu = await screen.findByRole("menu");
    await expect(menu).toHaveTextContent(m.signedInAs({ username: ada.username ?? "" }));

    await userEvent.click(screen.getByRole("menuitem", { name: m.signOut() }));

    await waitFor(async () => {
      await expect(args.onSignedOut).toHaveBeenCalledOnce();
    });
  },
});

// The keyboard opens the menu and signs out as well.
export const Keyboard = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(getDeleteCurrentSessionMockHandler());
  },
  play: async ({ args, canvas, userEvent }) => {
    await canvas.findByRole("button", { name: ada.displayName });
    await userEvent.tab();
    await userEvent.keyboard("{Enter}");

    await waitFor(async () => {
      await expect(screen.getByRole("menuitem", { name: m.signOut() })).toHaveFocus();
    });
    await userEvent.keyboard("{Enter}");

    await waitFor(async () => {
      await expect(args.onSignedOut).toHaveBeenCalledOnce();
    });
  },
});

// An identity without a display name shows its username.
export const UsernameOnly = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(getGetIdentityMockHandler({ data: { id: identityId, username: "ci-bot" } }));
  },
  play: async ({ canvas }) => {
    await expect(await canvas.findByRole("button", { name: "ci-bot" })).toHaveTextContent("CB");
  },
});

// Policies hide the identity from itself: the row says that someone is signed in, and the menu
// still signs out.
export const IdentityHidden = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(http.get("/api/v1/identities/:id", () => problem(404, "Identity not found")));
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole("button", { name: m.signedIn() }));

    const signOut = await screen.findByRole("menuitem", { name: m.signOut() });
    await waitFor(async () => {
      await expect(signOut).toBeVisible();
    });
    await expect(screen.getByRole("menu")).not.toHaveTextContent(
      m.signedInAs({ username: ada.username ?? "" }),
    );
  },
});

// The session had ended already (401): the sign-out leads on all the same.
export const SessionAlreadyEnded = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(http.delete("/api/v1/sessions/current", () => problem(401, "Unauthorized")));
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole("button", { name: ada.displayName }));
    await userEvent.click(await screen.findByRole("menuitem", { name: m.signOut() }));

    await waitFor(async () => {
      await expect(args.onSignedOut).toHaveBeenCalledOnce();
    });
  },
});

// The server failed to end the session: the user stays signed in, and a toast says so.
export const SignOutFailed = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(http.delete("/api/v1/sessions/current", () => problem(500, "Internal Server Error")));
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole("button", { name: ada.displayName }));
    await userEvent.click(await screen.findByRole("menuitem", { name: m.signOut() }));

    await expect(await screen.findByRole("alertdialog", { name: m.signOutFailed() })).toBeVisible();
    await expect(args.onSignedOut).not.toHaveBeenCalled();
  },
});

// The row's and the menu's texts in German.
export const German = meta.story({
  globals: { locale: "de" },
  beforeEach: ({ msw }) => {
    msw.use(getDeleteCurrentSessionMockHandler());
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole("button", { name: ada.displayName }));

    const signOut = await screen.findByRole("menuitem", {
      name: m.signOut({}, { locale: "de" }),
    });
    await waitFor(async () => {
      await expect(signOut).toBeVisible();
    });
  },
});
