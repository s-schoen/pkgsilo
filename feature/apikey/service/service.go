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

	"pkgsilo/core/authz"
	"pkgsilo/core/pagination"
	"pkgsilo/feature/apikey/repository"
	identityservice "pkgsilo/feature/identity/service"
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
//
// Every method but Authenticate first checks that the actor may read the API keys' identity, and reports
// one that it may not like a missing one, with ErrIdentityNotFound. An API key that the actor may not read
// is reported like a missing one too, with ErrAPIKeyNotFound. Every API key that they return holds only the
// fields that the actor may read, and the ID.
type APIKeyService interface {
	// Create stores a new API key of the identity with the given input and returns it with its key, which
	// is returned even if the actor may not read the API key, since it can't be retrieved later. A new API
	// key that the actor may not read is returned with only its ID and key. It returns an
	// *authz.FieldsDeniedError, without storing anything, if the actor may not set any of the fields that
	// the input sets. Leaving out the expiry needs no access to it.
	Create(ctx context.Context, actor, identityID uuid.UUID, in APIKeyInput) (CreatedAPIKey, error)
	// Get returns the API key with the given ID of the identity.
	Get(ctx context.Context, actor, identityID, id uuid.UUID) (APIKey, error)
	// List returns the page of up to limit API keys of the identity that skips the first offset, out of
	// those that the actor may read, ordered by creation time. Only the last page can be short.
	List(ctx context.Context, actor, identityID uuid.UUID, offset, limit int) (pagination.Page[APIKey], error)
	// Delete removes the API key with the given ID of the identity. Its key stops working at once. An API
	// key that the actor may read but not delete is reported with authz.ErrDenied.
	Delete(ctx context.Context, actor, identityID, id uuid.UUID) error
	// Authenticate returns the API key that the presented key belongs to and records its use. It
	// returns an error wrapping ErrMalformedKey, ErrUnknownKey or ErrExpiredKey if the key is not
	// accepted. Its errors never hold the key. It isn't authorized, since a request has no actor before
	// it's authenticated.
	Authenticate(ctx context.Context, key string) (AuthenticatedKey, error)
}

type apiKeyService struct {
	repo       repository.APIKeyRepository
	authorizer authz.Authorizer
	pepper     []byte
	now        func() time.Time
}

// NewAPIKeyService returns an APIKeyService that stores API keys in repo, with their keys hashed under
// the secret pepper, and asks authorizer what an actor may do.
func NewAPIKeyService(repo repository.APIKeyRepository, authorizer authz.Authorizer, pepper []byte) APIKeyService {
	return newAPIKeyService(repo, authorizer, pepper, time.Now)
}

func newAPIKeyService(
	repo repository.APIKeyRepository, authorizer authz.Authorizer, pepper []byte, now func() time.Time,
) *apiKeyService {
	return &apiKeyService{repo: repo, authorizer: authorizer, pepper: pepper, now: now}
}

func (s *apiKeyService) Create(
	ctx context.Context, actor, identityID uuid.UUID, in APIKeyInput,
) (CreatedAPIKey, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return CreatedAPIKey{}, fmt.Errorf("create api key: %w", err)
	}

	owner, err := s.readableOwner(ctx, decider, identityID)
	if err != nil {
		return CreatedAPIKey{}, fmt.Errorf("create api key: %w", err)
	}

	id := uuid.NewV4()
	if err := createFields.Only(setFields(in)...).Check(decider, toResource(id, owner)); err != nil {
		return CreatedAPIKey{}, fmt.Errorf("create api key: %w", err)
	}

	// Checked before writing, so that a failed decision can't fail a request whose write succeeded, and
	// lose the key that the caller couldn't retrieve again.
	fields, err := readFields.ReadableIf(decider, actionRead, toResource(id, owner))
	if err != nil {
		return CreatedAPIKey{}, fmt.Errorf("create api key: %w", err)
	}

	key := generateKey()
	rec, err := s.repo.Insert(ctx, repository.APIKeyRecord{
		ID:         id,
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

	return CreatedAPIKey{APIKey: toAPIKey(rec, fields), Key: key}, nil
}

func (s *apiKeyService) Get(ctx context.Context, actor, identityID, id uuid.UUID) (APIKey, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return APIKey{}, fmt.Errorf("get api key: %w", err)
	}

	owner, err := s.readableOwner(ctx, decider, identityID)
	if err != nil {
		return APIKey{}, fmt.Errorf("get api key: %w", err)
	}

	rec, err := s.getReadable(ctx, decider, owner, id)
	if err != nil {
		return APIKey{}, fmt.Errorf("get api key: %w", err)
	}

	fields, err := readFields.Readable(decider, toResource(id, owner))
	if err != nil {
		return APIKey{}, fmt.Errorf("get api key: %w", err)
	}

	return toAPIKey(rec, fields), nil
}

func (s *apiKeyService) List(
	ctx context.Context, actor, identityID uuid.UUID, offset, limit int,
) (pagination.Page[APIKey], error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return pagination.Page[APIKey]{}, fmt.Errorf("list api keys: %w", err)
	}

	owner, err := s.readableOwner(ctx, decider, identityID)
	if err != nil {
		return pagination.Page[APIKey]{}, fmt.Errorf("list api keys: %w", err)
	}

	recs, err := s.repo.ListAll(ctx, identityID)
	if err != nil {
		return pagination.Page[APIKey]{}, fmt.Errorf("list api keys: %w", err)
	}

	readable, err := authz.Filter(ctx, decider, actionRead, recs, func(rec repository.APIKeyRecord) authz.Resource {
		return toResource(rec.ID, owner)
	})
	if err != nil {
		return pagination.Page[APIKey]{}, fmt.Errorf("list api keys: %w", err)
	}

	// Only the fields of the rows on the page are checked.
	page := pagination.FromAll(readable, offset, limit)
	keys := make([]APIKey, len(page.Items))
	for i, rec := range page.Items {
		fields, err := readFields.Readable(decider, toResource(rec.ID, owner))
		if err != nil {
			return pagination.Page[APIKey]{}, fmt.Errorf("list api keys: %w", err)
		}
		keys[i] = toAPIKey(rec, fields)
	}

	return pagination.Page[APIKey]{Items: keys, HasMore: page.HasMore}, nil
}

