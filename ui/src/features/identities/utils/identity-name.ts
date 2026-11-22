import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";

// The id of pkgsilo itself, in createdBy and updatedBy: the nil UUID.
export const PKGSILO_ID = "00000000-0000-0000-0000-000000000000";

// The name an identity shows by: its display name, else its username, else its id, since the
// actor may be denied either field.
export function identityName(identity: IdentityResponse): string {
  return identity.displayName ?? identity.username ?? identity.id;
}

// A username as a handle, as a chip shows it: @ada.
export function handle(username: string): string {
  return `@${username}`;
}
