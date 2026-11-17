package api

import (
	"net/http"

	"pkgsilo/core/envelope"
	"pkgsilo/core/openapi"
	"pkgsilo/core/request"
	identityapi "pkgsilo/feature/identity/api"
)

// SecurityScheme is the authentication with the API keys of Authenticator, for the OpenAPI document.
func SecurityScheme() openapi.SecurityScheme {
	return openapi.SecurityScheme{
		Name: "apiKey",
		// The header as clients write it, not in the canonical form of keyHeader. Its name is case-insensitive.
		Header:      "X-API-Key",
		Description: "An API key of an identity, which the request acts as.",
	}
}

// Docs describes the API key endpoints of Routes in the OpenAPI document. The routes are mounted at a path
// with the identityID parameter.
func Docs() openapi.Section {
	identityID := openapi.PathID("identityID", "The ID of the identity that the API keys belong to.")
	id := openapi.PathID("id", "The ID of the API key.")

	return openapi.Section{
		Name: "API keys",
		Description: "An API key authenticates requests as the identity that it belongs to, in the X-API-Key " +
			"header. Its key is returned only once, when it's created. Fields that the policies don't allow the " +
			"caller to read are absent from a response.",
		Operations: []openapi.Operation{
			{
				Method: http.MethodGet, Pattern: "/", ID: "listAPIKeys", Summary: "List API keys",
				Description: "Lists the API keys of an identity that the caller may read, oldest first. The others " +
					"are left out, as if they didn't exist.",
				Parameters: []openapi.Parameter{identityID},
				Responses:  []openapi.Response{openapi.Collection[KeyResponse]("A page of API keys.")},
				Problems:   []envelope.ProblemType{identityapi.IdentityNotFoundType},
			},
			{
				Method: http.MethodPost, Pattern: "/", ID: "createAPIKey", Summary: "Create an API key",
				Description: "Creates an API key for an identity. The response holds its key, which no other response " +
					"does, and isn't cached.",
				Parameters: []openapi.Parameter{identityID},
				Body:       openapi.JSONBody[CreateKeyRequest]("The new API key."),
				Responses:  []openapi.Response{openapi.Created[CreatedKeyResponse]("The new API key with its key.")},
				Problems: []envelope.ProblemType{
					envelope.ForbiddenProblem, request.FieldsDeniedType, identityapi.IdentityNotFoundType,
				},
			},
			{
				Method: http.MethodGet, Pattern: "/{id}", ID: "getAPIKey", Summary: "Get an API key",
				Description: "Returns an API key, without its key. One that the caller may not read is answered like " +
					"a missing one.",
				Parameters: []openapi.Parameter{identityID, id},
				Responses:  []openapi.Response{openapi.Resource[KeyResponse]("The API key.")},
				Problems:   []envelope.ProblemType{identityapi.IdentityNotFoundType, apiKeyNotFoundType},
			},
			{
				Method: http.MethodDelete, Pattern: "/{id}", ID: "deleteAPIKey", Summary: "Delete an API key",
				Description: "Deletes an API key, which no longer authenticates requests from then on.",
				Parameters:  []openapi.Parameter{identityID, id},
				Responses:   []openapi.Response{openapi.NoContent("The API key is deleted.")},
				Problems: []envelope.ProblemType{
					envelope.ForbiddenProblem, identityapi.IdentityNotFoundType, apiKeyNotFoundType,
				},
			},
		},
	}
}
