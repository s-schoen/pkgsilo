import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  useLocation,
} from "@tanstack/react-router";
import { useState } from "react";
import { expect } from "storybook/test";
import { z } from "zod/v4";

import preview from "#.storybook/preview";
import { getCreateSessionMockHandler } from "@/api/gen/client/sessions/sessions.msw.ts";
import { SignInPage } from "@/features/session/components/sign-in-page.tsx";
import { m } from "@/paraglide/messages.js";

import type { Decorator } from "@storybook/react-vite";
import type { ComponentType } from "react";

// The credentials that the stories type, and the page where a sign-in leads, which shows its URL.
const text = { username: "ada", password: "correct horse", landed: "Signed in, at" };

// Where a sign-in leads: any page but the sign-in page, which shows its own URL.
function Landed() {
  const { href } = useLocation();
  const landedAt = `${text.landed} ${href}`;
  return <p>{landedAt}</p>;
}

// A router over a memory history at the story's URL (parameters.url), with the sign-in page at
// /signin, as routes/signin.tsx has it, without its check of the session, and every other path
// showing Landed.
function StoryRouter({ story: Story, url }: { story: ComponentType; url: string }) {
  const [router] = useState(() => {
    const rootRoute = createRootRoute({ component: Outlet });
    const signInRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "signin",
      validateSearch: z.object({ redirect: z.string().optional().catch(undefined) }),
      component: function SignInStory() {
        return <Story />;
      },
    });
    const landedRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: "$",
      component: Landed,
    });
    return createRouter({
      routeTree: rootRoute.addChildren([signInRoute, landedRoute]),
      history: createMemoryHistory({ initialEntries: [url] }),
    });
  });
  return <RouterProvider router={router} />;
}

const withRouter: Decorator = (Story, { parameters }) => (
  <StoryRouter
    story={Story}
    url={typeof parameters["url"] === "string" ? parameters["url"] : "/signin"}
  />
);

const meta = preview.meta({
  component: SignInPage,
  parameters: { layout: "fullscreen" },
  decorators: [withRouter],
  beforeEach: ({ msw }) => {
    msw.use(getCreateSessionMockHandler({ data: { id: "00000000-0000-4000-8000-000000000001" } }));
  },
});

// The card on the canvas, with the product's name, the title and the form.
export const Default = meta.story({
  play: async ({ canvas }) => {
    await expect(
      await canvas.findByRole("heading", { level: 1, name: m.signInTitle() }),
    ).toBeVisible();
    await expect(canvas.getByText("pkgsilo")).toBeVisible();
    await expect(canvas.getByLabelText(m.username())).toBeVisible();
    await expect(canvas.getByLabelText(m.password())).toBeVisible();
    await expect(canvas.getByRole("button", { name: m.signIn() })).toBeVisible();
  },
});

// A sign-in leads to the page of the redirect, where the user was sent from.
export const LeadsToRedirect = meta.story({
  parameters: { url: "/signin?redirect=%2Fgroups%2F1%3Ftab%3Dmembers" },
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(await canvas.findByLabelText(m.username()), text.username);
    await userEvent.type(canvas.getByLabelText(m.password()), `${text.password}{Enter}`);

    await expect(
      await canvas.findByText(`${text.landed} /groups/1?tab=members`),
    ).toBeInTheDocument();
  },
});

// A redirect to another host leads to the start page instead, so that the page is no open redirect.
export const IgnoresRedirectToAnotherHost = meta.story({
  parameters: { url: "/signin?redirect=%2F%2Fevil.example%2F" },
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(await canvas.findByLabelText(m.username()), text.username);
    await userEvent.type(canvas.getByLabelText(m.password()), `${text.password}{Enter}`);

    await expect(await canvas.findByText(`${text.landed} /`)).toBeInTheDocument();
  },
});

// The page's texts in German.
export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(
      await canvas.findByRole("heading", {
        level: 1,
        name: m.signInTitle({}, { locale: "de" }),
      }),
    ).toBeVisible();
    await expect(
      canvas.getByRole("button", { name: m.signIn({}, { locale: "de" }) }),
    ).toBeVisible();
  },
});
