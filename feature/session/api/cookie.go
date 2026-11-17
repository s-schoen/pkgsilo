package api

import "net/http"

// cookieName is the name of the session cookie. The __Host- prefix makes browsers accept it only with Secure,
// with the path / and without a domain, so that no subdomain or plain-HTTP page can set or overwrite it.
const cookieName = "__Host-session"

// setSessionCookie sets the session cookie to token. It has neither Max-Age nor Expires, so that it ends when
// the browser closes, and the session ends on the server long before that if it's idle. Browsers treat
// http://localhost as secure, so the cookie works there too.
func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
}

// expireSessionCookie makes the browser delete the session cookie, with Max-Age=0.
func expireSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
}
