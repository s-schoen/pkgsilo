import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { m } from "@/paraglide/messages.js";
import { renderRoute } from "@/test/render.tsx";

describe("router", () => {
  it("shows the start page at /", async () => {
    renderRoute("/");

    expect(await screen.findByText(m.welcome())).toBeInTheDocument();
  });

  it("shows the product name in the header of every page", async () => {
    renderRoute("/does-not-exist");

    expect(await screen.findByRole("heading", { name: "pkgsilo" })).toBeInTheDocument();
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
});
