import { HttpResponse } from "msw";

import type { Problem } from "@/api/gen/client/model/problem.ts";

// The response of a problem, as the API answers one.
export function problemResponse(problem: Problem) {
  return HttpResponse.json(problem, {
    status: problem.status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

// The API's problems that every endpoint may answer (core/envelope, core/request).
export const unauthorized: Problem = { type: "about:blank", title: "Unauthorized", status: 401 };

export const forbidden: Problem = { type: "about:blank", title: "Forbidden", status: 403 };

export function fieldsDenied(fields: ReadonlyArray<string>): Problem {
  return {
    type: "https://pkgsilo/problems/fields-denied",
    title: "Fields denied",
    status: 403,
    detail: "The request body sets members that the caller may not set.",
    errors: fields.map((field) => ({
      pointer: `/${field}`,
      detail: "may not be set by the caller",
    })),
  };
}
