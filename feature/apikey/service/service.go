// Package service implements the API key domain: the credentials that authenticate requests and the
// rules for managing them.
package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	"pkgsilo/core/pagination"
	"pkgsilo/feature/apikey/repository"
)

const (
	// keyPrefix starts every key, so that a key can be told apart from other values, by secret
	// scanners too.
	keyPrefix = "psk_"
	// keySuffixLength is the number of characters of a key that are stored and shown. They leave at
	// least 236 of its random bits unknown.
	keySuffixLength = 4
	// lastUsedInterval is the least time between two writes of an API key's last use, so that busy
	// keys don't cause a write per request.
	lastUsedInterval = time.Minute
)

// APIKeyService manages API keys and authenticates their keys. Changes are attributed to the given
// actor.
type APIKeyService interface {
	// Create stores a new API key of the identity with the given input and returns it with its key.
	Create(ctx context.Context, actor, identityID uuid.UUID, in APIKeyInput) (CreatedAPIKey, error)
	// Get returns the API key with the given ID of the identity.
	Get(ctx context.Context, identityID, id uuid.UUID) (APIKey, error)
	// List returns the page of up to limit API keys of the identity that skips the first offset,
	// ordered by creation time.
	List(ctx context.Context, identityID uuid.UUID, offset, limit int) (pagination.Page[APIKey], error)
	// Delete removes the API key with the given ID of the identity. Its key stops working at once.
	Delete(ctx context.Context, identityID, id uuid.UUID) error
	// Authenticate returns the API key that the presented key belongs to and records its use. It
	// returns an error wrapping ErrMalformedKey, ErrUnknownKey or ErrExpiredKey if the key is not
	// accepted. Its errors never hold the key.
	Authenticate(ctx context.Context, key string) (APIKey, error)
}

type apiKeyService struct {
	repo   repository.APIKeyRepository
	pepper []byte
	now    func() time.Time
}

// NewAPIKeyService returns an APIKeyService that stores API keys in repo, with their keys hashed under
// the secret pepper.
func NewAPIKeyService(repo repository.APIKeyRepository, pepper []byte) APIKeyService {
	return newAPIKeyService(repo, pepper, time.Now)
}

func newAPIKeyService(repo repository.APIKeyRepository, pepper []byte, now func() time.Time) *apiKeyService {
	return &apiKeyService{repo: repo, pepper: pepper, now: now}
}

func (s *apiKeyService) Create(
	ctx context.Context, actor, identityID uuid.UUID, in APIKeyInput,
) (CreatedAPIKey, error) {
	key := generateKey()
	rec, err := s.repo.Insert(ctx, repository.APIKeyRecord{
		ID:         uuid.NewV4(),
		IdentityID: identityID,
		Name:       in.Name,
		KeyHash:    s.hash(key),
		KeySuffix:  key[len(key)-keySuffixLength:],
		ExpiresAt:  in.ExpiresAt,
		CreatedBy:  actor,
		CreatedAt:  s.now(),
	})
	if err != nil {
		return CreatedAPIKey{}, fmt.Errorf("create api key: %w", translateError(err))
	}

	return CreatedAPIKey{APIKey: toAPIKey(rec), Key: key}, nil
}

func (s *apiKeyService) Get(ctx context.Context, identityID, id uuid.UUID) (APIKey, error) {
	rec, err := s.repo.Get(ctx, identityID, id)
	if errors.Is(err, repository.ErrNotFound) {
		err = s.notFoundCause(ctx, identityID, err)
	}
	if err != nil {
		return APIKey{}, fmt.Errorf("get api key: %w", translateError(err))
	}

	return toAPIKey(rec), nil
}

