package api

import (
	"net/http"

	"pkgsilo/core/envelope"
	"pkgsilo/core/openapi"
	"pkgsilo/core/request"
	identityapi "pkgsilo/feature/identity/api"
)

// Docs describes the group endpoints of Routes in the OpenAPI document.
func Docs() openapi.Section {
	id := openapi.PathID("id", "The ID of the group.")
	identityID := openapi.PathID("identityID", "The ID of the identity, the member.")
	// writeProblems are the problems of a request that changes a group.
	writeProblems := []envelope.ProblemType{
		envelope.ForbiddenProblem, request.FieldsDeniedType, groupNotFoundType, groupNameTakenType,
	}

	return openapi.Section{
		Name: "Groups",
		Description: "A group is a named set of identities, its members, which policies can apply to all at once. " +
			"Fields that the policies don't allow the caller to read are absent from a response.",
		Operations: []openapi.Operation{
			{
				Method: http.MethodGet, Pattern: "/", ID: "listGroups", Summary: "List groups",
				Description: "Lists the groups that the caller may read, oldest first. The others are left out, as " +
					"if they didn't exist.",
				Responses: []openapi.Response{openapi.Collection[GroupResponse]("A page of groups.")},
			},
			{
				Method: http.MethodPost, Pattern: "/", ID: "createGroup", Summary: "Create a group",
				Description: "Creates a group without members.",
				Body:        openapi.JSONBody[CreateGroupRequest]("The new group."),
				Responses:   []openapi.Response{openapi.Created[GroupResponse]("The new group.")},
				Problems:    []envelope.ProblemType{envelope.ForbiddenProblem, request.FieldsDeniedType, groupNameTakenType},
			},
			{
				Method: http.MethodGet, Pattern: "/{id}", ID: "getGroup", Summary: "Get a group",
				Description: "Returns a group. One that the caller may not read is answered like a missing one.",
				Parameters:  []openapi.Parameter{id},
				Responses:   []openapi.Response{openapi.Resource[GroupResponse]("The group.")},
				Problems:    []envelope.ProblemType{groupNotFoundType},
			},
			{
				Method: http.MethodPut, Pattern: "/{id}", ID: "replaceGroup", Summary: "Replace a group",
				Description: "Sets every settable field of a group.",
				Parameters:  []openapi.Parameter{id},
				Body:        openapi.JSONBody[ReplaceGroupRequest]("The fields of the group."),
				Responses:   []openapi.Response{openapi.Resource[GroupResponse]("The changed group.")},
				Problems:    writeProblems,
			},
			{
				Method: http.MethodPatch, Pattern: "/{id}", ID: "patchGroup", Summary: "Change a group",
				Description: "Changes the fields of a group that the JSON Merge Patch sets, and keeps the others.",
				Parameters:  []openapi.Parameter{id},
				Body:        openapi.MergePatchBody[PatchGroupRequest]("The fields to change."),
				Responses:   []openapi.Response{openapi.Resource[GroupResponse]("The changed group.")},
				Problems:    writeProblems,
			},
			{
				Method: http.MethodDelete, Pattern: "/{id}", ID: "deleteGroup", Summary: "Delete a group",
				Description: "Deletes a group with its memberships. Its members are kept.",
				Parameters:  []openapi.Parameter{id},
				Responses:   []openapi.Response{openapi.NoContent("The group is deleted.")},
				Problems:    []envelope.ProblemType{envelope.ForbiddenProblem, groupNotFoundType},
			},
			{
				Method: http.MethodGet, Pattern: "/{id}/members", ID: "listGroupMembers", Summary: "List group members",
				Description: "Lists the members of a group, in the order they were added.",
				Parameters:  []openapi.Parameter{id},
				Responses:   []openapi.Response{openapi.Collection[MemberResponse]("A page of members.")},
				Problems:    []envelope.ProblemType{envelope.ForbiddenProblem, groupNotFoundType},
			},
			{
				Method: http.MethodPut, Pattern: "/{id}/members/{identityID}", ID: "addGroupMember",
				Summary:     "Add a group member",
				Description: "Adds an identity to a group. The request has no body.",
				Parameters:  []openapi.Parameter{id, identityID},
				Responses:   []openapi.Response{openapi.NoContent("The identity is a member of the group.")},
				Problems: []envelope.ProblemType{
					envelope.BadRequestProblem, envelope.ForbiddenProblem, groupNotFoundType,
					identityapi.IdentityNotFoundType,
				},
			},
			{
				Method: http.MethodDelete, Pattern: "/{id}/members/{identityID}", ID: "removeGroupMember",
				Summary:     "Remove a group member",
				Description: "Removes an identity from a group.",
				Parameters:  []openapi.Parameter{id, identityID},
				Responses:   []openapi.Response{openapi.NoContent("The identity is no member of the group.")},
				Problems:    []envelope.ProblemType{envelope.ForbiddenProblem, groupNotFoundType, memberNotFoundType},
			},
		},
	}
}
