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
	// Verify returns the identity with the username, and the hash of its password, if password is its
	// password. Otherwise, it returns a *RejectedError whose reason is ErrUnknownCredentials, for no identity
	// with the username or an identity without a password, or ErrInvalidPassword, for a wrong password. It
	// takes about as long in every case, so that the response time doesn't reveal which usernames exist. A
	// password whose hash has outdated parameters is rehashed once it's verified; a rehash that fails doesn't
	// fail Verify. It isn't authorized, since a request has no actor before it's authenticated. Its errors
	// never hold the username or the password.
	Verify(ctx context.Context, username, password string) (Verified, error)
}

type verifier struct {
	repo repository.PasswordRepository
}

// NewVerifier returns a Verifier that checks passwords against those stored in repo.
func NewVerifier(repo repository.PasswordRepository) Verifier {
	return &verifier{repo: repo}
}

func (v *verifier) Verify(ctx context.Context, username, password string) (Verified, error) {
	creds, err := v.repo.GetCredentials(ctx, username)
	if err != nil && !errors.Is(err, repository.ErrIdentityNotFound) {
		return Verified{}, fmt.Errorf("verify password: %w", err)
	}

	if err != nil || creds.Hash == nil {
		// Hashes all the same, so that a missing identity or password takes as long as a wrong password.
		if _, _, err := verifyPassword(ctx, password, dummyHash); err != nil {
			return Verified{}, fmt.Errorf("verify password: %w", err)
		}

		return Verified{}, &RejectedError{IdentityID: creds.IdentityID, Reason: ErrUnknownCredentials}
	}

	match, outdated, err := verifyPassword(ctx, password, *creds.Hash)
	if err != nil {
		return Verified{}, fmt.Errorf("verify password of identity %s: %w", creds.IdentityID, err)
	}
	if !match {
		return Verified{}, &RejectedError{IdentityID: creds.IdentityID, Reason: ErrInvalidPassword}
	}

	verified := Verified{IdentityID: creds.IdentityID, PasswordHash: *creds.Hash}
	if outdated {
		verified.PasswordHash, verified.RehashErr = v.rehash(ctx, creds.IdentityID, *creds.Hash, password)
	}

	return verified, nil
}

// rehash replaces the identity's password hash oldHash with a hash of password with the current parameters,
// and returns the hash that the password has then: the new one, or oldHash if the rehash failed or the
// password changed since oldHash was read, in which case it keeps its new hash.
func (v *verifier) rehash(ctx context.Context, identityID uuid.UUID, oldHash, password string) (string, error) {
	newHash, err := hashPassword(ctx, password)
	if err != nil {
		return oldHash, fmt.Errorf("rehash password of identity %s: %w", identityID, err)
	}

	replaced, err := v.repo.UpdateHash(ctx, identityID, oldHash, newHash)
	if err != nil {
		return oldHash, fmt.Errorf("rehash password of identity %s: %w", identityID, err)
	}
	if !replaced {
		return oldHash, nil
	}

	return newHash, nil
}
