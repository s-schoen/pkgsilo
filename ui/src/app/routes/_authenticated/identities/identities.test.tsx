import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";

import { getListIdentitiesMockHandler } from "@/api/gen/client/identities/identities.msw.ts";
import { IdentityResponseAllowedActionsItem } from "@/api/gen/client/model/identityResponseAllowedActionsItem.ts";
import { toastQueue } from "@/components/ui/toast";
import { PKGSILO_ID } from "@/features/identities/utils/identity-name.ts";
import { m } from "@/paraglide/messages.js";
import { renderRoute } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { permissions, signedIn, testIdentity } from "@/test/session.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

const hour = 60 * 60 * 1000;

// Every action on an identity: the actor may do everything to the identities it can read.
const allowedActions = Object.values(IdentityResponseAllowedActionsItem);

// The signed-in identity, which pkgsilo created; grace, whom it created, without a display name;
// and an identity whose name and username the actor can't read, by a creator it can't read either.
const identities: Array<IdentityResponse> = [
  {
    ...testIdentity,
    createdBy: PKGSILO_ID,
    updatedBy: PKGSILO_ID,
    createdAt: new Date(Date.now() - 48 * hour).toISOString(),
    updatedAt: new Date(Date.now() - 3 * hour).toISOString(),
    allowedActions,
  },
  {
    id: "00000000-0000-4000-8000-000000000003",
    username: "grace",
    createdBy: testIdentity.id,
    createdAt: new Date(Date.now() - 24 * hour).toISOString(),
    updatedAt: new Date(Date.now() - 24 * hour).toISOString(),
    allowedActions,
  },
  {
    id: "00000000-0000-4000-8000-000000000004",
    createdBy: "00000000-0000-4000-8000-000000000099",
    allowedActions,
  },
];
const [ada, grace, hidden] = identities as [IdentityResponse, IdentityResponse, IdentityResponse];

// The identities that the API has: identities at the start of each test, without the ones that
// deleteIdentities deleted, so that a refetch, as after a denied delete, lists what's left.
let stored: Array<IdentityResponse> = [];

function listIdentities() {
  return getListIdentitiesMockHandler(() => ({
    data: stored,
    pagination: { offset: 0, limit: 100, nextOffset: null },
  }));
}

// Answers a delete with 204, or with 403 for the ids in denied, and records the ids it was asked
// for.
function deleteIdentities(requested: Array<string>, denied: Array<string> = []) {
  return http.delete("/api/v1/identities/:id", ({ params }) => {
    const id = String(params.id);
    requested.push(id);
    if (denied.includes(id)) {
      return HttpResponse.json(
        { type: "about:blank", title: "Forbidden", status: 403 },
        { status: 403, headers: { "Content-Type": "application/problem+json" } },
      );
    }
    stored = stored.filter((identity) => identity.id !== id);
    return new HttpResponse(null, { status: 204 });
  });
}

function rowOf(id: string) {
  const row = screen
    .getAllByRole("row")
    .find((candidate) => candidate.getAttribute("data-row-id") === id);
  if (row === undefined) {
    throw new Error(`no row for ${id}`);
  }
  return row;
}

// Selects the rows of these identities, then presses Delete and confirms the dialog, which counts
// the identities it deletes.
async function deleteSelected(
  user: ReturnType<typeof userEvent.setup>,
  ids: Array<string>,
  count: number,
) {
  await findRows();
  for (const id of ids) {
    await user.click(within(rowOf(id)).getByRole("checkbox"));
  }
  await user.click(screen.getByRole("button", { name: m.dataTableDelete() }));
  const dialog = await screen.findByRole("alertdialog", {
    name: m.dataTableDeleteTitle({ count }),
  });
  await user.click(within(dialog).getByRole("button", { name: m.dataTableDelete() }));
}

async function findRows() {
  const list = await screen.findByRole("grid", { name: m.identities() });
  return within(list).getAllByRole("row");
}

