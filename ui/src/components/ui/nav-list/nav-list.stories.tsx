import IconHome from "@tabler/icons-react/icons/IconHome";
import IconKey from "@tabler/icons-react/icons/IconKey";
import IconUsers from "@tabler/icons-react/icons/IconUsers";
// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";
import { expect, fn, within } from "storybook/test";

import preview from "#.storybook/preview";
import { NavItem, NavList, NavSection } from "@/components/ui/nav-list";
import { RoutePreloadContext } from "@/hooks/use-route-preload.ts";
import { m } from "@/paraglide/messages.js";

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

// A click with a modifier key leaves the item to the browser, which opens it in a new tab: the
// router doesn't navigate, and the NavList doesn't call onNavigate, so a drawer stays open.
export const OpenedInNewTab = meta.story({
  args: { onNavigate: fn() },
  beforeEach: () => {
    navigate.mockClear();
    // React Aria opens the tab with a click of its own, whose default action the story cancels
    // before any listener of the link runs, since React Aria stops the click's propagation.
    function keepInFrame(event: MouseEvent) {
      event.preventDefault();
    }
    window.addEventListener("click", keepInFrame, { capture: true });
    return () => {
      window.removeEventListener("click", keepInFrame, { capture: true });
    };
  },
  decorators: [
    (Story) => (
      <RouterProvider navigate={navigate}>
        <Story />
      </RouterProvider>
    ),
  ],
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.keyboard("{Control>}");
    await userEvent.click(canvas.getByRole("link", { name: text.groups }));
    await userEvent.keyboard("{/Control}");

    await expect(navigate).not.toHaveBeenCalled();
    await expect(args.onNavigate).not.toHaveBeenCalled();
  },
});

// The router's preloadRoute, as root-layout.tsx provides it.
const preload = fn();

// An item preloads its route once the keyboard focuses it, as it does once the pointer rests on it
// (src/hooks/use-route-preload.test.tsx). The story doesn't hover: its hover is synthetic, and
// Chromium's real pointer, which rests elsewhere, would end it.
export const Preloaded = meta.story({
  beforeEach: () => {
    preload.mockClear();
  },
  decorators: [
    (Story) => (
      <RoutePreloadContext value={preload}>
        <Story />
      </RoutePreloadContext>
    ),
  ],
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();

    await expect(canvas.getByRole("link", { name: text.home })).toHaveFocus();
    await expect(preload).toHaveBeenCalledOnce();
    await expect(preload).toHaveBeenCalledWith("/");
  },
});

// The app's menu, with its items grouped in a section under a heading, in the messages of the
// toolbar's locale. The current page's item is in the section.
function SectionedNavigation(args: { "aria-label": string }) {
  return (
    <div style={{ inlineSize: "var(--size-sidebar)" }}>
      <NavList {...args}>
        <NavItem href="/" icon={IconHome}>
          {m.home()}
        </NavItem>
        <NavSection title={m.administration()}>
          <NavItem href="/" icon={IconUsers} isCurrent>
            {m.identities()}
          </NavItem>
          <NavItem href="/" icon={IconKey}>
            {text.apiKeys}
          </NavItem>
        </NavSection>
      </NavList>
    </div>
  );
}

// A section's heading names the list of its items, and its current item is marked as any other.
export const WithSections = meta.story({
  render: (args) => <SectionedNavigation {...args} />,
  play: async ({ canvas }) => {
    const section = canvas.getByRole("list", { name: m.administration() });
    await expect(canvas.getByRole("heading", { name: m.administration() })).toBeVisible();
    await expect(within(section).getByRole("link", { name: m.identities() })).toHaveAttribute(
      "aria-current",
      "page",
    );
    await expect(canvas.getByRole("link", { name: m.home() })).not.toHaveAttribute("aria-current");
  },
});

// Tab moves from the items above a section into it, past its heading.
export const SectionFocusedWithKeyboard = meta.story({
  render: (args) => <SectionedNavigation {...args} />,
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();
    await userEvent.tab();
    await expect(canvas.getByRole("link", { name: m.identities() })).toHaveFocus();
  },
});

// The section in German.
export const WithSectionsGerman = meta.story({
  globals: { locale: "de" },
  render: (args) => <SectionedNavigation {...args} />,
  play: async ({ canvas }) => {
    await expect(
      canvas.getByRole("list", { name: m.administration({}, { locale: "de" }) }),
    ).toBeVisible();
    await expect(
      canvas.getByRole("link", { name: m.identities({}, { locale: "de" }) }),
    ).toBeVisible();
  },
});
