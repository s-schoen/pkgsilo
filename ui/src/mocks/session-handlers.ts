import { http, HttpResponse } from "msw";

import { permissionsOf } from "@/mocks/personas.ts";
import { forbidden, problemResponse, unauthorized } from "@/mocks/problems.ts";

import type { IdentitiesStore } from "@/mocks/identities-store.ts";
import type { Actor } from "@/mocks/personas.ts";
import type { MockSession } from "@/mocks/session.ts";

// The session's id, which the mock API never changes.
const sessionId = "00000000-0000-4000-8000-000000000001";

// The session of actor, which ends a year from now, so that it never expires during a dev session.
function sessionOf(actor: Actor) {
  const now = new Date();
  return {
    id: sessionId,
    identityId: actor.id,
    createdAt: now.toISOString(),
    expiresAt: new Date(now.getTime() + 365 * 24 * 60 * 60 * 1000).toISOString(),
  };
}

// The signed-in actor, which the guard of createSessionHandlers makes sure of for every handler
// after it. MSW answers the error with 500.
export function requireActor(session: MockSession): Actor {
  const actor = session.actor();
  if (actor === undefined) {
    throw new Error("signed out: the session's guard answers 401 first");
  }
  return actor;
}

// The handlers of the session and of what it's signed in as, for the mock API of
// `task ui:dev:mock`: they come before every other handler.
//
// - Signed out, every request but the sign-in answers 401.
// - A sign-in's username picks the persona (personas.ts), whatever the password; any other username
//   answers 401, as wrong credentials do.
// - GET /identities/{id}/permissions lists the persona's permissions, for the actor's own id or me
//   only, as the API does; another readable identity answers 403. Its owner, an id or me, must be
//   one of store's identities.
export function createSessionHandlers(session: MockSession, store: IdentitiesStore) {
  const actor = () => requireActor(session);

  return [
    http.all("*/api/v1/*", ({ request }) => {
      const isSignIn =
        request.method === "POST" && new URL(request.url).pathname === "/api/v1/sessions";
      if (session.actor() === undefined && !isSignIn) {
        return problemResponse(unauthorized);
      }
      // Passes the request on to the next handler.
      return undefined;
    }),

    http.post("*/api/v1/sessions", async ({ request }) => {
      const body: unknown = await request.json().catch(() => undefined);
      const username =
        typeof body === "object" && body !== null && "username" in body ? body.username : undefined;
      const signedIn = typeof username === "string" ? session.signIn(username) : undefined;
      return signedIn === undefined
        ? problemResponse(unauthorized)
        : HttpResponse.json({ data: sessionOf(signedIn) }, { status: 201 });
    }),

    http.get("*/api/v1/sessions/current", () => HttpResponse.json({ data: sessionOf(actor()) })),

    http.delete("*/api/v1/sessions/current", () => {
      session.signOut();
      return new HttpResponse(null, { status: 204 });
    }),

    http.get<{ id: string }>("*/api/v1/identities/:id/permissions", ({ params, request }) => {
      const signedIn = actor();
      if (params.id !== "me" && params.id !== signedIn.id) {
        const identity = store.get(params.id);
        return problemResponse(identity.ok ? forbidden : identity.problem);
      }
      const owner = new URL(request.url).searchParams.get("owner");
      const ownerId = owner === "me" ? signedIn.id : (owner ?? undefined);
      if (ownerId !== undefined) {
        const identity = store.get(ownerId);
        if (!identity.ok) {
          return problemResponse(identity.problem);
        }
      }
      return HttpResponse.json({
        data: { id: signedIn.id, allowedActions: permissionsOf(signedIn, ownerId) },
      });
    }),
  ];
}
