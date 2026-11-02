package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"pkgsilo/core/pagination"
	"pkgsilo/group/repository"
)

// GroupService manages groups and their members. Changes are attributed to the given actor.
type GroupService interface {
	// Create stores a new group with the given input.
	Create(ctx context.Context, actor uuid.UUID, in GroupInput) (Group, error)
	// Get returns the group with the given ID.
	Get(ctx context.Context, id uuid.UUID) (Group, error)
	// List returns the page of up to limit groups that skips the first offset, ordered by
	// creation time.
	List(ctx context.Context, offset, limit int) (pagination.Page[Group], error)
	// Replace overwrites all caller-set fields of the group with the given ID.
	Replace(ctx context.Context, actor, id uuid.UUID, in GroupInput) (Group, error)
	// Delete removes the group with the given ID and its memberships.
	Delete(ctx context.Context, id uuid.UUID) error
	// AddMember makes the identity a member of the group. Adding a member again changes nothing. A
	// missing group is reported before a missing identity.
	AddMember(ctx context.Context, actor, groupID, identityID uuid.UUID) error
	// ListMembers returns the page of up to limit members of the group that skips the first
	// offset, ordered by when they were added.
	ListMembers(ctx context.Context, groupID uuid.UUID, offset, limit int) (pagination.Page[Member], error)
	// RemoveMember removes the identity from the members of the group. A missing group is reported
	// before a missing member.
	RemoveMember(ctx context.Context, groupID, identityID uuid.UUID) error
}

type groupService struct {
	repo repository.GroupRepository
	now  func() time.Time
}

// NewGroupService returns a GroupService that stores groups and their members in repo.
func NewGroupService(repo repository.GroupRepository) GroupService {
	return newGroupService(repo, time.Now)
}

func newGroupService(repo repository.GroupRepository, now func() time.Time) *groupService {
	return &groupService{repo: repo, now: now}
}

func (s *groupService) Create(ctx context.Context, actor uuid.UUID, in GroupInput) (Group, error) {
	now := s.now()
	rec, err := s.repo.Insert(ctx, repository.GroupRecord{
		ID:        uuid.NewV4(),
		Name:      in.Name,
		CreatedBy: actor,
		UpdatedBy: actor,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return Group{}, fmt.Errorf("create group: %w", translateError(err))
	}

	return toGroup(rec), nil
}

func (s *groupService) Get(ctx context.Context, id uuid.UUID) (Group, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return Group{}, fmt.Errorf("get group: %w", translateError(err))
	}

	return toGroup(rec), nil
}

func (s *groupService) List(ctx context.Context, offset, limit int) (pagination.Page[Group], error) {
	// One record more than requested reveals whether further groups follow.
	recs, err := s.repo.List(ctx, offset, limit+1)
	if err != nil {
		return pagination.Page[Group]{}, fmt.Errorf("list groups: %w", err)
	}

	groups := make([]Group, len(recs))
	for i, rec := range recs {
		groups[i] = toGroup(rec)
	}

	return pagination.FromLookahead(groups, limit), nil
}

func (s *groupService) Replace(ctx context.Context, actor, id uuid.UUID, in GroupInput) (Group, error) {
	rec, err := s.repo.Update(ctx, repository.GroupRecord{
		ID:        id,
		Name:      in.Name,
		UpdatedBy: actor,
		UpdatedAt: s.now(),
	})
	if err != nil {
		return Group{}, fmt.Errorf("replace group: %w", translateError(err))
	}

	return toGroup(rec), nil
}

func (s *groupService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete group: %w", translateError(err))
	}

	return nil
}

func (s *groupService) AddMember(ctx context.Context, actor, groupID, identityID uuid.UUID) error {
	err := s.repo.InsertMember(ctx, repository.MemberRecord{
		GroupID:    groupID,
		IdentityID: identityID,
		CreatedBy:  actor,
		CreatedAt:  s.now(),
	})
	// When the group is missing too, the database reports whichever reference it checks first, so
	// the group is looked up to report a missing group first, like the other member methods.
	if errors.Is(err, repository.ErrIdentityNotFound) {
		if _, getErr := s.repo.Get(ctx, groupID); getErr != nil {
			err = getErr
		}
	}
	if err != nil {
		return fmt.Errorf("add group member: %w", translateError(err))
	}

	return nil
}

func (s *groupService) ListMembers(
	ctx context.Context, groupID uuid.UUID, offset, limit int,
) (pagination.Page[Member], error) {
	// One record more than requested reveals whether further members follow.
	recs, err := s.repo.ListMembers(ctx, groupID, offset, limit+1)
	if err != nil {
		return pagination.Page[Member]{}, fmt.Errorf("list group members: %w", err)
	}
	// A missing group has no member records either, so it's told apart by looking it up. Only
	// afterwards, so that a group deleted in between is reported as missing, not as empty.
	if len(recs) == 0 {
		if _, err := s.repo.Get(ctx, groupID); err != nil {
			return pagination.Page[Member]{}, fmt.Errorf("list group members: %w", translateError(err))
		}
	}

	members := make([]Member, len(recs))
	for i, rec := range recs {
		members[i] = toMember(rec)
	}

	return pagination.FromLookahead(members, limit), nil
}

func (s *groupService) RemoveMember(ctx context.Context, groupID, identityID uuid.UUID) error {
	err := s.repo.DeleteMember(ctx, groupID, identityID)
	// A missing group has no member record either, so it's told apart by looking it up. Only
	// afterwards, so that a group deleted in between is reported as missing.
	if errors.Is(err, repository.ErrMemberNotFound) {
		if _, getErr := s.repo.Get(ctx, groupID); getErr != nil {
			err = getErr
		}
	}
	if err != nil {
		return fmt.Errorf("remove group member: %w", translateError(err))
	}

	return nil
}

func toGroup(rec repository.GroupRecord) Group {
	return Group{
		ID:        rec.ID,
		Name:      rec.Name,
		CreatedBy: rec.CreatedBy,
		UpdatedBy: rec.UpdatedBy,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
}

func toMember(rec repository.MemberRecord) Member {
	return Member{
		IdentityID: rec.IdentityID,
		CreatedBy:  rec.CreatedBy,
		CreatedAt:  rec.CreatedAt,
	}
}
