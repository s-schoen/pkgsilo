import { DbClient } from "@tanstack/react-db";
import { QueryClient } from "@tanstack/react-query";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { m } from "@/paraglide/messages.js";
import { renderRoute } from "@/test/render.tsx";

describe("router", () => {
  it("shows the start page at /", async () => {
    renderRoute("/");

    expect(await screen.findByText(m.welcome())).toBeInTheDocument();
  });

  it("links to the start page from the product name in the header of every page", async () => {
    renderRoute("/does-not-exist");

    expect(await screen.findByRole("link", { name: "pkgsilo" })).toHaveAttribute("href", "/");
  });

  it("marks the navigation's entry of the page that's shown", async () => {
    renderRoute("/");
    await screen.findByText(m.welcome());

    const navigation = screen.getByRole("navigation", { name: m.mainNavigation() });
    expect(within(navigation).getByRole("link", { name: m.home() })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("marks no entry of the navigation for a page that isn't in it", async () => {
    renderRoute("/does-not-exist");
    await screen.findByText(m.notFound());

    const navigation = screen.getByRole("navigation", { name: m.mainNavigation() });
    expect(within(navigation).getByRole("link", { name: m.home() })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("shows that a page doesn't exist for a path that no route takes", async () => {
    renderRoute("/does-not-exist");

    expect(await screen.findByText(m.notFound())).toBeInTheDocument();
  });

  it("goes to the start page from the page that doesn't exist", async () => {
    const user = userEvent.setup();
    const { router } = renderRoute("/does-not-exist");

    await user.click(await screen.findByRole("link", { name: m.goHome() }));

    expect(await screen.findByText(m.welcome())).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("gives the routes the clients of the API and of TanStack DB", async () => {
    const { router } = renderRoute("/");
    await screen.findByText(m.welcome());

    expect(router.options.context.queryClient).toBeInstanceOf(QueryClient);
    expect(router.options.context.dbClient).toBeInstanceOf(DbClient);
  });
});
