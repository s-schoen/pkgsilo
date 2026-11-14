package tools

import (
	"context"
	"uuid"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/group/service"
)

// Tools offers the group tools.
type Tools struct {
	service service.GroupService
}

var _ mcpserver.Toolset = (*Tools)(nil)

// New returns the group tools, which manage groups and their members through svc.
func New(svc service.GroupService) *Tools {
	return &Tools{service: svc}
}

// Parts of the tools' descriptions that several tools share.
const (
	// groupTerm says what a group is, and that membership is how policies give access to identities.
	groupTerm = "A group is a named set of identities, its members, with a unique name. A policy can permit " +
		"or forbid actions to every member of a group at once, with principal in Group::\"<group ID>\", so " +
		"an identity gets what the policies give to the groups that it is a member of. "
	// masking says what the output holds.
	masking = "Fields that the policies do not allow you to read are absent from the output. createdBy and " +
		"updatedBy are the nil UUID for changes that pkgsilo made itself, such as creating the " +
		"administrators group. "
	// notFound says what group_not_found means.
	notFound = "Fails with group_not_found if the group does not exist, or if the policies do not allow you " +
		"to read it: the two cases look the same. "
)

// Register adds the group tools to reg.
func (t *Tools) Register(reg *mcpserver.Registry) {
	reg.Add(mcpserver.Tool{
		Name: "group_list",
		Description: "Lists the groups that the policies allow you to read, oldest first, one page at a time. " +
			groupTerm + "Use it to find a group, such as by its name, since there is no search. " +
			mcpserver.PagingDescription + "Groups that you may not read are left out, as if they did not exist. " +
			masking,
		Kind: mcpserver.Read,
	}, t.list)
	reg.Add(mcpserver.Tool{
		Name:        "group_get",
		Description: "Returns the group with the given ID. " + groupTerm + masking + notFound,
		Kind:        mcpserver.Read,
	}, t.get)
	reg.Add(mcpserver.Tool{
		Name: "group_create",
		Description: "Creates a group without members and returns it. " + groupTerm +
			"Add members with group_add_member. The output holds only its id if the policies do not allow you " +
			"to read it. " + masking + "Fails with group_name_taken if another group has the name. " +
			mcpserver.WriteErrorsDescription,
		Kind:     mcpserver.Write,
		Additive: true,
	}, t.create)
	reg.Add(mcpserver.Tool{
		Name: "group_update",
		Description: "Renames a group and returns it. " + groupTerm +
			"Leave name out to change nothing. Its members stay as they are: change them with " +
			"group_add_member and group_remove_member. " + masking + notFound +
			"Fails with group_name_taken if another group has the new name. " + mcpserver.WriteErrorsDescription,
		Kind:       mcpserver.Write,
		Idempotent: true,
	}, t.update)
	t.registerMembers(reg)
}

// registerMembers adds the tools that list and change the members of a group to reg.
func (t *Tools) registerMembers(reg *mcpserver.Registry) {
	reg.Add(mcpserver.Tool{
		Name: "group_list_members",
		Description: "Lists the members of a group, in the order they were added, one page at a time. " +
			groupTerm + "Each member is the ID of an identity, with who added it and when: get the identity " +
			"with identity_get. The list holds every member, even identities that the policies do not allow " +
			"you to read. " + mcpserver.PagingDescription + notFound +
			"Fails with forbidden if the policies allow you to read the group, but not its members.",
		Kind: mcpserver.Read,
	}, t.listMembers)
	reg.Add(mcpserver.Tool{
		Name: "group_add_member",
		Description: "Adds an identity to the members of a group, so that the identity gets what the policies " +
			"give to the group, which takes away what they forbid to it. " + groupTerm +
			"Adding a member again changes nothing and succeeds. The output is empty. " + notFound +
			"Fails with identity_not_found if the identity does not exist, or if the policies do not allow you " +
			"to read it, and with forbidden if the policies allow you to read the group, but not to add " +
			"members to it; nothing is changed then.",
		Kind:       mcpserver.Write,
		Idempotent: true,
	}, t.addMember)
	reg.Add(mcpserver.Tool{
		Name: "group_remove_member",
		Description: "Removes an identity from the members of a group, so that the identity loses what the " +
			"policies give to the group, unless they give it in another way. " + groupTerm +
			"Removing your own identity, such as from the administrators group, can take away your own " +
			"access, which you may then be unable to restore. The output is empty. " + notFound +
			"Fails with member_not_found if the identity is not a member of the group, or if the policies do " +
			"not allow you to read it, and with forbidden if the policies allow you to read the group, but not " +
			"to remove members from it; nothing is changed then.",
		Kind: mcpserver.Delete,
	}, t.removeMember)
}

func (t *Tools) list(
	ctx context.Context, actor uuid.UUID, in mcpserver.ListInput,
) (mcpserver.ListOutput[GroupOutput], error) {
	page := in.Pagination()
	groups, err := t.service.List(ctx, actor, page.Offset, page.Limit)
	if err != nil {
		return mcpserver.ListOutput[GroupOutput]{}, translateError(err)
	}

	return mcpserver.NewListOutput(groups, page.Offset, toGroupOutput), nil
}

func (t *Tools) get(ctx context.Context, actor uuid.UUID, in GetInput) (GroupOutput, error) {
	group, err := t.service.Get(ctx, actor, in.ID)
	if err != nil {
		return GroupOutput{}, translateError(err)
	}

	return toGroupOutput(group), nil
}

func (t *Tools) create(ctx context.Context, actor uuid.UUID, in CreateInput) (GroupOutput, error) {
	group, err := t.service.Create(ctx, actor, in.toInput())
	if err != nil {
		return GroupOutput{}, translateError(err)
	}

	return toGroupOutput(group), nil
}

func (t *Tools) update(ctx context.Context, actor uuid.UUID, in UpdateInput) (GroupOutput, error) {
	group, err := t.service.Patch(ctx, actor, in.ID, in.toPatch())
	if err != nil {
		return GroupOutput{}, translateError(err)
	}

	return toGroupOutput(group), nil
}

func (t *Tools) listMembers(
	ctx context.Context, actor uuid.UUID, in ListMembersInput,
) (mcpserver.ListOutput[MemberOutput], error) {
	page := in.Pagination()
	members, err := t.service.ListMembers(ctx, actor, in.GroupID, page.Offset, page.Limit)
	if err != nil {
		return mcpserver.ListOutput[MemberOutput]{}, translateError(err)
	}

	return mcpserver.NewListOutput(members, page.Offset, toMemberOutput), nil
}

func (t *Tools) addMember(ctx context.Context, actor uuid.UUID, in MemberInput) (mcpserver.DoneOutput, error) {
	if err := t.service.AddMember(ctx, actor, in.GroupID, in.IdentityID); err != nil {
		return mcpserver.DoneOutput{}, translateError(err)
	}

	return mcpserver.DoneOutput{}, nil
}

func (t *Tools) removeMember(ctx context.Context, actor uuid.UUID, in MemberInput) (mcpserver.DoneOutput, error) {
	if err := t.service.RemoveMember(ctx, actor, in.GroupID, in.IdentityID); err != nil {
		return mcpserver.DoneOutput{}, translateError(err)
	}

	return mcpserver.DoneOutput{}, nil
}
