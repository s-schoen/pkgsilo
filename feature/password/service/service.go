// Package service implements the password domain: the passwords of identities, the rules for setting them,
// and the check of a username and password.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"pkgsilo/core/authz"
	identityservice "pkgsilo/feature/identity/service"
	"pkgsilo/feature/password/repository"
)

// PasswordService manages the passwords of identities. An identity has at most one password.
//
// Every method first checks that the actor may read the password's identity, and reports one that it may
// not like a missing one, with ErrIdentityNotFound. A password that the actor may not read is reported like
// a missing one too, with ErrPasswordNotFound. Every password that they return holds only the fields that
// the actor may read.
type PasswordService interface {
	// Get returns the identity's password.
	Get(ctx context.Context, actor, identityID uuid.UUID) (Password, error)
	// Set sets the identity's password to in.NewPassword and returns it. An actor that may reset the password
	// sets it without the current one, which is the only way to set a first password, and in.CurrentPassword
	// is ignored. An actor that may only change it must give the current one: Set returns
	// ErrCurrentPasswordRequired if in.CurrentPassword is nil, and ErrWrongCurrentPassword if it doesn't
	// match. It returns ErrPasswordNotFound for an identity without a password, and authz.ErrDenied if the
	// actor may read the password but neither reset nor change it. It writes nothing on an error.
	Set(ctx context.Context, actor, identityID uuid.UUID, in PasswordInput) (Password, error)
	// Delete removes the identity's password, which then can't be used to log in. A password that the actor
	// may read but not delete is reported with authz.ErrDenied.
	//
	// Set and Delete end every session of the identity once the password is written, the actor's own
	// included, so that a stolen session doesn't survive. If that fails, the password stays written and they
	// return the error, so that the request fails and can be retried.
	Delete(ctx context.Context, actor, identityID uuid.UUID) error
}

// SessionTerminator ends the sessions of identities. The password service owns it, so that the session
// feature, which checks passwords with the Verifier, implements it without an import cycle.
type SessionTerminator interface {
	// EndSessions ends every session of the identity. An identity without sessions is not an error.
	EndSessions(ctx context.Context, identityID uuid.UUID) error
}

type passwordService struct {
	repo       repository.PasswordRepository
	authorizer authz.Authorizer
	terminator SessionTerminator
	now        func() time.Time
}

// NewPasswordService returns a PasswordService that stores passwords in repo, hashed with Argon2id, asks
// authorizer what an actor may do, and ends the sessions of an identity whose password changes with
// terminator.
func NewPasswordService(
	repo repository.PasswordRepository, authorizer authz.Authorizer, terminator SessionTerminator,
) PasswordService {
	return newPasswordService(repo, authorizer, terminator, time.Now)
}

func newPasswordService(
	repo repository.PasswordRepository, authorizer authz.Authorizer, terminator SessionTerminator,
	now func() time.Time,
) *passwordService {
	return &passwordService{repo: repo, authorizer: authorizer, terminator: terminator, now: now}
}

func (s *passwordService) Get(ctx context.Context, actor, identityID uuid.UUID) (Password, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Password{}, fmt.Errorf("get password: %w", err)
	}

	owner, err := s.readableOwner(ctx, decider, identityID)
	if err != nil {
		return Password{}, fmt.Errorf("get password: %w", err)
	}

	rec, err := s.getReadable(ctx, decider, owner)
	if err != nil {
		return Password{}, fmt.Errorf("get password: %w", err)
	}

	fields, err := actions.ReadFields().Readable(decider, toResource(owner))
	if err != nil {
		return Password{}, fmt.Errorf("get password: %w", err)
	}

	return toPassword(rec, fields), nil
}

func (s *passwordService) Set(
	ctx context.Context, actor, identityID uuid.UUID, in PasswordInput,
) (Password, error) {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return Password{}, fmt.Errorf("set password: %w", err)
	}

	owner, err := s.readableOwner(ctx, decider, identityID)
	if err != nil {
		return Password{}, fmt.Errorf("set password: %w", err)
	}

	rec, err := s.repo.Get(ctx, identityID)
	exists := err == nil
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return Password{}, fmt.Errorf("set password: %w", err)
	}

	reset, fields, err := authorizeSet(decider, toResource(owner), exists, in)
	if err != nil {
		return Password{}, fmt.Errorf("set password: %w", err)
	}

	hash, err := hashNewPassword(ctx, reset, in, rec.Hash)
	if err != nil {
		return Password{}, fmt.Errorf("set password: %w", err)
	}

	stored, err := s.repo.Set(ctx, repository.PasswordRecord{IdentityID: identityID, Hash: hash, SetAt: s.now()})
	if err != nil {
		return Password{}, fmt.Errorf("set password: %w", translateError(err))
	}

	// Only after the write: otherwise someone with the old password could log in before it and keep their
	// session.
	if err := s.terminator.EndSessions(ctx, identityID); err != nil {
		return Password{}, fmt.Errorf("set password: %w", err)
	}

	return toPassword(stored, fields), nil
}

