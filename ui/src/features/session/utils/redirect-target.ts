// The path of the sign-in page, where a request without a session leads.
export const signInPath = "/signin";

// The page to go to after a sign-in: the redirect search parameter of /signin when it's a path of
// this app, else the start page. Anything else, such as //evil.example (a URL of another host that
// starts with a slash) or https://evil.example, would make the sign-in page an open redirect. The
// sign-in page itself leads to the start page, so that it doesn't loop.
export function safeRedirectTarget(redirect: string | undefined): string {
  if (
    redirect === undefined ||
    !redirect.startsWith("/") ||
    redirect.startsWith("//") ||
    redirect.startsWith("/\\")
  ) {
    return "/";
  }
  const { pathname } = new URL(redirect, window.location.origin);
  return pathname === signInPath ? "/" : redirect;
}
