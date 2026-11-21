import IconHome from "@tabler/icons-react/icons/IconHome";
import IconKey from "@tabler/icons-react/icons/IconKey";
import IconUsers from "@tabler/icons-react/icons/IconUsers";
// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";
import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { NavItem, NavList } from "@/components/ui/nav-list";

// The texts of the navigation, where the app passes messages. The stories have no router, so every
// item links to the one route there is, and is an <a>.
const text = {
  navigation: "Main",
  home: "Home",
  groups: "Groups",
  apiKeys: "API keys",
  long: "Identities, groups and the policies that bind them",
};

const meta = preview.meta({
  component: NavList,
  args: { "aria-label": text.navigation, children: undefined },
  render: (args) => (
    <div style={{ inlineSize: "var(--size-sidebar)" }}>
      <NavList {...args}>
        <NavItem href="/" icon={IconHome} isCurrent>
          {text.home}
        </NavItem>
        <NavItem href="/" icon={IconUsers}>
          {text.groups}
        </NavItem>
        <NavItem href="/" icon={IconKey}>
          {text.apiKeys}
        </NavItem>
      </NavList>
    </div>
  ),
});

// A navigation landmark of links, in which the current page's item is marked.
export const Default = meta.story({
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("navigation", { name: args["aria-label"] })).toBeVisible();
    await expect(canvas.getByRole("link", { name: text.home })).toHaveAttribute(
      "aria-current",
      "page",
    );
    await expect(canvas.getByRole("link", { name: text.groups })).not.toHaveAttribute(
      "aria-current",
    );
  },
});

// Tab moves through the items in order, and the focused one shows the ring.
export const FocusedWithKeyboard = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();
    await expect(canvas.getByRole("link", { name: text.home })).toHaveFocus();

    await userEvent.tab();
    const groups = canvas.getByRole("link", { name: text.groups });
    await expect(groups).toHaveFocus();
    await expect(groups).toHaveAttribute("data-focus-visible");
  },
});

// A name longer than the menu is wide ends in an ellipsis, on one line.
export const LongName = meta.story({
  render: (args) => (
    <div style={{ inlineSize: "var(--size-sidebar)" }}>
      <NavList {...args}>
        <NavItem href="/" icon={IconUsers}>
          {text.long}
        </NavItem>
      </NavList>
    </div>
  ),
  play: async ({ canvas }) => {
    const link = canvas.getByRole("link", { name: text.long });
    // One row of the control's height, however long the name.
    await expect(link.getBoundingClientRect().height).toBeLessThan(40);
  },
});

// The router that React Aria's links navigate through, as root-layout.tsx provides it.
const navigate = fn();

// Following an item navigates through the router, and calls the NavList's onNavigate, with which a
// drawer closes.
export const Followed = meta.story({
  args: { onNavigate: fn() },
  beforeEach: () => {
    navigate.mockClear();
  },
  decorators: [
    (Story) => (
      <RouterProvider navigate={navigate}>
        <Story />
      </RouterProvider>
    ),
  ],
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("link", { name: text.groups }));

    await expect(navigate).toHaveBeenCalledOnce();
    await expect(args.onNavigate).toHaveBeenCalledOnce();
  },
});