// authorizeSet checks that the decider allows setting the resource, a password that exists or not, with in.
// It reports whether the actor resets the password, rather than changing it with the current one, and the
// fields of the password that the actor may read. It reports a password that exists but that the actor may
// not read like a missing one. It checks everything before anything is written, so that a failed decision
// can't fail a request whose write succeeded, but doesn't check that the current password matches.
func authorizeSet(
	decider authz.Decider, resource authz.Resource, exists bool, in PasswordInput,
) (bool, authz.ReadableFields, error) {
	none := actions.ReadFields().NoneReadable()

	readable, err := decider.Can(actions.Read(), resource)
	if err != nil {
		return false, none, fmt.Errorf("check read: %w", err)
	}
	if exists && !readable {
		return false, none, ErrPasswordNotFound
	}

	reset, err := decider.Can(resetAction, resource)
	if err != nil {
		return false, none, fmt.Errorf("check reset: %w", err)
	}
	if !reset {
		if err := checkChange(decider, resource, exists, in); err != nil {
			return false, none, err
		}
	}

	if !readable {
		return reset, none, nil
	}

	fields, err := actions.ReadFields().Readable(decider, resource)
	if err != nil {
		return false, none, fmt.Errorf("check fields: %w", err)
	}

	return reset, fields, nil
}

// hashNewPassword returns the hash of in.NewPassword. Unless the actor resets the password, it first checks
// that in.CurrentPassword, which must be set, is the password of currentHash.
func hashNewPassword(ctx context.Context, reset bool, in PasswordInput, currentHash string) (string, error) {
	if !reset {
		if err := checkCurrentPassword(ctx, *in.CurrentPassword, currentHash); err != nil {
			return "", err
		}
	}

	return hashPassword(ctx, in.NewPassword)
}

// checkCurrentPassword returns ErrWrongCurrentPassword if current isn't the password of hash.
func checkCurrentPassword(ctx context.Context, current, hash string) error {
	match, _, err := verifyPassword(ctx, current, hash)
	if err != nil {
		return fmt.Errorf("verify current password: %w", err)
	}
	if !match {
		return ErrWrongCurrentPassword
	}

	return nil
}

// checkChange checks that the decider allows changing the resource, a password that exists, with the
// current password, which in must hold. It doesn't check that the current password matches.
func checkChange(decider authz.Decider, resource authz.Resource, exists bool, in PasswordInput) error {
	// Only a reset sets a first password.
	if !exists {
		return ErrPasswordNotFound
	}

	can, err := decider.Can(changeAction, resource)
	if err != nil {
		return fmt.Errorf("check change: %w", err)
	}
	if !can {
		return authz.ErrDenied
	}

	if in.CurrentPassword == nil {
		return ErrCurrentPasswordRequired
	}

	return nil
}

func (s *passwordService) Delete(ctx context.Context, actor, identityID uuid.UUID) error {
	decider, err := s.authorizer.For(ctx, actor)
	if err != nil {
		return fmt.Errorf("delete password: %w", err)
	}

	owner, err := s.readableOwner(ctx, decider, identityID)
	if err != nil {
		return fmt.Errorf("delete password: %w", err)
	}

	if _, err := s.getReadable(ctx, decider, owner); err != nil {
		return fmt.Errorf("delete password: %w", err)
	}

	can, err := decider.Can(actions.Delete(), toResource(owner))
	if err != nil {
		return fmt.Errorf("delete password: check action: %w", err)
	}
	if !can {
		return fmt.Errorf("delete password: %w", authz.ErrDenied)
	}

	if err := s.repo.Delete(ctx, identityID); err != nil {
		return fmt.Errorf("delete password: %w", translateError(err))
	}

	if err := s.terminator.EndSessions(ctx, identityID); err != nil {
		return fmt.Errorf("delete password: %w", err)
	}

	return nil
}

// readableOwner returns the identity with the given ID as the owner of its password, with its groups. It
// reports an identity that the decider doesn't allow to read like a missing one, with ErrIdentityNotFound,
// so that no endpoint of its password reveals whether it exists.
func (s *passwordService) readableOwner(
	ctx context.Context, decider authz.Decider, identityID uuid.UUID,
) (*authz.Owner, error) {
	groups, err := s.repo.IdentityGroups(ctx, identityID)
	if err != nil {
		return nil, translateError(err)
	}

	can, err := decider.Can(identityservice.Actions().Read(),
		authz.Resource{Type: authz.IdentityType, ID: identityID, Groups: groups})
	if err != nil {
		return nil, fmt.Errorf("check identity read: %w", err)
	}
	if !can {
		return nil, ErrIdentityNotFound
	}

	return &authz.Owner{ID: identityID, Groups: groups}, nil
}

// getReadable returns the record of the owner's password. It reports a password that the decider doesn't
// allow to read like a missing one.
func (s *passwordService) getReadable(
	ctx context.Context, decider authz.Decider, owner *authz.Owner,
) (repository.PasswordRecord, error) {
	rec, err := s.repo.Get(ctx, owner.ID)
	if err != nil {
		return repository.PasswordRecord{}, translateError(err)
	}

	can, err := decider.Can(actions.Read(), toResource(owner))
	if err != nil {
		return repository.PasswordRecord{}, fmt.Errorf("check read: %w", err)
	}
	if !can {
		return repository.PasswordRecord{}, ErrPasswordNotFound
	}

	return rec, nil
}

// toResource returns the owner's password as a resource of a decision. Its ID is the owner's, since an
// identity has at most one password.
func toResource(owner *authz.Owner) authz.Resource {
	return authz.Resource{Type: actions.Type(), ID: owner.ID, Owner: owner}
}

// toPassword builds the password of a record, with the fields that the actor may read. Every other field
// stays hidden. By hand, since the model has no ID of its own to map.
func toPassword(rec repository.PasswordRecord, fields authz.ReadableFields) Password {
	return Password{SetAt: fields.Reveal("setAt", rec.SetAt)}
}
