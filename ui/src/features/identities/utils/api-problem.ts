// The problem types of the API's identities that the UI tells apart (feature/identity/api).
export const problemTypes = {
  usernameTaken: "https://pkgsilo/problems/username-taken",
  invalidBody: "https://pkgsilo/problems/invalid-body",
};

// The parts of the generated client's error that tell the failures apart: the response's status and
// its problem's type.
export type ApiProblem = { status?: number; info?: { type?: string } };

export function apiProblem(error: unknown): ApiProblem {
  return typeof error === "object" && error !== null ? error : {};
}

// The API's 409 for a username that another identity has.
export function isUsernameTaken(error: unknown): boolean {
  const { status, info } = apiProblem(error);
  return status === 409 && info?.type === problemTypes.usernameTaken;
}
