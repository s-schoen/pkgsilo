import { fixedIdentities } from "@/mocks/identities-data.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type { Pagination } from "@/api/gen/client/model/pagination.ts";
import type { Problem } from "@/api/gen/client/model/problem.ts";
import type { ValidationError } from "@/api/gen/client/model/validationError.ts";

// The outcome of a store's operation: its value, or the problem that the API answers instead.
export type StoreResult<T> = { ok: true; value: T } | { ok: false; problem: Problem };

export type IdentityPage = { data: Array<IdentityResponse>; pagination: Pagination };

// The fields of a create or replace request, and of a merge patch, as the request's JSON holds
// them: the store checks them, as the API does, so they may be of any type.
type IdentityBody = Record<string, unknown>;

export type IdentitiesStore = {
  list: (page: { offset: number; limit: number }) => IdentityPage;
  get: (id: string) => StoreResult<IdentityResponse>;
  create: (body: IdentityBody) => StoreResult<IdentityResponse>;
  replace: (id: string, body: IdentityBody) => StoreResult<IdentityResponse>;
  patch: (id: string, body: IdentityBody) => StoreResult<IdentityResponse>;
  remove: (id: string) => StoreResult<undefined>;
};

export type IdentitiesStoreOptions = {
  // The identities that the store starts with: fixedIdentities by default.
  identities?: ReadonlyArray<IdentityResponse>;
  // The identity whose changes the store records in createdBy and updatedBy.
  actorId: string;
  // The time of a change: the clock by default.
  now?: () => Date;
  // The id of a new identity: a random UUID by default.
  newId?: () => string;
};

// The problems of the API's identities (feature/identity/api, core/request), with their details.
const problemTypeBase = "https://pkgsilo/problems/";

const identityNotFound: Problem = {
  type: `${problemTypeBase}identity-not-found`,
  title: "Identity not found",
  status: 404,
  detail: "The identity does not exist.",
};

const usernameTaken: Problem = {
  type: `${problemTypeBase}username-taken`,
  title: "Username taken",
  status: 409,
  detail: "The username is already taken.",
};

function invalidBody(errors: Array<ValidationError>): Problem {
  return {
    type: `${problemTypeBase}invalid-body`,
    title: "Invalid request body",
    status: 422,
    detail: "The request body contains invalid members.",
    errors,
  };
}

// The rules of the service (feature/identity/service), with the messages of its validation.
const usernamePattern = /^[a-z0-9]+(?:[._-][a-z0-9]+)*$/;
const controlCharacter = /\p{Cc}/u;

const fieldRules = {
  username: (value: string) => {
    if (value === "") return "cannot be blank";
    if (value.length < 3 || value.length > 64) return "the length must be between 3 and 64";
    if (!usernamePattern.test(value)) {
      return "must consist of lowercase letters and digits, optionally separated by single '.', '_' or '-'";
    }
    return undefined;
  },
  displayName: (value: string) => {
    if (value === "") return "cannot be blank";
    if (value.trim() === "") return "must not be blank";
    if (controlCharacter.test(value)) return "must not contain control characters";
    // The u flag counts code points, as the API counts characters, not UTF-16 units.
    if (!/^[\s\S]{1,128}$/u.test(value)) return "the length must be between 1 and 128";
    return undefined;
  },
};

type Field = keyof typeof fieldRules;
const fields: ReadonlyArray<Field> = ["username", "displayName"];

// The errors of body's fields: required ones must be present; a present one must be a string that
// follows its rules. A merge patch's null would remove the field, which an identity can't lack.
function validate(body: IdentityBody, required: boolean): Array<ValidationError> {
  const errors: Array<ValidationError> = [];
  for (const field of fields) {
    const value = body[field];
    let detail: string | undefined;
    if (value === undefined) {
      detail = required ? "cannot be blank" : undefined;
    } else if (value === null) {
      detail = "cannot be blank";
    } else if (typeof value !== "string") {
      detail = "must be a string";
    } else {
      detail = fieldRules[field](value);
    }
    if (detail !== undefined) {
      errors.push({ pointer: `/${field}`, detail });
    }
  }
  return errors;
}

// An in-memory store of identities that answers as the API's identities do, for the mock API of
// `task ui:dev:mock`. The signed-in actor may do everything, so no field is ever absent. It keeps
// the order of creation, as the API lists them.
export function createIdentitiesStore(options: IdentitiesStoreOptions): IdentitiesStore {
  const { actorId, now = () => new Date(), newId = () => crypto.randomUUID() } = options;
  const identities = new Map<string, IdentityResponse>(
    (options.identities ?? fixedIdentities).map((identity) => [identity.id, { ...identity }]),
  );

  const isTaken = (username: string, exceptId?: string) =>
    [...identities.values()].some(
      (identity) => identity.username === username && identity.id !== exceptId,
    );

  // Applies the checked fields of body to the identity id, as actorId at now.
  const write = (id: string, body: IdentityBody): StoreResult<IdentityResponse> => {
    const current = identities.get(id);
    if (current === undefined) {
      return { ok: false, problem: identityNotFound };
    }
    const changed: IdentityResponse = { ...current, updatedBy: actorId };
    for (const field of fields) {
      const value = body[field];
      if (typeof value === "string") {
        changed[field] = value;
      }
    }
    if (changed.username !== undefined && isTaken(changed.username, id)) {
      return { ok: false, problem: usernameTaken };
    }
    changed.updatedAt = now().toISOString();
    identities.set(id, changed);
    return { ok: true, value: { ...changed } };
  };

  return {
    list: ({ offset, limit }) => {
      const all = [...identities.values()];
      const next = offset + limit;
      return {
        data: all.slice(offset, next).map((identity) => ({ ...identity })),
        pagination: { offset, limit, nextOffset: next < all.length ? next : null },
      };
    },

    get: (id) => {
      const identity = identities.get(id);
      return identity === undefined
        ? { ok: false, problem: identityNotFound }
        : { ok: true, value: { ...identity } };
    },

    create: (body) => {
      const errors = validate(body, true);
      if (errors.length > 0) {
        return { ok: false, problem: invalidBody(errors) };
      }
      const username = String(body.username);
      if (isTaken(username)) {
        return { ok: false, problem: usernameTaken };
      }
      const at = now().toISOString();
      const identity: IdentityResponse = {
        id: newId(),
        username,
        displayName: String(body.displayName),
        createdBy: actorId,
        updatedBy: actorId,
        createdAt: at,
        updatedAt: at,
      };
      identities.set(identity.id, identity);
      return { ok: true, value: { ...identity } };
    },

    // The API checks the body before it looks the identity up, as its handler decodes the body
    // before it calls the service.
    replace: (id, body) => {
      const errors = validate(body, true);
      return errors.length > 0 ? { ok: false, problem: invalidBody(errors) } : write(id, body);
    },

    patch: (id, body) => {
      const errors = validate(body, false);
      return errors.length > 0 ? { ok: false, problem: invalidBody(errors) } : write(id, body);
    },

    remove: (id) =>
      identities.delete(id)
        ? { ok: true, value: undefined }
        : { ok: false, problem: identityNotFound },
  };
}
