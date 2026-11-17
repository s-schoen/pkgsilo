package middleware

import (
	"net/http"

	"pkgsilo/core/envelope"
)

// CrossOriginRequestType is the problem of a state-changing request that a browser sent from another origin,
// which CrossOriginProtection rejects.
var CrossOriginRequestType = envelope.DefineProblemType("cross-origin-request", "Cross-origin request",
	http.StatusForbidden)

// CrossOriginProtection guards against cross-site request forgery with Go's http.CrossOriginProtection. It
// answers a POST, PUT, PATCH or DELETE that a browser sent from another origin with a CrossOriginRequestType
// problem:
//   - one whose Sec-Fetch-Site header is neither same-origin nor none,
//   - or, without that header, one whose Origin header doesn't match its Host.
//
// GET, HEAD and OPTIONS always pass, and so do requests without either header, such as those of curl, SDKs and
// MCP clients. Behind a proxy, Origin matches only if the proxy passes the Host header on unchanged.
func CrossOriginProtection(next http.Handler) http.Handler {
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		envelope.WriteProblem(w, CrossOriginRequestType.New(envelope.Occurrence{
			Detail: "The API accepts state-changing requests from browsers only from its own origin.",
		}))
	}))

	return protection.Handler(next)
}
