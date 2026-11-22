import { screen, waitFor, within } from "@testing-library/react";
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

// The signed-in identity, which pkgsilo created; grace, whom it created, without a display name;
// and an identity whose name and username the actor can't read, by a creator it can't read either.
const identities: Array<IdentityResponse> = [
  {
    ...testIdentity,
    createdBy: PKGSILO_ID,
    updatedBy: PKGSILO_ID,
    createdAt: new Date(Date.now() - 48 * hour).toISOString(),
    updatedAt: new Date(Date.now() - 3 * hour).toISOString(),
  },
  {
    id: "00000000-0000-4000-8000-000000000003",
    username: "grace",
    createdBy: testIdentity.id,
    createdAt: new Date(Date.now() - 24 * hour).toISOString(),
    updatedAt: new Date(Date.now() - 24 * hour).toISOString(),
  },
  {
    id: "00000000-0000-4000-8000-000000000004",
    createdBy: "00000000-0000-4000-8000-000000000099",
  },
];
const [ada, grace, hidden] = identities as [IdentityResponse, IdentityResponse, IdentityResponse];

function listIdentities() {
  return getListIdentitiesMockHandler({
    data: identities,
    pagination: { offset: 0, limit: 100, nextOffset: null },
  });
}

async function findRows() {
  const list = await screen.findByRole("grid", { name: m.identities() });
  return within(list).getAllByRole("row");
}

describe("/identities", () => {
  it("lists the identities by name, with the fallbacks for the fields it can't read", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute("/identities");

    expect(
      await screen.findByRole("heading", { level: 1, name: m.identities() }),
    ).toBeInTheDocument();
    // Ordered by the shown name: the id's digits before the letters.
    expect((await findRows()).map((row) => row.getAttribute("data-row-id"))).toEqual([
      hidden.id,
      ada.id,
      grace.id,
    ]);
    expect(screen.getByText(hidden.id)).toBeInTheDocument();
    expect(screen.getByText("grace")).toBeInTheDocument();
  });

  it("shows the properties as chips, and the badge on the signed-in identity", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute("/identities");

    const [hiddenRow, adaRow, graceRow] = (await findRows()) as [
      HTMLElement,
      HTMLElement,
      HTMLElement,
    ];
    expect(within(adaRow).getByText(m.identityYou())).toBeInTheDocument();
    expect(within(graceRow).queryByText(m.identityYou())).not.toBeInTheDocument();

    expect(within(adaRow).getByText(`@${testIdentity.username}`)).toBeInTheDocument();
    expect(within(adaRow).getByText("pkgsilo")).toBeInTheDocument();
    expect(within(adaRow).getByText("3 hours ago")).toBeInTheDocument();
    expect(within(graceRow).getByText(testIdentity.displayName)).toBeInTheDocument();
    expect(within(graceRow).getByText("yesterday")).toBeInTheDocument();

    // No chip for a field it can't read; a creator it can't read is unknown.
    expect(within(hiddenRow).queryByText(`${m.username()}:`)).not.toBeInTheDocument();
    expect(within(hiddenRow).queryByText(`${m.identityUpdatedAt()}:`)).not.toBeInTheDocument();
    expect(within(hiddenRow).getByText(m.identityUnknown())).toBeInTheDocument();
  });

  it("links a row's title to its identity's page", async () => {
    server.use(...signedIn(), listIdentities());
    const user = userEvent.setup();
    const { router } = renderRoute("/identities");

    const [, , graceRow] = (await findRows()) as [HTMLElement, HTMLElement, HTMLElement];
    await user.click(within(graceRow).getByRole("link", { name: "grace" }));

    expect(await screen.findByRole("heading", { level: 1, name: "grace" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(`/identities/${grace.id}`);
  });

  it("links a creator's chip to the creator, but not pkgsilo's or an unknown one", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute("/identities");

    const [hiddenRow, adaRow, graceRow] = (await findRows()) as [
      HTMLElement,
      HTMLElement,
      HTMLElement,
    ];
    const creator = (row: HTMLElement) =>
      within(row).getByText((_, element) =>
        Boolean(
          element?.matches("a, button") &&
          element.textContent.startsWith(`${m.identityCreatedBy()}:`),
        ),
      );
    expect(creator(graceRow)).toHaveAttribute("href", `/identities/${ada.id}`);
    expect(creator(adaRow)).not.toHaveAttribute("href");
    expect(creator(hiddenRow)).not.toHaveAttribute("href");
  });

  it("marks the menu's Identities as the current page", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute("/identities");

    const section = await screen.findByRole("list", { name: m.administration() });
    expect(within(section).getByRole("link", { name: m.identities() })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("filters by creator and orders by the URL", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute(`/identities?createdBy=${testIdentity.id},${PKGSILO_ID}&sort=-username`);

    await waitFor(async () => {
      expect((await findRows()).map((row) => row.getAttribute("data-row-id"))).toEqual([
        grace.id,
        ada.id,
      ]);
    });
  });

  it("keeps the search in the URL, leaving the default order out", async () => {
    server.use(...signedIn(), listIdentities());
    const user = userEvent.setup();
    const { router } = renderRoute("/identities");

    await user.type(await screen.findByRole("searchbox", { name: m.dataTableSearch() }), "grace");

    await waitFor(() => {
      expect(router.state.location.search).toEqual({ q: "grace" });
    });
    expect((await findRows()).map((row) => row.getAttribute("data-row-id"))).toEqual([grace.id]);
  });

  it("groups by creator from the URL", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute("/identities?group=createdBy");

    const tree = await screen.findByRole("treegrid", { name: m.identities() });
    // A group is a row of the tree without a row id of its own; the groups are ordered by label.
    const groupRows = () =>
      within(tree)
        .getAllByRole("row")
        .filter((row) => !row.hasAttribute("data-row-id"));
    await waitFor(() => {
      expect(groupRows()).toHaveLength(3);
    });
    const [adaGroup, pkgsiloGroup, unknownGroup] = groupRows();
    expect(adaGroup).toHaveTextContent(testIdentity.displayName);
    expect(pkgsiloGroup).toHaveTextContent("pkgsilo");
    expect(unknownGroup).toHaveTextContent(m.identityUnknown());
  });
});
