package api

import (
	"net/http"

	"pkgsilo/core/envelope"
	"pkgsilo/core/openapi"
	"pkgsilo/core/request"
	identityapi "pkgsilo/feature/identity/api"
)

// Docs describes the permission endpoint of Routes in the OpenAPI document. The routes are mounted at a path
// with the identityID parameter.
func Docs() openapi.Section {
	return openapi.Section{
		Name: "Permissions",
		Description: "What the policies allow the caller to do, for a client that has no object in hand, such as " +
			"a web UI that hides what would be denied. The answers only guide a client: every request is still " +
			"checked on its own, and an object's allowedActions tell exactly what the caller may do to it.",
		Operations: []openapi.Operation{
			{
				Method: http.MethodGet, Pattern: "/", ID: "listPermissions", Summary: "List the caller's permissions",
				Description: "Lists the actions that the policies may allow the caller to do. Without owner, it lists " +
					"every action that's allowed on some resource of its type: exactly for the creates of identities, " +
					"groups and policies, and erring only toward listing it for every other action. With owner, it " +
					"lists exactly the actions on the resources that the identity owns: creating its API keys, and " +
					"every action on its password. Only the caller's own permissions can be listed: another " +
					"identity is answered with forbidden, or with identity-not-found if the caller may not read it. " +
					"An owner that the caller may not read is answered with identity-not-found.",
				Parameters: []openapi.Parameter{
					openapi.PathIdentityID("identityID", "The ID of the caller's identity."),
					openapi.QueryIdentityID(ownerParam, "The identity that owns the resources to list the actions on."),
				},
				Responses: []openapi.Response{openapi.Resource[PermissionsResponse]("The caller's permissions.")},
				Problems: []envelope.ProblemType{
					envelope.ForbiddenProblem, identityapi.IdentityNotFoundType, request.InvalidQueryType,
				},
			},
		},
	}
}
