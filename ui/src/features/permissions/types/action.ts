import type { CreateGroupRequest } from "@/api/gen/client/model/createGroupRequest.ts";
import type { CreateIdentityRequest } from "@/api/gen/client/model/createIdentityRequest.ts";
import type { CreateKeyRequest } from "@/api/gen/client/model/createKeyRequest.ts";
import type { CreatePolicyRequest } from "@/api/gen/client/model/createPolicyRequest.ts";
import type { GroupResponseAllowedActionsItem } from "@/api/gen/client/model/groupResponseAllowedActionsItem.ts";
import type { IdentityResponseAllowedActionsItem } from "@/api/gen/client/model/identityResponseAllowedActionsItem.ts";
import type { KeyResponseAllowedActionsItem } from "@/api/gen/client/model/keyResponseAllowedActionsItem.ts";
import type { PasswordResponseAllowedActionsItem } from "@/api/gen/client/model/passwordResponseAllowedActionsItem.ts";
import type { PolicyResponseAllowedActionsItem } from "@/api/gen/client/model/policyResponseAllowedActionsItem.ts";

// The resource types of the API's policies (app/testdata/schema.cedarschema).
type ResourceType = "identity" | "group" | "apikey" | "password" | "policy";

// The create action of each field of a create request, such as identity:create:username: the API
// checks one per field that the request sets.
type CreateActions<Type extends ResourceType, Request> = `${Type}:create:${keyof Request & string}`;

// The actions that the UI asks the permissions list about: what the API's permissions endpoint
// lists, by name. The API types the list as plain strings, since one endpoint lists the actions of
// every type; this union holds the ones that the UI checks: every object action (each resource's
// allowedActions enum), reading each type, and the create of each field.
export type Action =
  | IdentityResponseAllowedActionsItem
  | GroupResponseAllowedActionsItem
  | KeyResponseAllowedActionsItem
  | PasswordResponseAllowedActionsItem
  | PolicyResponseAllowedActionsItem
  | `${ResourceType}:read`
  | CreateActions<"identity", CreateIdentityRequest>
  | CreateActions<"group", CreateGroupRequest>
  | CreateActions<"apikey", CreateKeyRequest>
  | CreateActions<"policy", CreatePolicyRequest>;
