// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";
import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { Link } from "@/components/ui/link";
import { PreviewCard, PreviewTrigger } from "@/components/ui/preview-card";

import type { ComponentProps } from "react";

const text = {
  owner: "Owner",
  username: "ada",
  displayName: "Ada Lovelace",
  profile: "View profile",
};

// A route of the app, as a link's href takes it.
type Href = ComponentProps<typeof Link>["href"];

// No route shows an identity yet: the cast stands in for a route such as /identities/$username.
const profileHref = "/identities/ada" as Href;

const navigate = fn();

const meta = preview.meta({
  component: PreviewCard,
  args: {
    heading: text.owner,
    children: (
      <>
        <strong>{text.displayName}</strong>
        <Link href={profileHref} variant="secondary">
          {text.profile}
        </Link>
      </>
    ),
  },
  argTypes: { children: { control: false } },
  render: (args) => (
    <PreviewTrigger>
      <Link href={profileHref}>{text.username}</Link>
      <PreviewCard {...args} />
    </PreviewTrigger>
  ),
  decorators: [
    // A click on a link navigates through this spy, not the story's frame.
    (Story) => (
      <RouterProvider navigate={navigate}>
        <Story />
      </RouterProvider>
    ),
  ],
  beforeEach: () => {
    navigate.mockClear();
  },
});

// The pointer opens the card after a rest on its trigger, and closes it once it leaves both.
export const OpenOnHover = meta.story({
  play: async ({ canvas, canvasElement, userEvent }) => {
    const trigger = canvas.getByRole("link", { name: text.username });
    // React Aria opens a preview on hover only while the pointer is the user's last input, and the
    // earlier stories of the page used the keyboard.
    await userEvent.click(canvasElement);

    await userEvent.hover(trigger);

    const card = await screen.findByRole("dialog", { name: text.owner }, { timeout: 2000 });
    await expect(card).toHaveTextContent(text.displayName);
    await expect(trigger).toHaveAttribute("aria-expanded", "true");

    // React Aria keeps the card open while the pointer is between the trigger and the card: it
    // closes once the pointer moves away from both.
    await userEvent.unhover(trigger);
    await userEvent.pointer({
      target: document.body,
      coords: { clientX: window.innerWidth - 1, clientY: window.innerHeight - 1 },
    });

    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  },
});

// The keyboard's focus opens it after the same delay; Tab moves into the card, to its link, and
// Escape closes it.
export const OpenOnFocus = meta.story({
  play: async ({ canvas, userEvent }) => {
    const trigger = canvas.getByRole("link", { name: text.username });

    await userEvent.tab();
    await expect(trigger).toHaveFocus();

    const card = await screen.findByRole("dialog", { name: text.owner }, { timeout: 2000 });

    await userEvent.tab();
    await expect(screen.getByRole("link", { name: text.profile })).toHaveFocus();
    // It enters with a transition, from opacity 0.
    await waitFor(async () => {
      await expect(card).toBeVisible();
    });

    await userEvent.keyboard("{Escape}");

    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  },
});

// A card that only names its trigger, without more content.
export const HeadingOnly = meta.story({
  args: { children: undefined },
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();
    await expect(canvas.getByRole("link", { name: text.username })).toHaveFocus();

    await expect(
      await screen.findByRole("dialog", { name: text.owner }, { timeout: 2000 }),
    ).toHaveTextContent(text.owner);
  },
});

// A click on the trigger follows its link, as without a card.
export const TriggerStillNavigates = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("link", { name: text.username }));

    await expect(navigate).toHaveBeenCalledWith(profileHref, undefined);
  },
});
