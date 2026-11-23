package api

import (
	"net/http"

	"pkgsilo/core/envelope"
	"pkgsilo/core/openapi"
	"pkgsilo/core/request"
)

// Docs describes the identity endpoints of Routes in the OpenAPI document.
func Docs() openapi.Section {
	id := openapi.PathIdentityID("id", "The ID of the identity.")
	// writeProblems are the problems of a request that changes an identity.
	writeProblems := []envelope.ProblemType{
		envelope.ForbiddenProblem, request.FieldsDeniedType, IdentityNotFoundType, usernameTakenType,
	}

	return openapi.Section{
		Name: "Identities",
		Description: "An identity is a user of pkgsilo, a person or an agent, with a unique username and a display " +
			"name. It authenticates with API keys, and the policies decide what it may do. Fields that the " +
			"policies don't allow the caller to read are absent from a response.",
		Operations: []openapi.Operation{
			{
				Method: http.MethodGet, Pattern: "/", ID: "listIdentities", Summary: "List identities",
				Description: "Lists the identities that the caller may read, oldest first. The others are left out, " +
					"as if they didn't exist.",
				Responses: []openapi.Response{openapi.Collection[IdentityResponse]("A page of identities.")},
			},
			{
				Method: http.MethodPost, Pattern: "/", ID: "createIdentity", Summary: "Create an identity",
				Description: "Creates an identity without API keys or groups.",
				Body:        openapi.JSONBody[CreateIdentityRequest]("The new identity."),
				Responses:   []openapi.Response{openapi.Created[IdentityResponse]("The new identity.")},
				Problems:    []envelope.ProblemType{envelope.ForbiddenProblem, request.FieldsDeniedType, usernameTakenType},
			},
			{
				Method: http.MethodGet, Pattern: "/{id}", ID: "getIdentity", Summary: "Get an identity",
				Description: "Returns an identity. One that the caller may not read is answered like a missing one.",
				Parameters:  []openapi.Parameter{id},
				Responses:   []openapi.Response{openapi.Resource[IdentityResponse]("The identity.")},
				Problems:    []envelope.ProblemType{IdentityNotFoundType},
			},
			{
				Method: http.MethodPut, Pattern: "/{id}", ID: "replaceIdentity", Summary: "Replace an identity",
				Description: "Sets every settable field of an identity.",
				Parameters:  []openapi.Parameter{id},
				Body:        openapi.JSONBody[ReplaceIdentityRequest]("The fields of the identity."),
				Responses:   []openapi.Response{openapi.Resource[IdentityResponse]("The changed identity.")},
				Problems:    writeProblems,
			},
			{
				Method: http.MethodPatch, Pattern: "/{id}", ID: "patchIdentity", Summary: "Change an identity",
				Description: "Changes the fields of an identity that the JSON Merge Patch sets, and keeps the others.",
				Parameters:  []openapi.Parameter{id},
				Body:        openapi.MergePatchBody[PatchIdentityRequest]("The fields to change."),
				Responses:   []openapi.Response{openapi.Resource[IdentityResponse]("The changed identity.")},
				Problems:    writeProblems,
			},
			{
				Method: http.MethodDelete, Pattern: "/{id}", ID: "deleteIdentity", Summary: "Delete an identity",
				Description: "Deletes an identity, with its API keys and group memberships.",
				Parameters:  []openapi.Parameter{id},
				Responses:   []openapi.Response{openapi.NoContent("The identity is deleted.")},
				Problems:    []envelope.ProblemType{envelope.ForbiddenProblem, IdentityNotFoundType},
			},
		},
	}
}
