import IconHome from "@tabler/icons-react/icons/IconHome";
import IconUsers from "@tabler/icons-react/icons/IconUsers";
// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";
import { expect, fn, screen, spyOn, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { AppLayout } from "@/components/layouts/app-layout.tsx";
import { NavItem, NavList } from "@/components/ui/nav-list";
import { RoutePreloadContext } from "@/hooks/use-route-preload.ts";
import { m } from "@/paraglide/messages.js";

// The texts of the layout, where the app passes messages and its pages.
const text = {
  navigation: "Main",
  home: "Home",
  groups: "Groups",
  page: "The content of the page.",
  action: "Action",
};

// The router that React Aria's links navigate through, and its preloadRoute, as root-layout.tsx
// provides them.
const navigate = fn();
const preload = fn();

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
    preload.mockClear();
  },
  decorators: [
    (Story) => (
      <RouterProvider navigate={navigate}>
        <RoutePreloadContext value={preload}>
          <Story />
        </RoutePreloadContext>
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

// The product name preloads the start page once the keyboard focuses it, as it does once the
// pointer rests on it (src/hooks/use-route-preload.test.tsx). The story doesn't hover: its hover is
// synthetic, and Chromium's real pointer, which rests elsewhere, would end it.
export const BrandPreloaded = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();

    await expect(canvas.getByRole("link", { name: "pkgsilo" })).toHaveFocus();
    await expect(preload).toHaveBeenCalledOnce();
    await expect(preload).toHaveBeenCalledWith("/");
  },
});

// The media query of the side menu's width, which a story turns wide, as a tablet that turns.
class SideMenuQuery extends EventTarget {
  readonly media = "(width >= 48rem)";
  matches = false;

  turnWide() {
    this.matches = true;
    this.dispatchEvent(new Event("change"));
  }
}

let sideMenuQuery = new SideMenuQuery();

// The drawer is open when the screen turns wide enough for the side menu: it closes, since its
// overlay would cover the side menu, which shows the same navigation.
export const DrawerClosedOnWideScreen = meta.story({
  globals: { viewport: { value: "mobile1", isRotated: false } },
  beforeEach: () => {
    sideMenuQuery = new SideMenuQuery();
    const matchMedia = window.matchMedia.bind(window);
    const spy = spyOn(window, "matchMedia").mockImplementation((query) =>
      query === sideMenuQuery.media
        ? (sideMenuQuery as unknown as MediaQueryList)
        : matchMedia(query),
    );
    return () => {
      spy.mockRestore();
    };
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: m.openNavigation() }));
    await screen.findByRole("dialog", { name: m.navigation() });
    // The browser paints the open drawer before the screen turns, as it does for a user: a drawer
    // that closes before its first frame is stuck in its exit.
    await new Promise((resolve) => {
      requestAnimationFrame(() => {
        requestAnimationFrame(resolve);
      });
    });

    sideMenuQuery.turnWide();

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
