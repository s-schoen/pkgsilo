import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";

import { getListIdentitiesMockHandler } from "@/api/gen/client/identities/identities.msw.ts";
import { toastQueue } from "@/components/ui/toast";
import { identitiesCollection } from "@/features/identities/api/identities-collection.ts";
import { PKGSILO_ID } from "@/features/identities/utils/identity-name.ts";
import { m } from "@/paraglide/messages.js";
import { createTestClients } from "@/test/clients.ts";
import { renderRoute } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { signedIn, testIdentity } from "@/test/session.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type { PatchIdentityRequest } from "@/api/gen/client/model/patchIdentityRequest.ts";

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

// Answers a patch with the patched identity and records the bodies of the requests.
function patchIdentity(bodies: Array<PatchIdentityRequest>) {
  return http.patch("/api/v1/identities/:id", async ({ request }) => {
    const body = (await request.json()) as PatchIdentityRequest;
    bodies.push(body);
    return HttpResponse.json({ data: { ...ada, ...body } });
  });
}

// Answers a patch with a problem of this status and type.
function patchProblem(status: number, type: string, title: string) {
  return http.patch("/api/v1/identities/:id", () =>
    HttpResponse.json(
      { type, title, status },
      { status, headers: { "Content-Type": "application/problem+json" } },
    ),
  );
}

// Opens the inline field with this label, replaces its value and presses Enter.
async function edit(user: ReturnType<typeof userEvent.setup>, label: string, value: string) {
  await user.click(screen.getByRole("button", { name: m.inlineEditLabel({ label }) }));
  const input = screen.getByRole("textbox", { name: label });
  await user.clear(input);
  await user.type(input, `${value}{Enter}`);
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
  beforeEach(() => {
    toastQueue.clear();
  });

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

  it("saves the display name and the username, each on its own", async () => {
    const bodies: Array<PatchIdentityRequest> = [];
    server.use(...signedIn(), listIdentities(), patchIdentity(bodies));
    const user = userEvent.setup();
    renderRoute(`/identities/${ada.id}`);

    await screen.findByRole("heading", { level: 1, name: testIdentity.displayName });
    await edit(user, m.identityDisplayName(), "Ada King");

    expect(await screen.findByRole("heading", { level: 1, name: "Ada King" })).toBeInTheDocument();
    // The breadcrumbs read the collection, as the list does: they show the change too.
    const breadcrumbs = screen.getByRole("navigation", { name: m.breadcrumbs() });
    expect(within(breadcrumbs).getByText("Ada King")).toBeInTheDocument();

    await edit(user, m.username(), "ada.king");

    await waitFor(() => {
      expect(property(m.username())).toHaveTextContent("ada.king");
    });
    expect(bodies).toEqual([{ displayName: "Ada King" }, { username: "ada.king" }]);
  });

  it("keeps an invalid username in edit mode without saving it", async () => {
    const bodies: Array<PatchIdentityRequest> = [];
    server.use(...signedIn(), listIdentities(), patchIdentity(bodies));
    const user = userEvent.setup();
    renderRoute(`/identities/${ada.id}`);

    await screen.findByRole("heading", { level: 1, name: testIdentity.displayName });
    await edit(user, m.username(), "Ada");

    expect(screen.getByRole("alert")).toHaveTextContent(m.identityUsernameInvalid());
    expect(screen.getByRole("textbox", { name: m.username() })).toBeInvalid();
    expect(bodies).toEqual([]);
  });

  it("shows a taken username on the field, which stays in edit mode", async () => {
    server.use(
      ...signedIn(),
      listIdentities(),
      patchProblem(409, "https://pkgsilo/problems/username-taken", "Username taken"),
    );
    const user = userEvent.setup();
    renderRoute(`/identities/${ada.id}`);

    await screen.findByRole("heading", { level: 1, name: testIdentity.displayName });
    await edit(user, m.username(), "grace");

    expect(await screen.findByRole("alert")).toHaveTextContent(m.identityUsernameTaken());
    expect(screen.getByRole("textbox", { name: m.username() })).toHaveValue("grace");
  });

  it("reverts a forbidden change and says so in a toast", async () => {
    server.use(
      ...signedIn(),
      listIdentities(),
      patchProblem(403, "https://pkgsilo/problems/fields-denied", "Fields denied"),
    );
    const user = userEvent.setup();
    renderRoute(`/identities/${ada.id}`);

    await screen.findByRole("heading", { level: 1, name: testIdentity.displayName });
    await edit(user, m.identityDisplayName(), "Ada King");

    expect(await screen.findByText(m.identitySaveForbidden())).toBeInTheDocument();
    expect(
      await screen.findByRole("heading", { level: 1, name: testIdentity.displayName }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("doesn't offer to edit a title that isn't the display name", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute(`/identities/${grace.id}`);

    await screen.findByRole("heading", { level: 1, name: "grace" });
    expect(
      screen.queryByRole("button", { name: m.inlineEditLabel({ label: m.identityDisplayName() }) }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: m.inlineEditLabel({ label: m.username() }) }),
    ).toBeInTheDocument();
  });

  it("takes a change from elsewhere, but not while the user edits the field", async () => {
    server.use(...signedIn(), listIdentities());
    const clients = createTestClients();
    const user = userEvent.setup();
    renderRoute(`/identities/${ada.id}`, clients);
    const collection = clients.dbClient.collection(identitiesCollection);

    await screen.findByRole("heading", { level: 1, name: testIdentity.displayName });
    // Another tab renamed her: the list's next load brings it.
    server.use(
      getListIdentitiesMockHandler({
        data: [{ ...ada, displayName: "Ada King", username: "ada.king" }, grace, hidden],
        pagination: { offset: 0, limit: 100, nextOffset: null },
      }),
    );
    await user.click(
      screen.getByRole("button", { name: m.inlineEditLabel({ label: m.username() }) }),
    );
    await user.type(screen.getByRole("textbox", { name: m.username() }), "x");
    await collection.utils.refetch();

    expect(await screen.findByRole("heading", { level: 1, name: "Ada King" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: m.username() })).toHaveValue(
      `${testIdentity.username}x`,
    );

    await user.keyboard("{Escape}");

    expect(property(m.username())).toHaveTextContent("ada.king");
  });
});
