import IconDots from "@tabler/icons-react/icons/IconDots";
import IconPlus from "@tabler/icons-react/icons/IconPlus";
// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";
import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { PageHeader } from "@/components/layouts/page-header.tsx";
import { Button } from "@/components/ui/button";
import { RoutePreloadContext } from "@/hooks/use-route-preload.ts";
import { m } from "@/paraglide/messages.js";

// The texts of the header, where a page passes messages and the API's names.
const text = {
  identities: "Identities",
  identity: "Ada Lovelace",
  newIdentity: "New identity",
  actions: "Actions",
};

// The router that React Aria's links navigate through, and its preloadRoute, as root-layout.tsx
// provides them.
const navigate = fn();
const preload = fn();

const meta = preview.meta({
  component: PageHeader,
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

const newIdentity = {
  id: "new",
  element: (
    <Button variant="primary" icon={IconPlus}>
      {text.newIdentity}
    </Button>
  ),
};

// The header of an identity's details page.
const detailsArgs = {
  breadcrumbs: [{ label: text.identities, href: "/" as const }, { label: text.identity }],
  actions: [
    { id: "menu", element: <Button variant="ghost" icon={IconDots} aria-label={text.actions} /> },
  ],
};

// A list page's header: its title, the page's <h1>, and its primary action at the end.
export const ListPage = meta.story({
  args: { title: text.identities, actions: [newIdentity] },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("heading", { level: 1, name: text.identities })).toBeVisible();
    await expect(canvas.getByRole("button", { name: text.newIdentity })).toBeVisible();
    await expect(canvas.queryByRole("navigation")).not.toBeInTheDocument();
  },
});

// A details page's header: the breadcrumbs to the page, without a title, which its main column
// holds, and a menu of actions. The last crumb is the page that's shown, which links nowhere.
export const DetailsPage = meta.story({
  args: detailsArgs,
  play: async ({ canvas }) => {
    const breadcrumbs = canvas.getByRole("navigation", { name: m.breadcrumbs() });
    await expect(breadcrumbs).toBeVisible();
    await expect(canvas.getByRole("link", { name: text.identities })).toHaveAttribute("href", "/");
    await expect(canvas.getByText(text.identity)).toHaveAttribute("aria-current", "page");
    await expect(canvas.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
  },
});

// A crumb's link navigates through the router.
export const CrumbFollowed = meta.story({
  args: detailsArgs,
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("link", { name: text.identities }));
    await expect(navigate).toHaveBeenCalledOnce();
  },
});

// A crumb's link preloads its route once the keyboard focuses it, as once the pointer rests on it.
export const CrumbPreloaded = meta.story({
  args: detailsArgs,
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();
    await expect(canvas.getByRole("link", { name: text.identities })).toHaveFocus();
    await expect(preload).toHaveBeenCalledWith("/");
  },
});

// An action that isn't visible, as one the actor may not take, isn't rendered; the others are.
export const HiddenAction = meta.story({
  args: {
    title: text.identities,
    actions: [{ ...newIdentity, isVisible: false }, ...detailsArgs.actions],
  },
  play: async ({ canvas }) => {
    await expect(canvas.queryByRole("button", { name: text.newIdentity })).not.toBeInTheDocument();
    await expect(canvas.getByRole("button", { name: text.actions })).toBeVisible();
  },
});

// Without a visible action, the header is its title alone.
export const NoVisibleAction = meta.story({
  args: { title: text.identities, actions: [{ ...newIdentity, isVisible: false }] },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("heading", { level: 1, name: text.identities })).toBeVisible();
    await expect(canvas.queryByRole("button")).not.toBeInTheDocument();
  },
});

// A name longer than the page is wide wraps, and the actions move below it.
export const LongTitle = meta.story({
  args: {
    title: `${text.identities} ${"·".repeat(20)} ${text.identity.repeat(6)}`,
    actions: [newIdentity],
  },
  decorators: [
    (Story) => (
      <div style={{ inlineSize: "20rem" }}>
        <Story />
      </div>
    ),
  ],
  play: async ({ canvas }) => {
    const heading = canvas.getByRole("heading", { level: 1 });
    await expect(heading.scrollWidth).toBeLessThanOrEqual(heading.clientWidth);
  },
});

// The breadcrumbs' name in German.
export const German = meta.story({
  args: detailsArgs,
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(
      canvas.getByRole("navigation", { name: m.breadcrumbs({}, { locale: "de" }) }),
    ).toBeVisible();
  },
});
