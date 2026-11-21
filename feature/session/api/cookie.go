package api

import (
	"net/http"
	"time"
	"uuid"

	"pkgsilo/core/authn"
)

// cookieName is the name of the session cookie. The __Host- prefix makes browsers accept it only with Secure,
// with the path / and without a domain, so that no subdomain or plain-HTTP page can set or overwrite it.
const cookieName = "__Host-session"

// setSessionCookie sets the session cookie to token, until expiresAt, when its session ends at the latest, so
// that a login outlasts a restart of the browser. The session ends on the server earlier if it's idle, and the
// next request with the cookie then answers 401. Expires is absolute rather than a Max-Age, so that it ends
// with the session to the second. Browsers treat http://localhost as secure, so the cookie works there too.
func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", Expires: expiresAt, Secure: true, HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// expireSessionCookie makes the browser delete the session cookie, with Max-Age=0.
func expireSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
}

// ExpireCookieOf makes the browser delete the session cookie of a request that's authenticated with a session
// of the identity, after every session of the identity has ended, such as when its password changed, so that
// the browser doesn't keep a cookie that no longer works. It changes nothing for a request that's
// authenticated otherwise, such as with an API key, or as another identity. Call it before the response is
// written; the request must have passed authn.Require.
func ExpireCookieOf(w http.ResponseWriter, r *http.Request, identityID uuid.UUID) {
	principal := authn.PrincipalFrom(r.Context())
	if principal.Method == method && principal.IdentityID == identityID {
		expireSessionCookie(w)
	}
}
