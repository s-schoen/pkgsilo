import { PatchIdentityBody } from "@/api/gen/zod/identities/identities.zod.ts";
import { m } from "@/paraglide/messages.js";

// The rules of an identity's fields, as the API's service checks them (feature/identity/service):
// the generated schemas check only the types, since the API states the rules in descriptions. The
// messages are functions, so that they follow the locale at the time of the check.

const usernamePattern = /^[a-z0-9]+(?:[._-][a-z0-9]+)*$/;
const usernameError = { error: () => m.identityUsernameInvalid() };

// A username: 3 to 64 lowercase letters and digits, optionally separated by single ".", "_" or "-".
export const usernameSchema = PatchIdentityBody.shape.username
  .unwrap()
  .min(3, usernameError)
  .max(64, usernameError)
  .regex(usernamePattern, usernameError);

// At most 128 characters and no control characters, which include line breaks. The u flag counts
// code points, as the API counts characters, not UTF-16 units.
const displayNamePattern = /^\P{Cc}{0,128}$/u;

// A display name: 1 to 128 characters, not all whitespace, without control characters such as line
// breaks.
export const displayNameSchema = PatchIdentityBody.shape.displayName
  .unwrap()
  .refine((value) => value.trim() !== "", { error: () => m.identityDisplayNameRequired() })
  .regex(displayNamePattern, { error: () => m.identityDisplayNameInvalid() });
