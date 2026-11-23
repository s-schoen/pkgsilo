import { http, HttpResponse } from "msw";

import { getGetIdentityMockHandler } from "@/api/gen/client/identities/identities.msw.ts";
import { getListPermissionsMockHandler } from "@/api/gen/client/permissions/permissions.msw.ts";
import { getGetCurrentSessionMockHandler } from "@/api/gen/client/sessions/sessions.msw.ts";
import { allActions } from "@/mocks/actions.ts";

// The identity that signedIn() signs in as.
export const testIdentity = {
  id: "00000000-0000-4000-8000-000000000002",
  username: "ada",
  displayName: "Ada Lovelace",
};

// The handler of the signed-in identity's permissions, with and without owner: allowedActions,
// every action by default. A test of what a page hides overrides signedIn()'s with
// server.use(permissions([…])).
export function permissions(allowedActions: ReadonlyArray<string> = allActions) {
  return getListPermissionsMockHandler({
    data: { id: testIdentity.id, allowedActions: [...allowedActions] },
  });
}

// The handlers of a browser that is signed in as testIdentity, for a test of a page that needs a
// session: server.use(...signedIn()). The session ends in a day. The identity may do everything
// (permissions()).
export function signedIn() {
  const now = new Date();
  return [
    getGetCurrentSessionMockHandler({
      data: {
        id: "00000000-0000-4000-8000-000000000001",
        identityId: testIdentity.id,
        createdAt: now.toISOString(),
        expiresAt: new Date(now.getTime() + 24 * 60 * 60 * 1000).toISOString(),
      },
    }),
    getGetIdentityMockHandler({ data: testIdentity }),
    permissions(),
  ];
}

// The problem of a request without a valid session, as the API answers it.
export function unauthorized() {
  return HttpResponse.json(
    { type: "about:blank", title: "Unauthorized", status: 401 },
    { status: 401, headers: { "Content-Type": "application/problem+json" } },
  );
}

// The handler of a browser that isn't signed in: the session's check answers 401.
export function signedOut() {
  return http.get("/api/v1/sessions/current", unauthorized);
}
