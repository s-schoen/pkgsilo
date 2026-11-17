package service

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"pkgsilo/feature/password/repository"
)

// Verifier checks a username and password, as a login does. It's separate from PasswordService, since it
// needs only the repository: the password service ends sessions, and the sessions need the verifier.
type Verifier interface {
	// Verify returns the ID of the identity with the username if password is its password. Otherwise, it
	// returns a *RejectedError whose reason is ErrUnknownCredentials, for no identity with the username or an
	// identity without a password, or ErrInvalidPassword, for a wrong password. It takes about as long in
	// every case, so that the response time doesn't reveal which usernames exist. A password whose hash has
	// outdated parameters is rehashed once it's verified. It isn't authorized, since a request has no actor
	// before it's authenticated. Its errors never hold the username or the password.
	Verify(ctx context.Context, username, password string) (uuid.UUID, error)
}

type verifier struct {
	repo repository.PasswordRepository
}

// NewVerifier returns a Verifier that checks passwords against those stored in repo.
func NewVerifier(repo repository.PasswordRepository) Verifier {
	return &verifier{repo: repo}
}

func (v *verifier) Verify(ctx context.Context, username, password string) (uuid.UUID, error) {
	creds, err := v.repo.GetCredentials(ctx, username)
	if err != nil && !errors.Is(err, repository.ErrIdentityNotFound) {
		return uuid.Nil(), fmt.Errorf("verify password: %w", err)
	}

	if err != nil || creds.Hash == nil {
		// Hashes all the same, so that a missing identity or password takes as long as a wrong password.
		if _, _, err := verifyPassword(ctx, password, dummyHash); err != nil {
			return uuid.Nil(), fmt.Errorf("verify password: %w", err)
		}

		return uuid.Nil(), &RejectedError{IdentityID: creds.IdentityID, Reason: ErrUnknownCredentials}
	}

	match, outdated, err := verifyPassword(ctx, password, *creds.Hash)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("verify password of identity %s: %w", creds.IdentityID, err)
	}
	if !match {
		return uuid.Nil(), &RejectedError{IdentityID: creds.IdentityID, Reason: ErrInvalidPassword}
	}

	if outdated {
		if err := v.rehash(ctx, creds.IdentityID, *creds.Hash, password); err != nil {
			return uuid.Nil(), fmt.Errorf("verify password of identity %s: %w", creds.IdentityID, err)
		}
	}

	return creds.IdentityID, nil
}

// rehash replaces the identity's password hash oldHash with a hash of password with the current parameters.
// A password that changed since oldHash was read keeps its new hash.
func (v *verifier) rehash(ctx context.Context, identityID uuid.UUID, oldHash, password string) error {
	newHash, err := hashPassword(ctx, password)
	if err != nil {
		return fmt.Errorf("rehash: %w", err)
	}

	if _, err := v.repo.UpdateHash(ctx, identityID, oldHash, newHash); err != nil {
		return fmt.Errorf("rehash: %w", err)
	}

	return nil
}