describe("/identities", () => {
  beforeEach(() => {
    toastQueue.clear();
    stored = identities;
  });

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
    // grace shows by its username, both as the name and in the chip.
    expect(screen.getAllByText("grace")).toHaveLength(2);
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

    expect(within(adaRow).getByText(testIdentity.username)).toBeInTheDocument();
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

  it("groups the creators it can't know as one unknown creator", async () => {
    // One creator it can't read, another, and a createdBy it can't read.
    const unknownCreators: Array<IdentityResponse> = [
      hidden,
      {
        id: "00000000-0000-4000-8000-000000000005",
        createdBy: "00000000-0000-4000-8000-000000000098",
      },
      { id: "00000000-0000-4000-8000-000000000006" },
    ];
    server.use(
      ...signedIn(),
      getListIdentitiesMockHandler({
        data: [ada, ...unknownCreators],
        pagination: { offset: 0, limit: 100, nextOffset: null },
      }),
    );
    renderRoute("/identities?group=createdBy");

    const tree = await screen.findByRole("treegrid", { name: m.identities() });
    const groupRows = () =>
      within(tree)
        .getAllByRole("row")
        .filter((row) => !row.hasAttribute("data-row-id"));
    await waitFor(() => {
      expect(groupRows()).toHaveLength(2);
    });
    const [pkgsiloGroup, unknownGroup] = groupRows() as [HTMLElement, HTMLElement];
    expect(pkgsiloGroup).toHaveTextContent("pkgsilo");
    expect(unknownGroup).toHaveTextContent(m.identityUnknown());
    // The group's count.
    expect(within(unknownGroup).getByText(String(unknownCreators.length))).toBeInTheDocument();
  });

  it("deletes an identity and says so in a toast", async () => {
    const requested: Array<string> = [];
    server.use(...signedIn(), listIdentities(), deleteIdentities(requested));
    const user = userEvent.setup();
    renderRoute("/identities");

    await deleteSelected(user, [grace.id], 1);

    expect(await screen.findByText(m.identityDeleted({ name: "grace" }))).toBeInTheDocument();
    expect(requested).toEqual([grace.id]);
    expect((await findRows()).map((row) => row.getAttribute("data-row-id"))).toEqual([
      hidden.id,
      ada.id,
    ]);
  });

  it("deletes several identities", async () => {
    const requested: Array<string> = [];
    server.use(...signedIn(), listIdentities(), deleteIdentities(requested));
    const user = userEvent.setup();
    renderRoute("/identities");

    await deleteSelected(user, [grace.id, hidden.id], 2);

    expect(await screen.findByText(m.identitiesDeleted({ count: 2 }))).toBeInTheDocument();
    expect(requested.toSorted()).toEqual([grace.id, hidden.id].toSorted());
    expect((await findRows()).map((row) => row.getAttribute("data-row-id"))).toEqual([ada.id]);
  });

  it("reports a partly failed delete and keeps the identities it couldn't delete", async () => {
    server.use(...signedIn(), listIdentities(), deleteIdentities([], [hidden.id]));
    const user = userEvent.setup();
    renderRoute("/identities");

    await deleteSelected(user, [grace.id, hidden.id], 2);

    expect(
      await screen.findByText(m.identitiesDeletedPartly({ deleted: 1, count: 2 })),
    ).toBeInTheDocument();
    await waitFor(async () => {
      expect((await findRows()).map((row) => row.getAttribute("data-row-id"))).toEqual([
        hidden.id,
        ada.id,
      ]);
    });
  });

  it("reports a denied delete and keeps the identity", async () => {
    server.use(...signedIn(), listIdentities(), deleteIdentities([], [grace.id]));
    const user = userEvent.setup();
    renderRoute("/identities");

    await deleteSelected(user, [grace.id], 1);

    expect(await screen.findByText(m.identitiesDeleteForbidden({ count: 1 }))).toBeInTheDocument();
    await waitFor(async () => {
      expect(await findRows()).toHaveLength(3);
    });
  });

  it("doesn't delete the signed-in identity", async () => {
    const requested: Array<string> = [];
    server.use(...signedIn(), listIdentities(), deleteIdentities(requested));
    const user = userEvent.setup();
    renderRoute("/identities");

    await findRows();
    await user.click(within(rowOf(ada.id)).getByRole("checkbox"));
    expect(screen.getByText(m.dataTableSelected({ count: 1 }))).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: m.dataTableDelete() })).not.toBeInTheDocument();

    // Among others, Delete leaves it out.
    await deleteSelected(user, [grace.id], 1);

    expect(await screen.findByText(m.identityDeleted({ name: "grace" }))).toBeInTheDocument();
    expect(requested).toEqual([grace.id]);
    expect(rowOf(ada.id)).toBeInTheDocument();
  });
});

// The list of an actor that may do less than everything: the permissions list and each identity's
// allowedActions, as the API answers them for that actor.
describe("/identities with fewer permissions", () => {
  beforeEach(() => {
    toastQueue.clear();
  });

  // An actor that may only read: no identity allows it anything.
  function signedInAsReader() {
    stored = identities.map((identity) => ({ ...identity, allowedActions: [] }));
    server.use(...signedIn(), listIdentities());
    server.use(permissions(["identity:read"]));
  }

  it("doesn't offer a reader to create an identity", async () => {
    signedInAsReader();
    renderRoute("/identities");

    await findRows();
    expect(screen.queryByRole("button", { name: m.identityNew() })).not.toBeInTheDocument();
  });

  it("doesn't offer a reader to create the first identity either", async () => {
    signedInAsReader();
    stored = [];
    renderRoute("/identities");

    expect(await screen.findByText(m.identitiesEmpty())).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: m.identityNew() })).not.toBeInTheDocument();
  });

  it("doesn't offer a reader to delete", async () => {
    signedInAsReader();
    const user = userEvent.setup();
    renderRoute("/identities");

    await findRows();
    await user.click(within(rowOf(grace.id)).getByRole("checkbox"));
    expect(screen.getByText(m.dataTableSelected({ count: 1 }))).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: m.dataTableDelete() })).not.toBeInTheDocument();
  });

  it("deletes only the identities that allow it", async () => {
    // Self-service: the actor may delete only grace, and edit nothing else.
    stored = identities.map((identity) => ({
      ...identity,
      allowedActions: identity.id === grace.id ? ["identity:delete" as const] : [],
    }));
    const requested: Array<string> = [];
    server.use(...signedIn(), listIdentities(), deleteIdentities(requested));
    server.use(permissions(["identity:read", "identity:delete"]));
    const user = userEvent.setup();
    renderRoute("/identities");

    await findRows();
    // Neither the "New identity" button…
    expect(screen.queryByRole("button", { name: m.identityNew() })).not.toBeInTheDocument();
    await user.click(within(rowOf(hidden.id)).getByRole("checkbox"));
    expect(screen.queryByRole("button", { name: m.dataTableDelete() })).not.toBeInTheDocument();

    // …nor Delete of hidden, which Delete leaves out among others.
    await deleteSelected(user, [grace.id], 1);

    expect(await screen.findByText(m.identityDeleted({ name: "grace" }))).toBeInTheDocument();
    expect(requested).toEqual([grace.id]);
  });
});
