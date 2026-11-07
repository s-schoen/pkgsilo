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

// actionRead is the action of reading an identity.
var actionRead = authz.NewAction("identity:read", authz.ActionGroupRead)

// IdentityService manages identities. Changes are attributed to the given actor.
type IdentityService interface {
	// Create stores a new identity with the given input.
	Create(ctx context.Context, actor uuid.UUID, in IdentityInput) (Identity, error)
	// Get returns the identity with the given ID. An identity that the actor may not read is reported
	// like a missing one.
	Get(ctx context.Context, actor, id uuid.UUID) (Identity, error)
	// List returns the page of up to limit identities that skips the first offset, out of those that
	// the actor may read, ordered by creation time. Only the last page can be short.
	List(ctx context.Context, actor uuid.UUID, offset, limit int) (pagination.Page[Identity], error)
	// Replace overwrites all caller-set fields of the identity with the given ID.
	Replace(ctx context.Context, actor, id uuid.UUID, in IdentityInput) (Identity, error)
	// Delete removes the identity with the given ID.
	Delete(ctx context.Context, id uuid.UUID) error
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
	now := s.now()
	rec, err := s.repo.Insert(ctx, repository.IdentityRecord{
		ID:          uuid.NewV4(),
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

	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", translateError(err))
	}

	groups, err := s.groupsOf(ctx, []repository.IdentityRecord{rec})
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", err)
	}

	can, err := decider.Can(actionRead, toResource(rec, groups))
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", err)
	}
	if !can {
		return Identity{}, fmt.Errorf("get identity: %w", ErrIdentityNotFound)
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

func (s *identityService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete identity: %w", translateError(err))
	}

	return nil
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
