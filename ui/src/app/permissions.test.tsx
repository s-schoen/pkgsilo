import { focusManager } from "@tanstack/react-query";
import { screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, describe, expect, it } from "vitest";

import { patchIdentity } from "@/api/gen/client/identities/identities.ts";
import { getListPermissionsMockHandler } from "@/api/gen/client/permissions/permissions.msw.ts";
import { createClients } from "@/app/clients.ts";
import { m } from "@/paraglide/messages.js";
import { renderRoute } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { signedIn, testIdentity } from "@/test/session.ts";

// Answers the signed-in identity's permissions and counts the requests.
function countedPermissions() {
  const requests = { count: 0 };
  server.use(
    getListPermissionsMockHandler(() => {
      requests.count++;
      return { data: { id: testIdentity.id, allowedActions: ["identity:read"] } };
    }),
  );
  return requests;
}

// The permissions of the signed-in identity, through the whole app.
describe("permissions", () => {
  afterEach(() => {
    focusManager.setFocused(undefined);
  });

  it("loads the permissions with the session", async () => {
    server.use(...signedIn());
    const requests = countedPermissions();
    renderRoute("/");

    expect(await screen.findByText(m.welcome())).toBeInTheDocument();
    expect(requests.count).toBe(1);
  });

  it("hides nothing while the permissions don't load, and loads them again", async () => {
    const answer = { failing: true };
    server.use(...signedIn());
    // After signedIn(), whose permissions it replaces.
    server.use(
      http.get("/api/v1/identities/me/permissions", () =>
        answer.failing
          ? HttpResponse.json(
              { type: "about:blank", title: "Bad Gateway", status: 502 },
              { status: 502, headers: { "Content-Type": "application/problem+json" } },
            )
          : HttpResponse.json({ data: { id: testIdentity.id, allowedActions: [] } }),
      ),
    );
    renderRoute("/", createClients({ defaultOptions: { queries: { retryDelay: 0 } } }));

    expect(await screen.findByText(m.welcome())).toBeInTheDocument();
    expect(screen.getByRole("link", { name: m.identities() })).toBeInTheDocument();

    answer.failing = false;
    focusManager.setFocused(false);
    focusManager.setFocused(true);

    await waitFor(() => {
      expect(screen.queryByRole("link", { name: m.identities() })).not.toBeInTheDocument();
    });
    expect(screen.getByText(m.welcome())).toBeInTheDocument();
  });

  it("refetches the permissions after a denied request", async () => {
    server.use(
      ...signedIn(),
      http.patch("/api/v1/identities/:id", () =>
        HttpResponse.json(
          {
            type: "https://pkgsilo/problems/fields-denied",
            title: "Fields denied",
            status: 403,
          },
          { status: 403, headers: { "Content-Type": "application/problem+json" } },
        ),
      ),
    );
    const requests = countedPermissions();
    renderRoute("/");
    expect(await screen.findByText(m.welcome())).toBeInTheDocument();

    await expect(patchIdentity(testIdentity.id, { displayName: "Ada" })).rejects.toMatchObject({
      status: 403,
    });

    await waitFor(() => {
      expect(requests.count).toBe(2);
    });
  });
});
