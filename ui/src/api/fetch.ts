// The fetch that every function of the generated client calls (orval.config.ts's mutator), so that
// the UI sees every response of the API in one place, whether a query, a mutation or a collection
// sent the request.
//
// It resolves with { data, status, headers }, where data is the response's body, and rejects an
// error response with an Error whose status is the HTTP status and whose info is the problem.

// The error that a request of the generated client rejects with: the response's HTTP status, and
// the problem that its body holds, typed by the operation's error responses. Orval reads this type
// from this file by its name.
export type ErrorType<Info> = Error & { info?: Info; status?: number };

// What a listener of onErrorResponse learns of an error response.
export type ErrorResponse = {
  // The request's method, such as PATCH.
  method: string;
  // The request's path, without the query, such as /api/v1/identities/<id>.
  path: string;
  status: number;
  // The problem that the response's body holds, if any.
  info: unknown;
};

type ErrorResponseListener = (response: ErrorResponse) => void;

const errorResponseListeners = new Set<ErrorResponseListener>();

// Calls listener with every error response of the API, before the request rejects. Returns the
// function that stops it.
export function onErrorResponse(listener: ErrorResponseListener): () => void {
  errorResponseListeners.add(listener);
  return () => {
    errorResponseListeners.delete(listener);
  };
}

export async function apiFetch<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, init);
  const body = [204, 205, 304].includes(response.status) ? null : await response.text();
  const data: unknown = body ? JSON.parse(body) : {};
  if (!response.ok) {
    const error: ErrorType<unknown> = new Error(
      `${init?.method ?? "GET"} ${url}: ${String(response.status)}`,
    );
    error.info = data;
    error.status = response.status;
    const errorResponse = {
      method: init?.method ?? "GET",
      path: url.split("?")[0],
      status: response.status,
      info: data,
    };
    for (const listener of errorResponseListeners) {
      listener(errorResponse);
    }
    throw error;
  }
  // The generated function names the type of its success response.
  return { data, status: response.status, headers: response.headers } as T;
}
