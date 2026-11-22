import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { delay, http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";

import { getListIdentitiesMockHandler } from "@/api/gen/client/identities/identities.msw.ts";
import { toastQueue } from "@/components/ui/toast";
import { displayNameSchema, usernameSchema } from "@/features/identities/utils/identity-schema.ts";
import { m } from "@/paraglide/messages.js";
import { renderRoute } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { signedIn, testIdentity } from "@/test/session.ts";

import type { CreateIdentityRequest } from "@/api/gen/client/model/createIdentityRequest.ts";
import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

const created = "00000000-0000-4000-8000-000000000042";

// The identities that the list holds before the create, and after it the created one too.
function listIdentities(identities: Array<IdentityResponse> = [testIdentity]) {
  return getListIdentitiesMockHandler({
    data: identities,
    pagination: { offset: 0, limit: 100, nextOffset: null },
  });
}

// Answers a create with the created identity and records the bodies of the requests.
function createIdentity(bodies: Array<CreateIdentityRequest>) {
  return http.post("/api/v1/identities", async ({ request }) => {
    const body = (await request.json()) as CreateIdentityRequest;
    bodies.push(body);
    // A moment, as the API takes one, so that the pending button shows.
    await delay(20);
    server.use(listIdentities([testIdentity, { id: created, ...body }]));
    return HttpResponse.json({ data: { id: created, ...body } }, { status: 201 });
  });
}

// Answers a create with a problem of this status and type.
function createProblem(status: number, type: string, title: string) {
  return http.post("/api/v1/identities", () =>
    HttpResponse.json(
      { type, title, status },
      { status, headers: { "Content-Type": "application/problem+json" } },
    ),
  );
}

async function findDisplayName() {
  return screen.findByRole("textbox", { name: m.identityDisplayName() });
}

function username() {
  return screen.getByRole("textbox", { name: m.username() });
}

describe("/identities/new", () => {
  beforeEach(() => {
    toastQueue.clear();
  });

  it("shows the breadcrumbs, the fields and the actions", async () => {
    server.use(...signedIn(), listIdentities());
    renderRoute("/identities/new");

    expect(await findDisplayName()).toHaveAttribute("placeholder", m.identityDisplayName());
    expect(username()).toHaveValue("");
    expect(screen.getByRole("heading", { level: 1, name: m.identityNew() })).toBeInTheDocument();
    const breadcrumbs = screen.getByRole("navigation", { name: m.breadcrumbs() });
    expect(within(breadcrumbs).getByRole("link", { name: m.identities() })).toHaveAttribute(
      "href",
      "/identities",
    );
    expect(within(breadcrumbs).getByText(m.identityNew())).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: m.identityCreate() })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: m.cancel() })).toBeInTheDocument();
  });

  it("creates the identity and replaces the page with its own", async () => {
    const bodies: Array<CreateIdentityRequest> = [];
    server.use(...signedIn(), listIdentities(), createIdentity(bodies));
    const user = userEvent.setup();
    const { router } = renderRoute("/identities/new");

    await user.type(await findDisplayName(), "Alan Turing");
    await user.type(username(), "alan");
    const create = screen.getByRole("button", { name: m.identityCreate() });
    await user.click(create);

    expect(create).toHaveAttribute("data-pending");
    expect(
      await screen.findByRole("heading", { level: 1, name: "Alan Turing" }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(`/identities/${created}`);
    expect(screen.getByText(m.identityCreated({ name: "Alan Turing" }))).toBeInTheDocument();
    expect(bodies).toEqual([{ displayName: "Alan Turing", username: "alan" }]);

    // The create page is gone from the history: Back leads to the page before it.
    router.history.back();
    await waitFor(() => {
      expect(router.state.location.pathname).not.toBe("/identities/new");
    });
  });

  it("submits with Enter in a field", async () => {
    const bodies: Array<CreateIdentityRequest> = [];
    server.use(...signedIn(), listIdentities(), createIdentity(bodies));
    const user = userEvent.setup();
    renderRoute("/identities/new");

    await user.type(await findDisplayName(), "Alan Turing");
    await user.type(username(), "alan{Enter}");

    expect(
      await screen.findByRole("heading", { level: 1, name: "Alan Turing" }),
    ).toBeInTheDocument();
    expect(bodies).toEqual([{ displayName: "Alan Turing", username: "alan" }]);
  });

  it("shows the errors of invalid input and creates nothing", async () => {
    const bodies: Array<CreateIdentityRequest> = [];
    server.use(...signedIn(), listIdentities(), createIdentity(bodies));
    const user = userEvent.setup();
    renderRoute("/identities/new");

    await user.type(await findDisplayName(), " ");
    await user.type(username(), "Alan");
    await user.click(screen.getByRole("button", { name: m.identityCreate() }));

    const alerts = screen.getAllByRole("alert").map((alert) => alert.textContent);
    expect(alerts).toContain(displayNameSchema.safeParse(" ").error?.issues[0]?.message);
    expect(alerts).toContain(usernameSchema.safeParse("Alan").error?.issues[0]?.message);
    expect(username()).toBeInvalid();
    expect(screen.getByRole("button", { name: m.identityCreate() })).toBeDisabled();
    expect(bodies).toEqual([]);
  });

  it("shows a taken username on its field, until the user changes it", async () => {
    server.use(
      ...signedIn(),
      listIdentities(),
      createProblem(409, "https://pkgsilo/problems/username-taken", "Username taken"),
    );
    const user = userEvent.setup();
    const { router } = renderRoute("/identities/new");

    await user.type(await findDisplayName(), "Ada Again");
    await user.type(username(), `${testIdentity.username}{Enter}`);

    expect(await screen.findByRole("alert")).toHaveTextContent(m.identityUsernameTaken());
    expect(username()).toBeInvalid();
    expect(username()).toHaveValue(testIdentity.username);
    expect(router.state.location.pathname).toBe("/identities/new");

    await user.type(username(), "2");

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: m.identityCreate() })).toBeEnabled();
  });

  it("keeps the input after a failure, which a toast shows", async () => {
    server.use(
      ...signedIn(),
      listIdentities(),
      createProblem(403, "https://pkgsilo/problems/forbidden", "Forbidden"),
    );
    const user = userEvent.setup();
    const { router } = renderRoute("/identities/new");

    await user.type(await findDisplayName(), "Alan Turing");
    await user.type(username(), "alan{Enter}");

    expect(await screen.findByText(m.identityCreateForbidden())).toBeInTheDocument();
    expect(await findDisplayName()).toHaveValue("Alan Turing");
    expect(username()).toHaveValue("alan");
    expect(router.state.location.pathname).toBe("/identities/new");
  });

  it("leads back to the list on Cancel", async () => {
    server.use(...signedIn(), listIdentities());
    const user = userEvent.setup();
    const { router } = renderRoute("/identities/new");

    await findDisplayName();
    await user.click(screen.getByRole("button", { name: m.cancel() }));

    expect(
      await screen.findByRole("heading", { level: 1, name: m.identities() }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/identities");
  });

  it("is reached from the list's header and its empty state", async () => {
    server.use(...signedIn(), listIdentities([]));
    const user = userEvent.setup();
    const { router } = renderRoute("/identities");

    expect(await screen.findByText(m.identitiesEmpty())).toBeInTheDocument();
    // The header's button, then the empty state's.
    const newIdentity = (index: number) =>
      screen.getAllByRole("button", { name: m.identityNew() })[index] ?? document.body;
    expect(screen.getAllByRole("button", { name: m.identityNew() })).toHaveLength(2);

    await user.click(newIdentity(1));
    expect(await findDisplayName()).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/identities/new");

    await user.click(screen.getByRole("button", { name: m.cancel() }));
    await screen.findByText(m.identitiesEmpty());
    await user.click(newIdentity(0));
    expect(await findDisplayName()).toBeInTheDocument();
  });
});
