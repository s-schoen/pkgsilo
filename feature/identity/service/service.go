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

// IdentityService manages identities. Changes are attributed to the given actor.
//
// Every identity that it returns holds only the fields that the actor may read, and the ID.
type IdentityService interface {
	// Create stores a new identity with the given input. It returns an *authz.FieldsDeniedError, without
	// storing anything, if the actor may not set any of its fields. A new identity that the actor may not
	// read is returned with only its ID.
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
	// Patch overwrites the fields that the patch sets of the identity with the given ID, and keeps the
	// others, even if they change concurrently. An identity that the actor may not read is reported like a
	// missing one. It returns an *authz.FieldsDeniedError, without writing anything, if the actor may not
	// set any of the fields that the patch sets. A patch that sets no field writes nothing, so that the
	// identity is returned as it is.
	Patch(ctx context.Context, actor, id uuid.UUID, patch IdentityPatch) (Identity, error)
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
	resource := authz.Resource{Type: authz.IdentityType, ID: id}
	if err := actions.CreateFields().Check(decider, resource); err != nil {
		return Identity{}, fmt.Errorf("create identity: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded.
	fields, err := actions.ReadFields().ReadableIf(decider, actions.Read(), resource)
	if err != nil {
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

	return toIdentity.Map(rec, fields), nil
}

func (s *identityService) Get(ctx context.Context, actor, id uuid.UUID) (Identity, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", err)
	}

	rec, resource, err := s.getReadable(ctx, decider, id)
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", err)
	}

	fields, err := actions.ReadFields().Readable(decider, resource)
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", err)
	}

	return toIdentity.Map(rec, fields), nil
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

	readable, err := authz.Filter(ctx, decider, actions.Read(), recs, func(rec repository.IdentityRecord) authz.Resource {
		return toResource(rec, groups)
	})
	if err != nil {
		return pagination.Page[Identity]{}, fmt.Errorf("list identities: %w", err)
	}

	// Only the fields of the rows on the page are checked.
	page := pagination.FromAll(readable, offset, limit)
	identities := make([]Identity, len(page.Items))
	for i, rec := range page.Items {
		fields, err := actions.ReadFields().Readable(decider, toResource(rec, groups))
		if err != nil {
			return pagination.Page[Identity]{}, fmt.Errorf("list identities: %w", err)
		}
		identities[i] = toIdentity.Map(rec, fields)
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
	if err := actions.UpdateFields().Check(decider, resource); err != nil {
		return Identity{}, fmt.Errorf("replace identity: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded.
	fields, err := actions.ReadFields().Readable(decider, resource)
	if err != nil {
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

	return toIdentity.Map(rec, fields), nil
}

func (s *identityService) Patch(ctx context.Context, actor, id uuid.UUID, patch IdentityPatch) (Identity, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Identity{}, fmt.Errorf("patch identity: %w", err)
	}

	rec, resource, err := s.getReadable(ctx, decider, id)
	if err != nil {
		return Identity{}, fmt.Errorf("patch identity: %w", err)
	}

	// Patching sets only the fields in the patch, so only those are checked.
	if err := actions.UpdateFields().Only(authz.SetFields(patch)...).Check(decider, resource); err != nil {
		return Identity{}, fmt.Errorf("patch identity: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded.
	fields, err := actions.ReadFields().Readable(decider, resource)
	if err != nil {
		return Identity{}, fmt.Errorf("patch identity: %w", err)
	}

	if len(authz.SetFields(patch)) == 0 {
		return toIdentity.Map(rec, fields), nil
	}

	rec, err = s.repo.Patch(ctx, repository.IdentityPatch{
		ID:          id,
		Username:    patch.Username,
		DisplayName: patch.DisplayName,
		UpdatedBy:   actor,
		UpdatedAt:   s.now(),
	})
	if err != nil {
		return Identity{}, fmt.Errorf("patch identity: %w", translateError(err))
	}

	return toIdentity.Map(rec, fields), nil
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

	can, err := decider.Can(actions.Delete(), resource)
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
	can, err := decider.Can(actions.Read(), resource)
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

// toIdentity builds the identity of a record, with its ID and the fields that the actor may read. Every other
// field stays hidden.
var toIdentity = authz.NewMapper[repository.IdentityRecord, Identity](actions)