func (s *apiKeyService) List(
	ctx context.Context, identityID uuid.UUID, offset, limit int,
) (pagination.Page[APIKey], error) {
	// One record more than requested reveals whether further API keys follow.
	recs, err := s.repo.List(ctx, identityID, offset, limit+1)
	if err != nil {
		return pagination.Page[APIKey]{}, fmt.Errorf("list api keys: %w", err)
	}
	if len(recs) == 0 {
		if err := s.notFoundCause(ctx, identityID, nil); err != nil {
			return pagination.Page[APIKey]{}, fmt.Errorf("list api keys: %w", translateError(err))
		}
	}

	keys := make([]APIKey, len(recs))
	for i, rec := range recs {
		keys[i] = toAPIKey(rec)
	}

	return pagination.FromLookahead(keys, limit), nil
}

func (s *apiKeyService) Delete(ctx context.Context, identityID, id uuid.UUID) error {
	err := s.repo.Delete(ctx, identityID, id)
	if errors.Is(err, repository.ErrNotFound) {
		err = s.notFoundCause(ctx, identityID, err)
	}
	if err != nil {
		return fmt.Errorf("delete api key: %w", translateError(err))
	}

	return nil
}

func (s *apiKeyService) Authenticate(ctx context.Context, key string) (APIKey, error) {
	if !strings.HasPrefix(key, keyPrefix) {
		return APIKey{}, ErrMalformedKey
	}

	rec, err := s.repo.GetByHash(ctx, s.hash(key))
	if errors.Is(err, repository.ErrNotFound) {
		return APIKey{}, ErrUnknownKey
	}
	if err != nil {
		return APIKey{}, fmt.Errorf("authenticate api key: %w", err)
	}

	now := s.now()
	if rec.ExpiresAt != nil && !now.Before(*rec.ExpiresAt) {
		return APIKey{}, fmt.Errorf("api key %s: %w", rec.ID, ErrExpiredKey)
	}

	if rec.LastUsedAt == nil || now.Sub(*rec.LastUsedAt) >= lastUsedInterval {
		if err := s.repo.UpdateLastUsed(ctx, rec.ID, now); err != nil {
			return APIKey{}, fmt.Errorf("authenticate api key: %w", err)
		}
		rec.LastUsedAt = &now
	}

	return toAPIKey(rec), nil
}

// notFoundCause returns the error for an API key or list of API keys that wasn't found: err, or
// repository.ErrIdentityNotFound if the identity doesn't exist. A missing identity has no API keys
// either, so it's told apart by looking it up. Only afterwards, so that an identity deleted in between
// is reported as missing.
func (s *apiKeyService) notFoundCause(ctx context.Context, identityID uuid.UUID, err error) error {
	exists, existsErr := s.repo.IdentityExists(ctx, identityID)
	switch {
	case existsErr != nil:
		return fmt.Errorf("look up identity: %w", existsErr)
	case !exists:
		return repository.ErrIdentityNotFound
	default:
		return err
	}
}

// hash returns the keyed hash of key that's stored and looked up. It's deterministic, so that a key
// can be found by it, which is safe without a slow password hash because keys can't be guessed.
func (s *apiKeyService) hash(key string) []byte {
	mac := hmac.New(sha256.New, s.pepper)
	// A hash's Write never returns an error.
	_, _ = mac.Write([]byte(key))

	return mac.Sum(nil)
}

// generateKey returns a new key: the prefix and at least 256 random bits, since rand.Text holds at
// least 128. Unlike encoded random bytes of that size, every character of it is equally random, which
// matters for the suffix that identifies the key.
func generateKey() string {
	return keyPrefix + rand.Text() + rand.Text()
}

func toAPIKey(rec repository.APIKeyRecord) APIKey {
	return APIKey{
		ID:         rec.ID,
		IdentityID: rec.IdentityID,
		Name:       rec.Name,
		KeySuffix:  rec.KeySuffix,
		ExpiresAt:  rec.ExpiresAt,
		LastUsedAt: rec.LastUsedAt,
		CreatedBy:  rec.CreatedBy,
		CreatedAt:  rec.CreatedAt,
	}
}
