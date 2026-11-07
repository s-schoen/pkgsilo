package service

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"pkgsilo/core/authz"
	"pkgsilo/core/pagination"
	"pkgsilo/feature/identity/repository"
)

// The actions on identities. Each settable field has its own action for each kind of write, such as
// identity:update:username, so that policies can limit writes to some fields. Creating has actions of its
// own, so that an actor who may rename itself can't create identities.
var (
	actionRead   = authz.NewAction("identity:read", authz.ActionGroupRead)
	actionDelete = authz.NewAction("identity:delete")
	createFields = authz.NewFieldActions("identity:create", "username", "displayName")
	updateFields = authz.NewFieldActions("identity:update", "username", "displayName")
)

// CreatableFields returns the names of the fields that creating an identity sets, each of which has an
// identity:create:<field> action.
func CreatableFields() []string {
	return createFields.Fields()
}

// UpdatableFields returns the names of the fields that replacing an identity sets, each of which has an
// identity:update:<field> action.
func UpdatableFields() []string {
	return updateFields.Fields()
}

// IdentityService manages identities. Changes are attributed to the given actor.
type IdentityService interface {
	// Create stores a new identity with the given input. It returns an *authz.FieldsDeniedError, without
	// storing anything, if the actor may not set any of its fields.
	Create(ctx context.Context, actor uuid.UUID, in IdentityInput) (Identity, error)
	// Get returns the identity with the given ID. An identity that the actor may not read is reported
	// like a missing one.
	Get(ctx context.Context, actor, id uuid.UUID) (Identity, error)
	// List returns the page of up to limit identities that skips the first offset, out of those that
	// the actor may read, ordered by creation time. Only the last page can be short.
	List(ctx context.Context, actor uuid.UUID, offset, limit int) (pagination.Page[Identity], error)
	// Replace overwrites all caller-set fields of the identity with the given ID. An identity that the
	// actor may not read is reported like a missing one. It returns an *authz.FieldsDeniedError, without
	// writing anything, if the actor may not set any of the fields, even to their current values.
	Replace(ctx context.Context, actor, id uuid.UUID, in IdentityInput) (Identity, error)
	// Delete removes the identity with the given ID. An identity that the actor may not read is reported
	// like a missing one, and one that it may read but not delete with authz.ErrDenied.
	Delete(ctx context.Context, actor, id uuid.UUID) error
}

type identityService struct {
	repo       repository.IdentityRepository
	authorizer authz.Authorizer
	now        func() time.Time
}

// NewIdentityService returns an IdentityService that stores identities in repo and asks authorizer what
// an actor may do.
func NewIdentityService(repo repository.IdentityRepository, authorizer authz.Authorizer) IdentityService {
	return newIdentityService(repo, authorizer, time.Now)
}

func newIdentityService(
	repo repository.IdentityRepository, authorizer authz.Authorizer, now func() time.Time,
) *identityService {
	return &identityService{repo: repo, authorizer: authorizer, now: now}
}

func (s *identityService) Create(ctx context.Context, actor uuid.UUID, in IdentityInput) (Identity, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Identity{}, fmt.Errorf("create identity: %w", err)
	}

	// A new identity is in no group yet.
	id := uuid.NewV4()
	if err := createFields.Check(decider, authz.Resource{Type: authz.IdentityType, ID: id}); err != nil {
		return Identity{}, fmt.Errorf("create identity: %w", err)
	}

	now := s.now()
	rec, err := s.repo.Insert(ctx, repository.IdentityRecord{
		ID:          id,
		Username:    in.Username,
		DisplayName: in.DisplayName,
		CreatedBy:   actor,
		UpdatedBy:   actor,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return Identity{}, fmt.Errorf("create identity: %w", translateError(err))
	}

	return toIdentity(rec), nil
}

func (s *identityService) Get(ctx context.Context, actor, id uuid.UUID) (Identity, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", err)
	}

	rec, _, err := s.getReadable(ctx, decider, id)
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", err)
	}

	return toIdentity(rec), nil
}

