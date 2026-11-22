import { http, HttpResponse } from "msw";

import type { Problem } from "@/api/gen/client/model/problem.ts";
import type { ValidationError } from "@/api/gen/client/model/validationError.ts";
import type { IdentitiesStore, StoreResult } from "@/mocks/identities-store.ts";

const collectionPath = "*/api/v1/identities";
const resourcePath = `${collectionPath}/:id`;

// The page size of the API's lists, without a limit, and the largest one (core/request).
const defaultLimit = 50;
const maxLimit = 100;

function problemResponse(problem: Problem) {
  return HttpResponse.json(problem, {
    status: problem.status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

// The API's answer to a result: the resource in its envelope, or the problem.
function resourceResponse<T>(result: StoreResult<T>) {
  return result.ok ? HttpResponse.json({ data: result.value }) : problemResponse(result.problem);
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

// The handlers of the identities endpoints over store, for the mock API of `task ui:dev:mock`.
export function createIdentitiesHandlers(store: IdentitiesStore) {
  return [
    http.get(collectionPath, ({ request }) => {
      const page = parsePage(new URL(request.url));
      return page.ok ? HttpResponse.json(store.list(page.value)) : problemResponse(page.problem);
    }),

    http.post(collectionPath, async ({ request }) => {
      const body = await readBody(request);
      const result = body.ok ? store.create(body.value) : body;
      if (!result.ok) {
        return problemResponse(result.problem);
      }
      const location = `${new URL(request.url).pathname}/${result.value.id}`;
      return HttpResponse.json(
        { data: result.value },
        { status: 201, headers: { Location: location } },
      );
    }),

    http.get<{ id: string }>(resourcePath, ({ params }) => resourceResponse(store.get(params.id))),

    http.put<{ id: string }>(resourcePath, async ({ params, request }) => {
      const body = await readBody(request);
      return resourceResponse(body.ok ? store.replace(params.id, body.value) : body);
    }),

    http.patch<{ id: string }>(resourcePath, async ({ params, request }) => {
      const body = await readBody(request);
      return resourceResponse(body.ok ? store.patch(params.id, body.value) : body);
    }),

    http.delete<{ id: string }>(resourcePath, ({ params }) => {
      const result = store.remove(params.id);
      return result.ok ? new HttpResponse(null, { status: 204 }) : problemResponse(result.problem);
    }),
  ];
}
