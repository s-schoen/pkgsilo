package service

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"pkgsilo/core/authz"
	"pkgsilo/core/pagination"
	"pkgsilo/feature/group/repository"
	identityservice "pkgsilo/feature/identity/service"
)

// GroupService manages groups and their members. Changes are attributed to the given actor.
//
// A group that the actor may not read is reported like a missing one by every method, its members' included.
// One that it may read, but not do the method's action to, is reported with authz.ErrDenied. Every group that
// it returns holds only the fields that the actor may read, and the ID.
type GroupService interface {
	// Create stores a new group with the given input. It returns an *authz.FieldsDeniedError, without
	// storing anything, if the actor may not set any of its fields. A new group that the actor may not
	// read is returned with only its ID.
	Create(ctx context.Context, actor uuid.UUID, in GroupInput) (Group, error)
	// Get returns the group with the given ID.
	Get(ctx context.Context, actor, id uuid.UUID) (Group, error)
	// List returns the page of up to limit groups that skips the first offset, out of those that the
	// actor may read, ordered by creation time. Only the last page can be short.
	List(ctx context.Context, actor uuid.UUID, offset, limit int) (pagination.Page[Group], error)
	// Replace overwrites all caller-set fields of the group with the given ID. It returns an
	// *authz.FieldsDeniedError, without writing anything, if the actor may not set any of the fields, even
	// to their current values.
	Replace(ctx context.Context, actor, id uuid.UUID, in GroupInput) (Group, error)
	// Patch overwrites the fields that the patch sets of the group with the given ID, and keeps the others,
	// even if they change concurrently. It returns an *authz.FieldsDeniedError, without writing anything, if
	// the actor may not set any of the fields that the patch sets. A patch that sets no field writes
	// nothing, so that the group is returned as it is.
	Patch(ctx context.Context, actor, id uuid.UUID, patch GroupPatch) (Group, error)
	// Delete removes the group with the given ID and its memberships.
	Delete(ctx context.Context, actor, id uuid.UUID) error
	// AddMember makes the identity a member of the group. Adding a member again changes nothing. An
	// identity that the actor may not read is reported like a missing one.
	AddMember(ctx context.Context, actor, groupID, identityID uuid.UUID) error
	// ListMembers returns the page of up to limit members of the group that skips the first offset,
	// ordered by when they were added. Reading the members of a group covers all of them, so it includes
	// identities that the actor may not read.
	ListMembers(ctx context.Context, actor, groupID uuid.UUID, offset, limit int) (pagination.Page[Member], error)
	// RemoveMember removes the identity from the members of the group.
	RemoveMember(ctx context.Context, actor, groupID, identityID uuid.UUID) error
}

type groupService struct {
	repo       repository.GroupRepository
	authorizer authz.Authorizer
	now        func() time.Time
}

// NewGroupService returns a GroupService that stores groups and their members in repo and asks authorizer
// what an actor may do.
func NewGroupService(repo repository.GroupRepository, authorizer authz.Authorizer) GroupService {
	return newGroupService(repo, authorizer, time.Now)
}

func newGroupService(repo repository.GroupRepository, authorizer authz.Authorizer, now func() time.Time) *groupService {
	return &groupService{repo: repo, authorizer: authorizer, now: now}
}

func (s *groupService) Create(ctx context.Context, actor uuid.UUID, in GroupInput) (Group, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Group{}, fmt.Errorf("create group: %w", err)
	}

	id := uuid.NewV4()
	if err := createFields.Check(decider, toResource(id)); err != nil {
		return Group{}, fmt.Errorf("create group: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded.
	fields, err := readFields.ReadableIf(decider, actionRead, toResource(id))
	if err != nil {
		return Group{}, fmt.Errorf("create group: %w", err)
	}

	now := s.now()
	rec, err := s.repo.Insert(ctx, repository.GroupRecord{
		ID:        id,
		Name:      in.Name,
		CreatedBy: actor,
		UpdatedBy: actor,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return Group{}, fmt.Errorf("create group: %w", translateError(err))
	}

	return toGroup(rec, fields), nil
}

func (s *groupService) Get(ctx context.Context, actor, id uuid.UUID) (Group, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Group{}, fmt.Errorf("get group: %w", err)
	}

	rec, err := s.getReadable(ctx, decider, id)
	if err != nil {
		return Group{}, fmt.Errorf("get group: %w", err)
	}

	fields, err := readFields.Readable(decider, toResource(id))
	if err != nil {
		return Group{}, fmt.Errorf("get group: %w", err)
	}

	return toGroup(rec, fields), nil
}

func (s *groupService) List(ctx context.Context, actor uuid.UUID, offset, limit int) (pagination.Page[Group], error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return pagination.Page[Group]{}, fmt.Errorf("list groups: %w", err)
	}

	recs, err := s.repo.ListAll(ctx)
	if err != nil {
		return pagination.Page[Group]{}, fmt.Errorf("list groups: %w", err)
	}

	readable, err := authz.Filter(ctx, decider, actionRead, recs, func(rec repository.GroupRecord) authz.Resource {
		return toResource(rec.ID)
	})
	if err != nil {
		return pagination.Page[Group]{}, fmt.Errorf("list groups: %w", err)
	}

	// Only the fields of the rows on the page are checked.
	page := pagination.FromAll(readable, offset, limit)
	groups := make([]Group, len(page.Items))
	for i, rec := range page.Items {
		fields, err := readFields.Readable(decider, toResource(rec.ID))
		if err != nil {
			return pagination.Page[Group]{}, fmt.Errorf("list groups: %w", err)
		}
		groups[i] = toGroup(rec, fields)
	}

	return pagination.Page[Group]{Items: groups, HasMore: page.HasMore}, nil
}

