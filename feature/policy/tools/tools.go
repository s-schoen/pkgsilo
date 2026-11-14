package tools

import (
	"context"
	"uuid"

	"pkgsilo/core/envelope"
	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/policy/service"
)

// Tools offers the policy tools.
type Tools struct {
	service service.PolicyService
}

var _ mcpserver.Toolset = (*Tools)(nil)

// New returns the policy tools, which read the schema and manage policies through svc.
func New(svc service.PolicyService) *Tools {
	return &Tools{service: svc}
}

// Parts of the tools' descriptions that several tools share.
const (
	// policyTerm says what a policy is, and how its statements decide.
	policyTerm = "A policy is a named document of one or more Cedar statements that decide what identities may " +
		"do in pkgsilo. Each statement permits or forbids actions, such as identity:read, to principals, " +
		"which are identities, such as Identity::\"<identity ID>\" or every member of a group with principal " +
		"in Group::\"<group ID>\", on resources, such as permit (principal in Group::\"<group ID>\", action " +
		"in Action::\"read\", resource);. Everything that no statement permits is denied, and a forbid wins " +
		"over every permit. A change applies to the next request. "
	// checks says how a source is checked before it's saved, and what invalid_policy means.
	checks = "Before it is saved, the source is checked: it must parse, hold at least one statement and at most " +
		"16384 bytes, use only the actions and entity types of the schema that policy_get_schema returns, " +
		"test with has only for attributes that the schema declares, and name only groups and identities that " +
		"exist, by their IDs in lowercase with hyphens; all policies together may hold at most 1000 " +
		"statements. Fails with invalid_policy, saying what to fix, if the source breaks these rules. "
	// masking says what the output holds.
	masking = "Fields that the policies do not allow you to read are absent from the output. createdBy and " +
		"updatedBy are the nil UUID for changes that pkgsilo made itself, such as creating the administrators " +
		"policy. "
	// notFound says what policy_not_found means.
	notFound = "Fails with policy_not_found if the policy does not exist, or if the policies do not allow you " +
		"to read it: the two cases look the same. "
	// fieldsDenied says what fields_denied means for a tool that writes.
	fieldsDenied = "Fails with fields_denied, naming the arguments, if the policies do not allow you to set " +
		"some of them, and with invalid_arguments, naming each invalid argument and why, if they break the " +
		"rules; nothing is changed then."
	// lockout warns that a change can take away the actor's own access.
	lockout = "Changing or deleting a policy that gives you access, such as the administrators policy, can " +
		"take away your own access, which you may then be unable to restore. "
	// paging says how to page through a list.
	paging = "Page through the list by calling it again with offset set to the nextOffset of the previous " +
		"page, until there is no nextOffset. "
)

// Register adds the policy tools to reg.
func (t *Tools) Register(reg *mcpserver.Registry) {
	reg.Add(mcpserver.Tool{
		Name: "policy_get_schema",
		Description: "Returns the Cedar schema that every policy is checked against, in Cedar's human-readable " +
			"schema format: the entity types, such as Identity and Group, and every action, with the action " +
			"groups that it is in and the types of principal and resource that it applies to. Read it before " +
			"writing a policy with policy_create or policy_update, so that the policy uses only declared " +
			"actions and types. An action group, such as read or identity:readFields, matches every action in " +
			"it with action in Action::\"<group>\". The actions on fields, such as identity:read:username, " +
			"decide which fields of an object you can read or set. Any identity may read the schema.",
		Kind: mcpserver.Read,
	}, t.getSchema)
	reg.Add(mcpserver.Tool{
		Name: "policy_list",
		Description: "Lists the policies that the policies allow you to read, oldest first, one page at a time. " +
			policyTerm + "Use it to find a policy, such as by its name, since there is no search. " + paging +
			"Policies that you may not read are left out, as if they did not exist, though they still apply. " +
			masking,
		Kind: mcpserver.Read,
	}, t.list)
	reg.Add(mcpserver.Tool{
		Name:        "policy_get",
		Description: "Returns the policy with the given ID. " + policyTerm + masking + notFound,
		Kind:        mcpserver.Read,
	}, t.get)
	reg.Add(mcpserver.Tool{
		Name: "policy_create",
		Description: "Creates a policy and returns it. " + policyTerm +
			"Call policy_get_schema first to learn the actions and entity types. " + checks +
			"The output holds only its id if the policies do not allow you to read it. " + masking +
			"Fails with policy_name_taken if another policy has the name. " + fieldsDenied,
		Kind: mcpserver.Write,
	}, t.create)
	reg.Add(mcpserver.Tool{
		Name: "policy_update",
		Description: "Changes the name or the source of a policy, or both, and returns it. " + policyTerm +
			"Leave a member out to keep it, and all of them to change nothing. A new source replaces all " +
			"statements of the policy. " + checks + "The source is checked even if only the name changes, " +
			"so a policy whose source names a group that was deleted since can't be renamed before its " +
			"source is fixed. " + lockout + masking + notFound +
			"Fails with policy_name_taken if another policy has the new name. " + fieldsDenied,
		Kind: mcpserver.Write,
	}, t.update)
	reg.Add(mcpserver.Tool{
		Name: "policy_delete",
		Description: "Deletes a policy, so that its statements no longer apply: what it permitted is denied " +
			"unless another policy permits it, and what it forbade is allowed again if another policy permits " +
			"it. " + policyTerm + lockout + "The output is empty. " + notFound +
			"Fails with forbidden if the policies allow you to read the policy, but not to delete it; nothing " +
			"is changed then.",
		Kind: mcpserver.Delete,
	}, t.delete)
}

func (t *Tools) getSchema(context.Context, uuid.UUID, GetSchemaInput) (SchemaOutput, error) {
	return SchemaOutput{Schema: envelope.Visible(t.service.Schema())}, nil
}

func (t *Tools) list(
	ctx context.Context, actor uuid.UUID, in mcpserver.ListInput,
) (mcpserver.ListOutput[PolicyOutput], error) {
	page := in.Pagination()
	policies, err := t.service.List(ctx, actor, page.Offset, page.Limit)
	if err != nil {
		return mcpserver.ListOutput[PolicyOutput]{}, translateError(err)
	}

	return mcpserver.NewListOutput(policies, page.Offset, toPolicyOutput), nil
}

func (t *Tools) get(ctx context.Context, actor uuid.UUID, in GetInput) (PolicyOutput, error) {
	policy, err := t.service.Get(ctx, actor, in.ID)
	if err != nil {
		return PolicyOutput{}, translateError(err)
	}

	return toPolicyOutput(policy), nil
}

func (t *Tools) create(ctx context.Context, actor uuid.UUID, in CreateInput) (PolicyOutput, error) {
	policy, err := t.service.Create(ctx, actor, in.toInput())
	if err != nil {
		return PolicyOutput{}, translateError(err)
	}

	return toPolicyOutput(policy), nil
}

func (t *Tools) update(ctx context.Context, actor uuid.UUID, in UpdateInput) (PolicyOutput, error) {
	policy, err := t.service.Patch(ctx, actor, in.ID, in.toPatch())
	if err != nil {
		return PolicyOutput{}, translateError(err)
	}

	return toPolicyOutput(policy), nil
}

func (t *Tools) delete(ctx context.Context, actor uuid.UUID, in DeleteInput) (mcpserver.DoneOutput, error) {
	if err := t.service.Delete(ctx, actor, in.ID); err != nil {
		return mcpserver.DoneOutput{}, translateError(err)
	}

	return mcpserver.DoneOutput{}, nil
}