func (s *apiKeyService) Delete(ctx context.Context, actor, identityID, id uuid.UUID) error {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}

	owner, err := s.readableOwner(ctx, decider, identityID)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}

	if _, err := s.getReadable(ctx, decider, owner, id); err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}

	can, err := decider.Can(actionDelete, toResource(id, owner))
	if err != nil {
		return fmt.Errorf("delete api key: check action: %w", err)
	}
	if !can {
		return fmt.Errorf("delete api key: %w", authz.ErrDenied)
	}

	if err := s.repo.Delete(ctx, identityID, id); err != nil {
		return fmt.Errorf("delete api key: %w", translateError(err))
	}

	return nil
}

func (s *apiKeyService) Authenticate(ctx context.Context, key string) (AuthenticatedKey, error) {
	if !strings.HasPrefix(key, keyPrefix) {
		return AuthenticatedKey{}, ErrMalformedKey
	}

	rec, err := s.repo.GetByHash(ctx, s.hash(key))
	if errors.Is(err, repository.ErrNotFound) {
		return AuthenticatedKey{}, ErrUnknownKey
	}
	if err != nil {
		return AuthenticatedKey{}, fmt.Errorf("authenticate api key: %w", err)
	}

	now := s.now()
	if rec.ExpiresAt != nil && !now.Before(*rec.ExpiresAt) {
		return AuthenticatedKey{}, fmt.Errorf("api key %s: %w", rec.ID, ErrExpiredKey)
	}

	if rec.LastUsedAt == nil || now.Sub(*rec.LastUsedAt) >= lastUsedInterval {
		if err := s.repo.UpdateLastUsed(ctx, rec.ID, now); err != nil {
			return AuthenticatedKey{}, fmt.Errorf("authenticate api key: %w", err)
		}
	}

	return AuthenticatedKey{ID: rec.ID, IdentityID: rec.IdentityID}, nil
}

// readableOwner returns the identity with the given ID as the owner of its API keys, with its groups. It
// reports an identity that the decider doesn't allow to read like a missing one, with ErrIdentityNotFound,
// so that no endpoint of its API keys reveals whether it exists.
func (s *apiKeyService) readableOwner(
	ctx context.Context, decider authz.Decider, identityID uuid.UUID,
) (*authz.Owner, error) {
	groups, err := s.repo.IdentityGroups(ctx, identityID)
	if err != nil {
		return nil, translateError(err)
	}

	can, err := decider.Can(identityservice.ReadAction(),
		authz.Resource{Type: authz.IdentityType, ID: identityID, Groups: groups})
	if err != nil {
		return nil, fmt.Errorf("check identity read: %w", err)
	}
	if !can {
		return nil, ErrIdentityNotFound
	}

	return &authz.Owner{ID: identityID, Groups: groups}, nil
}

// getReadable returns the record of the API key with the given ID of the owner. It reports an API key that
// the decider doesn't allow to read like a missing one.
func (s *apiKeyService) getReadable(
	ctx context.Context, decider authz.Decider, owner *authz.Owner, id uuid.UUID,
) (repository.APIKeyRecord, error) {
	rec, err := s.repo.Get(ctx, owner.ID, id)
	if err != nil {
		return repository.APIKeyRecord{}, translateError(err)
	}

	can, err := decider.Can(actionRead, toResource(id, owner))
	if err != nil {
		return repository.APIKeyRecord{}, fmt.Errorf("check read: %w", err)
	}
	if !can {
		return repository.APIKeyRecord{}, ErrAPIKeyNotFound
	}

	return rec, nil
}

// setFields returns the names of the fields that creating an API key with the input sets. An API key
// without an expiry never expires, so leaving it out sets nothing.
func setFields(in APIKeyInput) []string {
	fields := []string{"name"}
	if in.ExpiresAt != nil {
		fields = append(fields, "expiresAt")
	}

	return fields
}

// toResource returns the API key with the given ID of the owner as a resource of a decision.
func toResource(id uuid.UUID, owner *authz.Owner) authz.Resource {
	return authz.Resource{Type: resourceType, ID: id, Owner: owner}
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

// toAPIKey returns the API key of the record with its ID and the readable fields. Every other field stays
// hidden, so that one that's missing here can't be revealed by mistake.
func toAPIKey(rec repository.APIKeyRecord, fields authz.ReadableFields) APIKey {
	return APIKey{
		ID:         rec.ID,
		IdentityID: authz.Reveal(fields, "identityId", rec.IdentityID),
		Name:       authz.Reveal(fields, "name", rec.Name),
		KeySuffix:  authz.Reveal(fields, "keySuffix", rec.KeySuffix),
		ExpiresAt:  authz.Reveal(fields, "expiresAt", rec.ExpiresAt),
		LastUsedAt: authz.Reveal(fields, "lastUsedAt", rec.LastUsedAt),
		CreatedBy:  authz.Reveal(fields, "createdBy", rec.CreatedBy),
		CreatedAt:  authz.Reveal(fields, "createdAt", rec.CreatedAt),
	}
}
