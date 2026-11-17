package api

import (
	"net/http"

	"pkgsilo/core/envelope"
	"pkgsilo/core/openapi"
	"pkgsilo/core/request"
)

// Docs describes the policy endpoints of Routes in the OpenAPI document.
func Docs() openapi.Section {
	id := openapi.PathID("id", "The ID of the policy.")
	// writeProblems are the problems of a request that changes a policy.
	writeProblems := []envelope.ProblemType{
		envelope.ForbiddenProblem, request.FieldsDeniedType, policyNotFoundType, policyNameTakenType,
		invalidPolicyType,
	}

	return openapi.Section{
		Name: "Policies",
		Description: "A policy holds Cedar statements that permit or forbid identities and groups actions on the " +
			"resources of pkgsilo. Whatever no policy permits is denied. Fields that the policies don't allow the " +
			"caller to read are absent from a response.",
		Operations: []openapi.Operation{
			{
				Method: http.MethodGet, Pattern: "/", ID: "listPolicies", Summary: "List policies",
				Description: "Lists the policies that the caller may read, oldest first. The others are left out, " +
					"as if they didn't exist.",
				Responses: []openapi.Response{openapi.Collection[PolicyResponse]("A page of policies.")},
			},
			{
				Method: http.MethodPost, Pattern: "/", ID: "createPolicy", Summary: "Create a policy",
				Description: "Creates a policy, which applies from then on. Its source must be valid Cedar that " +
					"matches the schema of pkgsilo.",
				Body:      openapi.JSONBody[CreatePolicyRequest]("The new policy."),
				Responses: []openapi.Response{openapi.Created[PolicyResponse]("The new policy.")},
				Problems: []envelope.ProblemType{
					envelope.ForbiddenProblem, request.FieldsDeniedType, policyNameTakenType, invalidPolicyType,
				},
			},
			{
				Method: http.MethodGet, Pattern: "/{id}", ID: "getPolicy", Summary: "Get a policy",
				Description: "Returns a policy. One that the caller may not read is answered like a missing one.",
				Parameters:  []openapi.Parameter{id},
				Responses:   []openapi.Response{openapi.Resource[PolicyResponse]("The policy.")},
				Problems:    []envelope.ProblemType{policyNotFoundType},
			},
			{
				Method: http.MethodPut, Pattern: "/{id}", ID: "replacePolicy", Summary: "Replace a policy",
				Description: "Sets every settable field of a policy.",
				Parameters:  []openapi.Parameter{id},
				Body:        openapi.JSONBody[ReplacePolicyRequest]("The fields of the policy."),
				Responses:   []openapi.Response{openapi.Resource[PolicyResponse]("The changed policy.")},
				Problems:    writeProblems,
			},
			{
				Method: http.MethodPatch, Pattern: "/{id}", ID: "patchPolicy", Summary: "Change a policy",
				Description: "Changes the fields of a policy that the JSON Merge Patch sets, and keeps the others.",
				Parameters:  []openapi.Parameter{id},
				Body:        openapi.MergePatchBody[PatchPolicyRequest]("The fields to change."),
				Responses:   []openapi.Response{openapi.Resource[PolicyResponse]("The changed policy.")},
				Problems:    writeProblems,
			},
			{
				Method: http.MethodDelete, Pattern: "/{id}", ID: "deletePolicy", Summary: "Delete a policy",
				Description: "Deletes a policy, which no longer applies from then on.",
				Parameters:  []openapi.Parameter{id},
				Responses:   []openapi.Response{openapi.NoContent("The policy is deleted.")},
				Problems:    []envelope.ProblemType{envelope.ForbiddenProblem, policyNotFoundType},
			},
		},
	}
}