func (s *groupService) Replace(ctx context.Context, actor, id uuid.UUID, in GroupInput) (Group, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Group{}, fmt.Errorf("replace group: %w", err)
	}

	if _, err := s.getReadable(ctx, decider, id); err != nil {
		return Group{}, fmt.Errorf("replace group: %w", err)
	}

	// Replacing sets every field, so each one is checked, even if its value doesn't change.
	if err := updateFields.Check(decider, toResource(id)); err != nil {
		return Group{}, fmt.Errorf("replace group: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded.
	fields, err := readFields.Readable(decider, toResource(id))
	if err != nil {
		return Group{}, fmt.Errorf("replace group: %w", err)
	}

	rec, err := s.repo.Update(ctx, repository.GroupRecord{
		ID:        id,
		Name:      in.Name,
		UpdatedBy: actor,
		UpdatedAt: s.now(),
	})
	if err != nil {
		return Group{}, fmt.Errorf("replace group: %w", translateError(err))
	}

	return toGroup(rec, fields), nil
}

func (s *groupService) Patch(ctx context.Context, actor, id uuid.UUID, patch GroupPatch) (Group, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Group{}, fmt.Errorf("patch group: %w", err)
	}

	rec, err := s.getReadable(ctx, decider, id)
	if err != nil {
		return Group{}, fmt.Errorf("patch group: %w", err)
	}

	// Patching sets only the fields in the patch, so only those are checked.
	if err := updateFields.Only(patch.fields()...).Check(decider, toResource(id)); err != nil {
		return Group{}, fmt.Errorf("patch group: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded.
	fields, err := readFields.Readable(decider, toResource(id))
	if err != nil {
		return Group{}, fmt.Errorf("patch group: %w", err)
	}

	if len(patch.fields()) == 0 {
		return toGroup(rec, fields), nil
	}

	rec, err = s.repo.Patch(ctx, repository.GroupPatch{
		ID:        id,
		Name:      patch.Name,
		UpdatedBy: actor,
		UpdatedAt: s.now(),
	})
	if err != nil {
		return Group{}, fmt.Errorf("patch group: %w", translateError(err))
	}

	return toGroup(rec, fields), nil
}

func (s *groupService) Delete(ctx context.Context, actor, id uuid.UUID) error {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	if err := s.checkAction(ctx, decider, id, actionDelete); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete group: %w", translateError(err))
	}

	return nil
}

func (s *groupService) AddMember(ctx context.Context, actor, groupID, identityID uuid.UUID) error {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return fmt.Errorf("add group member: %w", err)
	}

	// The action on the group comes before the identity, so that an actor who may not add members learns
	// nothing about the identity.
	if err := s.checkAction(ctx, decider, groupID, actionAddMember); err != nil {
		return fmt.Errorf("add group member: %w", err)
	}

	if err := s.checkIdentityReadable(ctx, decider, identityID); err != nil {
		return fmt.Errorf("add group member: %w", err)
	}

	err = s.repo.InsertMember(ctx, repository.MemberRecord{
		GroupID:    groupID,
		IdentityID: identityID,
		CreatedBy:  actor,
		CreatedAt:  s.now(),
	})
	if err != nil {
		return fmt.Errorf("add group member: %w", translateError(err))
	}

	return nil
}

