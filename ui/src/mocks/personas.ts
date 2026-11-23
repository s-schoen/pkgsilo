import { IdentityResponseAllowedActionsItem } from "@/api/gen/client/model/identityResponseAllowedActionsItem.ts";
import { allActions, isOwnedAction } from "@/mocks/actions.ts";
import { adminId, readerId, selfId } from "@/mocks/identities-data.ts";

// The personas of the mock API, which the sign-in's username picks, each a fixed identity:
// - admin may do everything;
// - reader may only read;
// - self may read everything and edit only its own identity's fields: the case that shows why an
//   object's own allowedActions are exact, while the permissions list only tells that an action is
//   allowed on some identity.
export type Persona = "admin" | "reader" | "self";

export const personaIds: Readonly<Record<Persona, string>> = {
  admin: adminId,
  reader: readerId,
  self: selfId,
};

// The signed-in identity and its persona.
export type Actor = { id: string; persona: Persona };

// The persona whose username is username, if any.
export function personaOf(username: string): Persona | undefined {
  return Object.keys(personaIds).find((persona) => persona === username) as Persona | undefined;
}

// Whether action reads, such as identity:read, identity:read:username or group:readMembers.
function isRead(action: string): boolean {
  return action.split(":")[1]?.startsWith("read") ?? false;
}

// Whether actor may do action on the resource with this id, or, for a create, on a new one. The
// policies of the personas, as Cedar would decide them.
export function isAllowed(actor: Actor, action: string, resourceId?: string): boolean {
  switch (actor.persona) {
    case "admin":
      return true;
    case "reader":
      return isRead(action);
    case "self":
      return isRead(action) || (action.startsWith("identity:update:") && resourceId === actor.id);
  }
}

// Whether actor may do action on some resource of its type: the permissions list without owner.
function isAllowedOnSome(actor: Actor, action: string): boolean {
  return (
    isAllowed(actor, action) || (actor.persona === "self" && action.startsWith("identity:update:"))
  );
}

// The permissions list of GET /identities/me/permissions: without owner, every action that's allowed
// on some resource; with owner, the actions on the resources that the identity with that id owns.
export function permissionsOf(actor: Actor, ownerId?: string): Array<string> {
  return ownerId === undefined
    ? allActions.filter((action) => isAllowedOnSome(actor, action))
    : allActions.filter((action) => isOwnedAction(action) && isAllowed(actor, action, ownerId));
}

// The allowedActions of the identity with this id, as actor reads it.
export function identityActions(
  actor: Actor,
  id: string,
): Array<IdentityResponseAllowedActionsItem> {
  return Object.values(IdentityResponseAllowedActionsItem).filter((action) =>
    isAllowed(actor, action, id),
  );
}
