import { screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, it } from "vitest";

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
  it("loads the permissions with the session", async () => {
    server.use(...signedIn());
    const requests = countedPermissions();
    renderRoute("/");

    expect(await screen.findByText(m.welcome())).toBeInTheDocument();
    expect(requests.count).toBe(1);
  });

  it("fails a page at once when the permissions don't load", async () => {
    const requests = { count: 0 };
    server.use(...signedIn());
    // After signedIn(), whose permissions it replaces.
    server.use(
      http.get("/api/v1/identities/me/permissions", () => {
        requests.count++;
        return HttpResponse.json(
          { type: "about:blank", title: "Bad Gateway", status: 502 },
          { status: 502, headers: { "Content-Type": "application/problem+json" } },
        );
      }),
    );
    const { router } = renderRoute(
      "/",
      createClients({ defaultOptions: { queries: { retryDelay: 0 } } }),
    );

    await waitFor(() => {
      expect(router.state.matches.some((match) => match.status === "error")).toBe(true);
    });
    expect(screen.queryByText(m.welcome())).not.toBeInTheDocument();
    expect(requests.count).toBe(1);
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