func (s *groupService) ListMembers(
	ctx context.Context, actor, groupID uuid.UUID, offset, limit int,
) (pagination.Page[Member], error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return pagination.Page[Member]{}, fmt.Errorf("list group members: %w", err)
	}

	if err := s.checkAction(ctx, decider, groupID, actionReadMembers); err != nil {
		return pagination.Page[Member]{}, fmt.Errorf("list group members: %w", err)
	}

	// One record more than requested reveals whether further members follow. The members have no rules
	// of their own, so the page can be cut out in the database.
	recs, err := s.repo.ListMembers(ctx, groupID, offset, limit+1)
	if err != nil {
		return pagination.Page[Member]{}, fmt.Errorf("list group members: %w", err)
	}

	members := make([]Member, len(recs))
	for i, rec := range recs {
		members[i] = toMember(rec)
	}

	return pagination.FromLookahead(members, limit), nil
}

func (s *groupService) RemoveMember(ctx context.Context, actor, groupID, identityID uuid.UUID) error {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}

	if err := s.checkAction(ctx, decider, groupID, actionRemoveMember); err != nil {
		return fmt.Errorf("remove group member: %w", err)
	}

	if err := s.repo.DeleteMember(ctx, groupID, identityID); err != nil {
		return fmt.Errorf("remove group member: %w", translateError(err))
	}

	return nil
}

// getReadable returns the record of the group with the given ID. It reports a group that the decider
// doesn't allow to read like a missing one, so that every endpoint hides it before checking anything else.
func (s *groupService) getReadable(
	ctx context.Context, decider authz.Decider, id uuid.UUID,
) (repository.GroupRecord, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return repository.GroupRecord{}, translateError(err)
	}

	can, err := decider.Can(actionRead, toResource(id))
	if err != nil {
		return repository.GroupRecord{}, fmt.Errorf("check read: %w", err)
	}
	if !can {
		return repository.GroupRecord{}, ErrGroupNotFound
	}

	return rec, nil
}

// checkAction checks that the decider allows the action on the group with the given ID, after reading it:
// a group that it doesn't allow to read is reported like a missing one, and a denied action with
// authz.ErrDenied.
func (s *groupService) checkAction(
	ctx context.Context, decider authz.Decider, id uuid.UUID, action authz.Action,
) error {
	if _, err := s.getReadable(ctx, decider, id); err != nil {
		return err
	}

	can, err := decider.Can(action, toResource(id))
	if err != nil {
		return fmt.Errorf("check action: %w", err)
	}
	if !can {
		return authz.ErrDenied
	}

	return nil
}

// checkIdentityReadable checks that the decider allows reading the identity with the given ID, with its
// groups, and reports one that it doesn't allow like a missing one. Whether the identity exists is left to
// the write that refers to it, which reports a missing identity in the same way.
func (s *groupService) checkIdentityReadable(ctx context.Context, decider authz.Decider, id uuid.UUID) error {
	groups, err := s.repo.ListIdentityGroups(ctx, id)
	if err != nil {
		return fmt.Errorf("load identity groups: %w", err)
	}

	can, err := decider.Can(identityservice.ReadAction(),
		authz.Resource{Type: authz.IdentityType, ID: id, Groups: groups})
	if err != nil {
		return fmt.Errorf("check identity read: %w", err)
	}
	if !can {
		return ErrIdentityNotFound
	}

	return nil
}

// toResource returns the group with the given ID as a resource of a decision. A group is a member of no
// group.
func toResource(id uuid.UUID) authz.Resource {
	return authz.Resource{Type: authz.GroupType, ID: id}
}

// toGroup returns the group of the record with its ID and the readable fields. Every other field stays
// hidden, so that one that's missing here can't be revealed by mistake.
func toGroup(rec repository.GroupRecord, fields authz.ReadableFields) Group {
	return Group{
		ID:        rec.ID,
		Name:      authz.Reveal(fields, "name", rec.Name),
		CreatedBy: authz.Reveal(fields, "createdBy", rec.CreatedBy),
		UpdatedBy: authz.Reveal(fields, "updatedBy", rec.UpdatedBy),
		CreatedAt: authz.Reveal(fields, "createdAt", rec.CreatedAt),
		UpdatedAt: authz.Reveal(fields, "updatedAt", rec.UpdatedAt),
	}
}

func toMember(rec repository.MemberRecord) Member {
	return Member{
		IdentityID: rec.IdentityID,
		CreatedBy:  rec.CreatedBy,
		CreatedAt:  rec.CreatedAt,
	}
}
