import { http, HttpResponse } from "msw";

import { identityActions, isAllowed } from "@/mocks/personas.ts";
import { fieldsDenied, forbidden, problemResponse } from "@/mocks/problems.ts";

import type { IdentityResponse } from "@/api/gen/client/model/identityResponse.ts";
import type { ValidationError } from "@/api/gen/client/model/validationError.ts";
import type { IdentitiesStore, StoreResult } from "@/mocks/identities-store.ts";
import type { Actor } from "@/mocks/personas.ts";

const collectionPath = "*/api/v1/identities";
const resourcePath = `${collectionPath}/:id`;

// The page size of the API's lists, without a limit, and the largest one (core/request).
const defaultLimit = 50;
const maxLimit = 100;

// The fields of an identity that a request's body may set.
const writableFields = ["username", "displayName"] as const;

// The fields of body that actor may not set on the identity with this id, or on a new one: each
// needs its own create or update action.
function deniedFields(
  actor: Actor,
  verb: "create" | "update",
  body: Record<string, unknown>,
  id?: string,
): Array<string> {
  return writableFields.filter(
    (field) => body[field] !== undefined && !isAllowed(actor, `identity:${verb}:${field}`, id),
  );
}

// The page that the query asks for, as the API reads offset and limit, or the 400 for invalid ones.
function parsePage(url: URL): StoreResult<{ offset: number; limit: number }> {
  const errors: Array<ValidationError> = [];
  const parse = (parameter: string, fallback: number, min: number, max: number) => {
    const raw = url.searchParams.get(parameter);
    if (raw === null) {
      return fallback;
    }
    const value = Number(raw);
    if (!/^-?\d+$/.test(raw)) {
      errors.push({ parameter, detail: "must be an integer" });
    } else if (value < min) {
      errors.push({ parameter, detail: `must be no less than ${String(min)}` });
    } else if (value > max) {
      errors.push({ parameter, detail: `must be no greater than ${String(max)}` });
    }
    return value;
  };
  const offset = parse("offset", 0, 0, Number.MAX_SAFE_INTEGER);
  const limit = parse("limit", defaultLimit, 1, maxLimit);
  if (errors.length > 0) {
    return {
      ok: false,
      problem: {
        type: "https://pkgsilo/problems/invalid-query",
        title: "Invalid query parameters",
        status: 400,
        detail: "The request contains invalid query parameters.",
        errors,
      },
    };
  }
  return { ok: true, value: { offset, limit } };
}

// The request's JSON object, or the 400 that the API answers a body that isn't one with.
async function readBody(request: Request): Promise<StoreResult<Record<string, unknown>>> {
  const badRequest = {
    ok: false,
    problem: {
      type: "about:blank",
      title: "Bad Request",
      status: 400,
      detail: "The request body must be a JSON object.",
    },
  } as const;
  try {
    const body: unknown = await request.json();
    return typeof body === "object" && body !== null && !Array.isArray(body)
      ? { ok: true, value: body as Record<string, unknown> }
      : badRequest;
  } catch {
    return badRequest;
  }
}

// The handlers of the identities endpoints over store, for the mock API of `task ui:dev:mock`, as
// actor: every identity carries the actions that actor may do to it (allowedActions), and a write
// that actor may not do answers 403, a write of fields with fields-denied. Every identity is
// readable to every persona. me in a path stands for actor.
export function createIdentitiesHandlers(store: IdentitiesStore, actor: () => Actor) {
  const withActions = (identity: IdentityResponse): IdentityResponse => ({
    ...identity,
    allowedActions: identityActions(actor(), identity.id),
  });
  // The API's answer to a result: the identity in its envelope, or the problem.
  const identityResponse = (result: StoreResult<IdentityResponse>) =>
    result.ok
      ? HttpResponse.json({ data: withActions(result.value) })
      : problemResponse(result.problem);
  const idOf = (id: string) => (id === "me" ? actor().id : id);

  // Writes body's fields to the identity id with write, unless actor may not set them.
  const update = async (
    id: string,
    request: Request,
    write: (id: string, body: Record<string, unknown>) => StoreResult<IdentityResponse>,
  ) => {
    const body = await readBody(request);
    if (!body.ok) {
      return problemResponse(body.problem);
    }
    const existing = store.get(id);
    if (!existing.ok) {
      return problemResponse(existing.problem);
    }
    const denied = deniedFields(actor(), "update", body.value, id);
    return denied.length > 0
      ? problemResponse(fieldsDenied(denied))
      : identityResponse(write(id, body.value));
  };

  return [
    http.get(collectionPath, ({ request }) => {
      const page = parsePage(new URL(request.url));
      if (!page.ok) {
        return problemResponse(page.problem);
      }
      const { data, pagination } = store.list(page.value);
      return HttpResponse.json({ data: data.map(withActions), pagination });
    }),

    http.post(collectionPath, async ({ request }) => {
      const body = await readBody(request);
      if (!body.ok) {
        return problemResponse(body.problem);
      }
      const denied = deniedFields(actor(), "create", body.value);
      if (denied.length > 0) {
        return problemResponse(fieldsDenied(denied));
      }
      const result = store.create(body.value);
      if (!result.ok) {
        return problemResponse(result.problem);
      }
      const location = `${new URL(request.url).pathname}/${result.value.id}`;
      return HttpResponse.json(
        { data: withActions(result.value) },
        { status: 201, headers: { Location: location } },
      );
    }),

    http.get<{ id: string }>(resourcePath, ({ params }) =>
      identityResponse(store.get(idOf(params.id))),
    ),

    http.put<{ id: string }>(resourcePath, ({ params, request }) =>
      update(idOf(params.id), request, store.replace),
    ),

    http.patch<{ id: string }>(resourcePath, ({ params, request }) =>
      update(idOf(params.id), request, store.patch),
    ),

    http.delete<{ id: string }>(resourcePath, ({ params }) => {
      const id = idOf(params.id);
      const existing = store.get(id);
      if (!existing.ok) {
        return problemResponse(existing.problem);
      }
      if (!isAllowed(actor(), "identity:delete", id)) {
        return problemResponse(forbidden);
      }
      const result = store.remove(id);
      return result.ok ? new HttpResponse(null, { status: 204 }) : problemResponse(result.problem);
    }),
  ];
}
