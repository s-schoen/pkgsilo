import { getApiKeysMock } from "@/api/gen/client/api-keys/api-keys.msw.ts";
import { getGroupsMock } from "@/api/gen/client/groups/groups.msw.ts";
import { getHealthMock } from "@/api/gen/client/health/health.msw.ts";
import { getPasswordsMock } from "@/api/gen/client/passwords/passwords.msw.ts";
import { getPoliciesMock } from "@/api/gen/client/policies/policies.msw.ts";
import { createIdentitiesHandlers } from "@/mocks/identities-handlers.ts";
import { createIdentitiesStore } from "@/mocks/identities-store.ts";
import { createSessionHandlers, requireActor } from "@/mocks/session-handlers.ts";
import { createMockSession } from "@/mocks/session.ts";

// The handlers of `task ui:dev:mock`. Orval generates the rest from the OpenAPI document, so every
// route of the API answers a response of its shape with random data: replace it with fixed data in
// a handler here once a view needs stable lists. MSW takes the first handler that matches, so the
// handlers that override a generated one come first.
//
// The session starts signed in as the admin, so that the UI needs no sign-in; signing in again
// picks a persona by its username (admin, reader or self, personas.ts), which decides the
// permissions, the identities' allowedActions and which writes answer 403. The other resources'
// generated handlers don't check them.
//
// The session and the identities live in the page: each call starts anew with the fixed
// identities, and browser.ts calls it once per page load, so the changes last until a reload.
export function createHandlers() {
  const session = createMockSession();
  const actor = () => requireActor(session);
  const store = createIdentitiesStore({ actorId: () => actor().id });
  return [
    ...createSessionHandlers(session, store),
    ...createIdentitiesHandlers(store, actor),
    ...getGroupsMock(),
    ...getPoliciesMock(),
    ...getApiKeysMock(),
    ...getPasswordsMock(),
    ...getHealthMock(),
  ];
}