func (s *identityService) List(
	ctx context.Context, actor uuid.UUID, offset, limit int,
) (pagination.Page[Identity], error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return pagination.Page[Identity]{}, fmt.Errorf("list identities: %w", err)
	}

	recs, err := s.repo.ListAll(ctx)
	if err != nil {
		return pagination.Page[Identity]{}, fmt.Errorf("list identities: %w", err)
	}

	groups, err := s.groupsOf(ctx, recs)
	if err != nil {
		return pagination.Page[Identity]{}, fmt.Errorf("list identities: %w", err)
	}

	readable, err := authz.Filter(ctx, decider, actionRead, recs, func(rec repository.IdentityRecord) authz.Resource {
		return toResource(rec, groups)
	})
	if err != nil {
		return pagination.Page[Identity]{}, fmt.Errorf("list identities: %w", err)
	}

	page := pagination.FromAll(readable, offset, limit)
	identities := make([]Identity, len(page.Items))
	for i, rec := range page.Items {
		identities[i] = toIdentity(rec)
	}

	return pagination.Page[Identity]{Items: identities, HasMore: page.HasMore}, nil
}

func (s *identityService) Replace(ctx context.Context, actor, id uuid.UUID, in IdentityInput) (Identity, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Identity{}, fmt.Errorf("replace identity: %w", err)
	}

	_, resource, err := s.getReadable(ctx, decider, id)
	if err != nil {
		return Identity{}, fmt.Errorf("replace identity: %w", err)
	}

	// Replacing sets every field, so each one is checked, even if its value doesn't change.
	if err := updateFields.Check(decider, resource); err != nil {
		return Identity{}, fmt.Errorf("replace identity: %w", err)
	}

	rec, err := s.repo.Update(ctx, repository.IdentityRecord{
		ID:          id,
		Username:    in.Username,
		DisplayName: in.DisplayName,
		UpdatedBy:   actor,
		UpdatedAt:   s.now(),
	})
	if err != nil {
		return Identity{}, fmt.Errorf("replace identity: %w", translateError(err))
	}

	return toIdentity(rec), nil
}

func (s *identityService) Delete(ctx context.Context, actor, id uuid.UUID) error {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return fmt.Errorf("delete identity: %w", err)
	}

	_, resource, err := s.getReadable(ctx, decider, id)
	if err != nil {
		return fmt.Errorf("delete identity: %w", err)
	}

	can, err := decider.Can(actionDelete, resource)
	if err != nil {
		return fmt.Errorf("delete identity: %w", err)
	}
	if !can {
		return fmt.Errorf("delete identity: %w", authz.ErrDenied)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete identity: %w", translateError(err))
	}

	return nil
}

// getReadable returns the record of the identity with the given ID, and the identity as a resource of
// decisions. It reports an identity that the decider doesn't allow to read like a missing one, so that
// every endpoint hides it before checking anything else.
func (s *identityService) getReadable(
	ctx context.Context, decider authz.Decider, id uuid.UUID,
) (repository.IdentityRecord, authz.Resource, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return repository.IdentityRecord{}, authz.Resource{}, translateError(err)
	}

	groups, err := s.groupsOf(ctx, []repository.IdentityRecord{rec})
	if err != nil {
		return repository.IdentityRecord{}, authz.Resource{}, err
	}

	resource := toResource(rec, groups)
	can, err := decider.Can(actionRead, resource)
	if err != nil {
		return repository.IdentityRecord{}, authz.Resource{}, fmt.Errorf("check read: %w", err)
	}
	if !can {
		return repository.IdentityRecord{}, authz.Resource{}, ErrIdentityNotFound
	}

	return rec, resource, nil
}

// groupsOf returns the groups of each of the given identities, loaded in one query.
func (s *identityService) groupsOf(
	ctx context.Context, recs []repository.IdentityRecord,
) (map[uuid.UUID][]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(recs))
	for i, rec := range recs {
		ids[i] = rec.ID
	}

	memberships, err := s.repo.ListMemberships(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups: %w", err)
	}

	groups := make(map[uuid.UUID][]uuid.UUID, len(recs))
	for _, membership := range memberships {
		groups[membership.IdentityID] = append(groups[membership.IdentityID], membership.GroupID)
	}

	return groups, nil
}

// toResource returns the identity as a resource of a decision, with its groups out of the given ones.
func toResource(rec repository.IdentityRecord, groups map[uuid.UUID][]uuid.UUID) authz.Resource {
	return authz.Resource{Type: authz.IdentityType, ID: rec.ID, Groups: groups[rec.ID]}
}

func toIdentity(rec repository.IdentityRecord) Identity {
	return Identity{
		ID:          rec.ID,
		Username:    rec.Username,
		DisplayName: rec.DisplayName,
		CreatedBy:   rec.CreatedBy,
		UpdatedBy:   rec.UpdatedBy,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}
