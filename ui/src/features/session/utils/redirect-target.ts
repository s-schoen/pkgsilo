// The path of the sign-in page, where a request without a session leads.
export const signInPath = "/signin";

// A control character, such as a tab or a newline, which a URL's parser drops: "/\t/evil.example"
// would be //evil.example, and the router refuses to go to an href with one.
const controlCharacter = /\p{Cc}/u;

// The page to go to after a sign-in: the redirect search parameter of /signin when it's a path of
// this app, else the start page. Anything else, such as //evil.example or /\evil.example (URLs of
// another host that start with a slash, as a browser reads them) or https://evil.example, would make
// the sign-in page an open redirect. The sign-in page itself leads to the start page, so that it
// doesn't loop.
export function safeRedirectTarget(redirect: string | undefined): string {
  if (redirect === undefined || !redirect.startsWith("/") || controlCharacter.test(redirect)) {
    return "/";
  }
  const { origin } = window.location;
  const url = new URL(redirect, origin);
  return url.origin !== origin || url.pathname === signInPath ? "/" : redirect;
}
