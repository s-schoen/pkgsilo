package api

import (
	"net/http"

	"pkgsilo/core/envelope"
	"pkgsilo/core/openapi"
)

// SecurityScheme is the authentication with the session cookie of Authenticator, for the OpenAPI document.
func SecurityScheme() openapi.SecurityScheme {
	return openapi.SecurityScheme{
		Name:   "session",
		Cookie: cookieName,
		Description: "The session cookie that a login sets, for browsers on the same origin as the API. It " +
			"authenticates the routes under /api/v1 only, not the MCP endpoint. An API key wins over it when a " +
			"request carries both.",
	}
}

// Docs describes the session endpoints of Routes in the OpenAPI document.
func Docs() openapi.Section {
	return openapi.Section{
		Name: "Sessions",
		Description: "A session keeps a browser logged in as an identity, in an HttpOnly cookie that a login with a " +
			"password sets. It ends after an hour without a request or a day after the login, as the operator " +
			"configures, or when it's deleted. No policy applies to logging in, nor to reading or ending your " +
			"own session, so that no policy can block a logout.",
		Operations: []openapi.Operation{
			{
				Method: http.MethodPost, Pattern: "/", ID: "createSession", Summary: "Log in", Public: true,
				Description: "Logs in with a username and password, and sets the session cookie " + cookieName +
					" (Secure, HttpOnly, SameSite=Strict), which expires when the session ends at the latest, so " +
					"that a login outlasts a restart of the browser. The response never holds the " +
					"session's token. A session cookie that the request already carries ends. A wrong password, an " +
					"unknown username and an identity without a password all get the same 401.",
				Body:      openapi.JSONBody[LoginRequest]("The credentials."),
				Responses: []openapi.Response{openapi.Created[SessionResponse]("The new session.")},
				Problems:  []envelope.ProblemType{envelope.UnauthorizedProblem},
			},
			{
				Method: http.MethodGet, Pattern: currentPattern, ID: "getCurrentSession", Summary: "Get the current session",
				Description: "Returns the session that the request is authenticated with, which tells a browser who " +
					"it's logged in as. A request with an API key has no session.",
				Responses: []openapi.Response{openapi.Resource[SessionResponse]("The current session.")},
				Problems:  []envelope.ProblemType{sessionNotFoundType},
			},
			{
				Method: http.MethodDelete, Pattern: currentPattern, ID: "deleteCurrentSession", Summary: "Log out",
				Description: "Ends the session that the request is authenticated with and deletes its cookie. A " +
					"request with an API key has no session.",
				Responses: []openapi.Response{openapi.NoContent("The session has ended.")},
				Problems:  []envelope.ProblemType{sessionNotFoundType},
			},
		},
	}
}
