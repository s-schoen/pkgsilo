package api

import (
	"net/http"

	"pkgsilo/core/envelope"
	"pkgsilo/core/openapi"
	identityapi "pkgsilo/feature/identity/api"
)

// sessionsEnd says what setting or deleting a password does to the identity's sessions.
const sessionsEnd = "Every session of the identity ends, so that it's logged out of every browser. If the caller " +
	"is that identity and sent its session cookie, the response deletes the cookie. Its API keys keep working."

// Docs describes the password endpoints of Routes in the OpenAPI document. The routes are mounted at a path
// with the identityID parameter.
func Docs() openapi.Section {
	identityID := openapi.PathID("identityID", "The ID of the identity that the password belongs to.")

	return openapi.Section{
		Name: "Passwords",
		Description: "An identity can have one password, besides its API keys. No response holds a password or " +
			"its hash. Fields that the policies don't allow the caller to read are absent from a response.",
		Operations: []openapi.Operation{
			{
				Method: http.MethodGet, Pattern: "/", ID: "getPassword", Summary: "Get a password",
				Description: "Returns when an identity's password was set. An identity without a password, or one " +
					"whose password the caller may not read, is answered with password-not-found.",
				Parameters: []openapi.Parameter{identityID},
				Responses:  []openapi.Response{openapi.Resource[PasswordResponse]("The password.")},
				Problems:   []envelope.ProblemType{identityapi.IdentityNotFoundType, passwordNotFoundType},
			},
			{
				Method: http.MethodPut, Pattern: "/", ID: "setPassword", Summary: "Set a password",
				Description: "Sets an identity's password. A caller that the policies allow to reset it needs no " +
					"current password, which is the only way to set a first password. A caller that may only " +
					"change it must send the current password in currentPassword, or gets an invalid-body " +
					"problem on that member. Changing a password that doesn't exist answers password-not-found. " +
					sessionsEnd,
				Parameters: []openapi.Parameter{identityID},
				Body:       openapi.JSONBody[SetPasswordRequest]("The new password."),
				Responses:  []openapi.Response{openapi.Resource[PasswordResponse]("The password as set.")},
				Problems: []envelope.ProblemType{
					envelope.ForbiddenProblem, identityapi.IdentityNotFoundType, passwordNotFoundType,
				},
			},
			{
				Method: http.MethodDelete, Pattern: "/", ID: "deletePassword", Summary: "Delete a password",
				Description: "Deletes an identity's password. The identity can then only use its API keys. " +
					sessionsEnd,
				Parameters: []openapi.Parameter{identityID},
				Responses:  []openapi.Response{openapi.NoContent("The password is deleted.")},
				Problems: []envelope.ProblemType{
					envelope.ForbiddenProblem, identityapi.IdentityNotFoundType, passwordNotFoundType,
				},
			},
		},
	}
}
