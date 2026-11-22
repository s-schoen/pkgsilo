import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { getListIdentitiesMockHandler } from "@/api/gen/client/identities/identities.msw.ts";
import { PKGSILO_ID } from "@/features/identities/utils/identity-name.ts";
import { m } from "@/paraglide/messages.js";
import { renderRoute } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { signedIn, testIdentity } from "@/test/session.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

const hour = 60 * 60 * 1000;

// The signed-in identity, which pkgsilo created and grace changed last; grace, without a display
// name, whom ada created; and an identity of which the actor reads only the id and a creator that
// it can't read.
const ada: IdentityResponse = {
  ...testIdentity,
  createdBy: PKGSILO_ID,
  createdAt: new Date(Date.now() - 48 * hour).toISOString(),
  updatedBy: "00000000-0000-4000-8000-000000000003",
  updatedAt: new Date(Date.now() - 3 * hour).toISOString(),
};
const grace: IdentityResponse = {
  id: "00000000-0000-4000-8000-000000000003",
  username: "grace",
  createdBy: testIdentity.id,
};
const hidden: IdentityResponse = {
  id: "00000000-0000-4000-8000-000000000004",
  createdBy: "00000000-0000-4000-8000-000000000099",
};

function listIdentities() {
  return getListIdentitiesMockHandler({
    data: [ada, grace, hidden],
    pagination: { offset: 0, limit: 100, nextOffset: null },
  });
}

// The value of the sidebar's row with this label.
function property(label: string) {
  const term = screen.getAllByRole("term").find((element) => element.textContent === label);
  const row = term?.parentElement;
  if (row === undefined || row === null) {
    throw new Error(`no row for ${label}`);
  }
  return within(row).getByRole("definition");
}

describe("/identities/$id", () => {
  it("shows the identity's name as the title, and its properties", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute(`/identities/${ada.id}`);

    expect(
      await screen.findByRole("heading", { level: 1, name: testIdentity.displayName }),
    ).toBeInTheDocument();
    const breadcrumbs = screen.getByRole("navigation", { name: m.breadcrumbs() });
    expect(within(breadcrumbs).getByRole("link", { name: m.identities() })).toHaveAttribute(
      "href",
      "/identities",
    );
    expect(within(breadcrumbs).getByText(testIdentity.displayName)).toHaveAttribute(
      "aria-current",
      "page",
    );

    expect(property(m.username())).toHaveTextContent(testIdentity.username);
    // pkgsilo created it: named, but no link.
    expect(property(m.identityCreatedBy())).toHaveTextContent("pkgsilo");
    expect(within(property(m.identityCreatedBy())).queryByRole("link")).not.toBeInTheDocument();
    expect(within(property(m.identityCreatedAt())).getByRole("img")).toHaveTextContent(
      "2 days ago",
    );
    // grace changed it last: a link to her page, by her username, since she has no display name.
    expect(
      within(property(m.identityUpdatedBy())).getByRole("link", { name: "grace" }),
    ).toHaveAttribute("href", `/identities/${grace.id}`);
    expect(within(property(m.identityUpdatedAt())).getByRole("img")).toHaveTextContent(
      "3 hours ago",
    );
  });

  it("hides the fields it can't read and names an unknown creator", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute(`/identities/${hidden.id}`);

    expect(await screen.findByRole("heading", { level: 1, name: hidden.id })).toBeInTheDocument();
    expect(screen.getAllByRole("term").map((term) => term.textContent)).toEqual([
      m.identityCreatedBy(),
    ]);
    expect(property(m.identityCreatedBy())).toHaveTextContent(m.identityUnknown());
    expect(within(property(m.identityCreatedBy())).queryByRole("link")).not.toBeInTheDocument();
  });

  it("falls back to the username as the title", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute(`/identities/${grace.id}`);

    expect(await screen.findByRole("heading", { level: 1, name: "grace" })).toBeInTheDocument();
  });

  it("follows a reference to another identity's page", async () => {
    server.use(...signedIn(), listIdentities());
    const user = userEvent.setup();
    const { router } = renderRoute(`/identities/${grace.id}`);

    await screen.findByRole("heading", { level: 1, name: "grace" });
    await user.click(
      within(property(m.identityCreatedBy())).getByRole("link", {
        name: testIdentity.displayName,
      }),
    );

    expect(
      await screen.findByRole("heading", { level: 1, name: testIdentity.displayName }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(`/identities/${ada.id}`);
  });

  it("shows the page that doesn't exist for an unknown id", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute("/identities/00000000-0000-4000-8000-000000000042");

    expect(
      await screen.findByRole("heading", { level: 1, name: m.notFound() }),
    ).toBeInTheDocument();
  });
});
