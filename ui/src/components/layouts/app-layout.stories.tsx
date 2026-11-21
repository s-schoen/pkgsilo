import IconHome from "@tabler/icons-react/icons/IconHome";
import IconUsers from "@tabler/icons-react/icons/IconUsers";
// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";
import { expect, fn, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { AppLayout } from "@/components/layouts/app-layout.tsx";
import { NavItem, NavList } from "@/components/ui/nav-list";
import { m } from "@/paraglide/messages.js";

// The texts of the layout, where the app passes messages and its pages.
const text = {
  navigation: "Main",
  home: "Home",
  groups: "Groups",
  page: "The content of the page.",
  action: "Action",
};

// The router that React Aria's links navigate through, as root-layout.tsx provides it.
const navigate = fn();

const meta = preview.meta({
  component: AppLayout,
  args: {
    navigation: ({ onNavigate }) => (
      <NavList aria-label={text.navigation} onNavigate={onNavigate}>
        <NavItem href="/" icon={IconHome} isCurrent>
          {text.home}
        </NavItem>
        <NavItem href="/" icon={IconUsers}>
          {text.groups}
        </NavItem>
      </NavList>
    ),
    children: <p>{text.page}</p>,
  },
  parameters: { layout: "fullscreen" },
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
});

// A wide screen: the header, the navigation in the side menu, and the page on its surface.
export const Desktop = meta.story({
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("link", { name: "pkgsilo" })).toHaveAttribute("href", "/");
    await expect(canvas.getByRole("navigation", { name: text.navigation })).toBeVisible();
    await expect(canvas.getByRole("main")).toHaveTextContent(text.page);
    // The header spans the top; the side menu sits at the start of the page, below it.
    const header = canvas.getByRole("banner").getBoundingClientRect();
    const sidebar = canvas
      .getByRole("navigation", { name: text.navigation })
      .getBoundingClientRect();
    const main = canvas.getByRole("main").getBoundingClientRect();
    await expect(sidebar.top).toBeGreaterThanOrEqual(header.bottom);
    await expect(main.top).toBeGreaterThanOrEqual(header.bottom);
    await expect(sidebar.right).toBeLessThanOrEqual(main.left);
    await expect(header.width).toBeGreaterThan(main.width);
    // The side menu shows the navigation, so the header's menu button is gone.
    await expect(
      canvas.queryByRole("button", { name: m.openNavigation() }),
    ).not.toBeInTheDocument();
  },
});

// The controls at the end of the header.
export const WithActions = meta.story({
  args: {
    actions: <button type="button">{text.action}</button>,
  },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("button", { name: text.action })).toBeVisible();
  },
});

// A small screen: the side menu is gone, and the header's button opens the navigation in a drawer,
// which closes once a link is followed.
export const Mobile = meta.story({
  globals: { viewport: { value: "mobile1", isRotated: false } },
  play: async ({ canvas, userEvent }) => {
    await expect(
      canvas.queryByRole("navigation", { name: text.navigation }),
    ).not.toBeInTheDocument();

    // The page takes the whole width, below the header.
    const header = canvas.getByRole("banner").getBoundingClientRect();
    const main = canvas.getByRole("main").getBoundingClientRect();
    await expect(main.top).toBeGreaterThanOrEqual(header.bottom);
    await expect(main.width).toBeGreaterThan(header.width * 0.9);

    await userEvent.click(canvas.getByRole("button", { name: m.openNavigation() }));

    const drawer = await screen.findByRole("dialog", { name: m.navigation() });
    await expect(drawer).toContainElement(
      screen.getByRole("navigation", { name: text.navigation }),
    );

    await userEvent.click(screen.getByRole("link", { name: text.groups }));

    await expect(navigate).toHaveBeenCalledOnce();
    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  },
});

// The header's texts in German.
export const German = meta.story({
  globals: { locale: "de", viewport: { value: "mobile1", isRotated: false } },
  play: async ({ canvas }) => {
    await expect(
      canvas.getByRole("button", { name: m.openNavigation({}, { locale: "de" }) }),
    ).toBeVisible();
  },
});
