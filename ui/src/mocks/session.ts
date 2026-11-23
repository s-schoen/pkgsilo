import { personaIds, personaOf } from "@/mocks/personas.ts";

import type { Actor, Persona } from "@/mocks/personas.ts";

// The session of the mock API: signed in as a persona, or signed out.
export type MockSession = {
  // The signed-in identity, or undefined when signed out.
  actor: () => Actor | undefined;
  // Signs in as the persona whose username is username, and returns it; any other username signs
  // nobody in, as a wrong password wouldn't, and returns undefined.
  signIn: (username: string) => Actor | undefined;
  signOut: () => void;
};

// A session that starts signed in as the admin, so that the UI needs no sign-in.
export function createMockSession(): MockSession {
  const actorOf = (persona: Persona): Actor => ({ id: personaIds[persona], persona });
  let actor: Actor | undefined = actorOf("admin");

  return {
    actor: () => actor,
    signIn: (username) => {
      const signedIn = personaOf(username);
      actor = signedIn === undefined ? undefined : actorOf(signedIn);
      return actor;
    },
    signOut: () => {
      actor = undefined;
    },
  };
}
