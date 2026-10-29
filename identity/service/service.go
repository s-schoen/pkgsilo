package service

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"pkgsilo/core/pagination"
	"pkgsilo/identity/repository"
)

// IdentityService manages identities. Changes are attributed to the given actor.
type IdentityService interface {
	Create(ctx context.Context, actor uuid.UUID, in IdentityInput) (Identity, error)
	Get(ctx context.Context, id uuid.UUID) (Identity, error)
	// List returns the page of up to limit identities that skips the first offset, ordered by
	// creation time.
	List(ctx context.Context, offset, limit int) (pagination.Page[Identity], error)
	// Replace overwrites all caller-set fields of the identity with the given ID.
	Replace(ctx context.Context, actor, id uuid.UUID, in IdentityInput) (Identity, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type identityService struct {
	repo repository.IdentityRepository
	now  func() time.Time
}

// NewIdentityService returns an IdentityService that stores identities in repo.
func NewIdentityService(repo repository.IdentityRepository) IdentityService {
	return newIdentityService(repo, time.Now)
}

func newIdentityService(repo repository.IdentityRepository, now func() time.Time) *identityService {
	return &identityService{repo: repo, now: now}
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

func (s *identityService) Get(ctx context.Context, id uuid.UUID) (Identity, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", translateError(err))
	}

	return toIdentity(rec), nil
}

func (s *identityService) List(ctx context.Context, offset, limit int) (pagination.Page[Identity], error) {
	// One record more than requested reveals whether further identities follow.
	recs, err := s.repo.List(ctx, offset, limit+1)
	if err != nil {
		return pagination.Page[Identity]{}, fmt.Errorf("list identities: %w", err)
	}

	identities := make([]Identity, len(recs))
	for i, rec := range recs {
		identities[i] = toIdentity(rec)
	}

	return pagination.FromLookahead(identities, limit), nil
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
