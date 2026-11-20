import { getApiKeysMock } from "@/api/gen/client/api-keys/api-keys.msw.ts";
import { getGroupsMock } from "@/api/gen/client/groups/groups.msw.ts";
import { getHealthMock } from "@/api/gen/client/health/health.msw.ts";
import { getIdentitiesMock } from "@/api/gen/client/identities/identities.msw.ts";
import { getPasswordsMock } from "@/api/gen/client/passwords/passwords.msw.ts";
import { getPoliciesMock } from "@/api/gen/client/policies/policies.msw.ts";
import {
  getGetCurrentSessionMockHandler,
  getSessionsMock,
} from "@/api/gen/client/sessions/sessions.msw.ts";

// The session that the mock API answers with: signed in from the first request, so that the UI
// needs no login. A year from now, so that it never expires during a dev session.
function currentSession() {
  const now = new Date();
  return {
    data: {
      id: "00000000-0000-4000-8000-000000000001",
      identityId: "00000000-0000-4000-8000-000000000002",
      createdAt: now.toISOString(),
      expiresAt: new Date(now.getTime() + 365 * 24 * 60 * 60 * 1000).toISOString(),
    },
  };
}

// The handlers of `task ui:dev:mock`. Orval generates the rest from the OpenAPI document, so every
// route of the API answers a response of its shape with random data: replace it with fixed data in
// a handler here once a view needs stable lists. MSW takes the first handler that matches, so the
// handlers that override a generated one come first.
export function createHandlers() {
  return [
    getGetCurrentSessionMockHandler(() => currentSession()),
    ...getSessionsMock(),
    ...getIdentitiesMock(),
    ...getGroupsMock(),
    ...getPoliciesMock(),
    ...getApiKeysMock(),
    ...getPasswordsMock(),
    ...getHealthMock(),
  ];
}
