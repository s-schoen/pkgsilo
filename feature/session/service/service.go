// Package service implements the session domain: logging in with a password, and the sessions that keep a
// browser logged in afterwards.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	passwordservice "pkgsilo/feature/password/service"
	"pkgsilo/feature/session/repository"
)

const (
	// tokenPrefix starts every token, so that a token can be told apart from other values, by secret scanners
	// too.
	tokenPrefix = "pss_"
	// lastSeenInterval is the least time between two writes of a session's last use, so that a busy session
	// doesn't cause a write per request. The idle timeout is accurate to about that much.
	lastSeenInterval = time.Minute
)

// SessionService logs identities in with their passwords and authenticates the tokens of their sessions.
//
// No method checks a policy, which is a deliberate exception: logging in is authentication, like presenting
// an API key, and an identity always sees and ends the session that its request is authenticated with, so
// that no policy can block a logout. Get and Delete see only the actor's own sessions.
type SessionService interface {
	// Login checks the input's username and password and returns a new session of their identity, with its
	// token. It first ends every expired session, and the session of the input's presented token. It returns
	// a *RejectedError whose reason is ErrUnknownCredentials or ErrInvalidPassword if the username and
	// password aren't accepted. It isn't authorized, since a request has no actor before it logs in. Its
	// errors never hold the username, the password or a token.
	Login(ctx context.Context, in LoginInput) (CreatedSession, error)
	// Authenticate returns the session that the presented token belongs to and records its use. It returns
	// an error wrapping ErrMalformedToken, ErrUnknownToken or ErrExpiredSession if the token isn't accepted.
	// Its errors never hold the token. It isn't authorized, since a request has no actor before it's
	// authenticated.
	Authenticate(ctx context.Context, token string) (AuthenticatedSession, error)
	// Get returns the actor's session with the given ID, or ErrSessionNotFound if the actor has none.
	Get(ctx context.Context, actor, id uuid.UUID) (Session, error)
	// Delete ends the actor's session with the given ID, whose token stops working at once. It returns
	// ErrSessionNotFound if the actor has none.
	Delete(ctx context.Context, actor, id uuid.UUID) error
	// EndSessions ends every session of the identity, whose tokens stop working at once. An identity without
	// sessions is not an error. It isn't authorized: the password service calls it after it authorized the
	// change of the identity's password, as the session terminator that it owns.
	EndSessions(ctx context.Context, identityID uuid.UUID) error
}

var _ passwordservice.SessionTerminator = SessionService(nil)

type sessionService struct {
	repo      repository.SessionRepository
	verifier  passwordservice.Verifier
	lifetimes Lifetimes
	now       func() time.Time
}

// NewSessionService returns a SessionService that stores sessions in repo, checks passwords with verifier and
// ends sessions after lifetimes.
func NewSessionService(
	repo repository.SessionRepository, verifier passwordservice.Verifier, lifetimes Lifetimes,
) SessionService {
	return newSessionService(repo, verifier, lifetimes, time.Now)
}

func newSessionService(
	repo repository.SessionRepository, verifier passwordservice.Verifier, lifetimes Lifetimes, now func() time.Time,
) *sessionService {
	return &sessionService{repo: repo, verifier: verifier, lifetimes: lifetimes, now: now}
}

func (s *sessionService) Login(ctx context.Context, in LoginInput) (CreatedSession, error) {
	identityID, err := s.verifier.Verify(ctx, in.Username, in.Password)
	if err != nil {
		return CreatedSession{}, fmt.Errorf("log in: %w", translateVerifyError(err))
	}

	now := s.now()
	if err := s.repo.DeleteExpired(ctx, now, now.Add(-s.lifetimes.IdleTimeout)); err != nil {
		return CreatedSession{}, fmt.Errorf("log in: %w", err)
	}
	// Any token with the prefix, whether its session is valid or not: deleting what doesn't exist is cheap.
	if strings.HasPrefix(in.PresentedToken, tokenPrefix) {
		if err := s.repo.DeleteByHash(ctx, hash(in.PresentedToken)); err != nil {
			return CreatedSession{}, fmt.Errorf("log in: %w", err)
		}
	}

	token := generateToken()
	rec := repository.SessionRecord{
		ID:         uuid.NewV4(),
		IdentityID: identityID,
		TokenHash:  hash(token),
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(s.lifetimes.MaxAge),
		UserAgent:  in.UserAgent,
	}
	if in.ClientIP.IsValid() {
		rec.ClientIP = new(in.ClientIP.String())
	}
	stored, err := s.repo.Insert(ctx, rec)
	if errors.Is(err, repository.ErrIdentityNotFound) {
		// The identity was deleted since its password was verified.
		return CreatedSession{}, fmt.Errorf("log in: %w",
			&RejectedError{IdentityID: identityID, Reason: ErrUnknownCredentials})
	}
	if err != nil {
		return CreatedSession{}, fmt.Errorf("log in: %w", err)
	}

	return CreatedSession{Session: toSession(stored), Token: token}, nil
}

func (s *sessionService) Authenticate(ctx context.Context, token string) (AuthenticatedSession, error) {
	if !strings.HasPrefix(token, tokenPrefix) {
		return AuthenticatedSession{}, ErrMalformedToken
	}

	rec, err := s.repo.GetByHash(ctx, hash(token))
	if errors.Is(err, repository.ErrNotFound) {
		return AuthenticatedSession{}, ErrUnknownToken
	}
	if err != nil {
		return AuthenticatedSession{}, fmt.Errorf("authenticate session: %w", err)
	}

	now := s.now()
	if !now.Before(rec.ExpiresAt) || !now.Before(rec.LastSeenAt.Add(s.lifetimes.IdleTimeout)) {
		return AuthenticatedSession{}, fmt.Errorf("session %s: %w", rec.ID, ErrExpiredSession)
	}

	if now.Sub(rec.LastSeenAt) >= lastSeenInterval {
		if err := s.repo.UpdateLastSeen(ctx, rec.ID, now); err != nil {
			return AuthenticatedSession{}, fmt.Errorf("authenticate session: %w", err)
		}
	}

	return AuthenticatedSession{ID: rec.ID, IdentityID: rec.IdentityID}, nil
}

func (s *sessionService) Get(ctx context.Context, actor, id uuid.UUID) (Session, error) {
	rec, err := s.repo.Get(ctx, actor, id)
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", translateError(err))
	}

	return toSession(rec), nil
}

func (s *sessionService) Delete(ctx context.Context, actor, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, actor, id); err != nil {
		return fmt.Errorf("delete session: %w", translateError(err))
	}

	return nil
}

func (s *sessionService) EndSessions(ctx context.Context, identityID uuid.UUID) error {
	if err := s.repo.DeleteByIdentity(ctx, identityID); err != nil {
		return fmt.Errorf("end sessions: %w", err)
	}

	return nil
}

// translateVerifyError replaces a rejection of the verifier with the service's own.
func translateVerifyError(err error) error {
	rejected, ok := errors.AsType[*passwordservice.RejectedError](err)
	if !ok {
		return err
	}

	reason := ErrInvalidPassword
	if errors.Is(rejected.Reason, passwordservice.ErrUnknownCredentials) {
		reason = ErrUnknownCredentials
	}

	return &RejectedError{IdentityID: rejected.IdentityID, Reason: reason}
}

// toSession builds the session of a record.
func toSession(rec repository.SessionRecord) Session {
	return Session{
		ID:         rec.ID,
		IdentityID: rec.IdentityID,
		CreatedAt:  rec.CreatedAt,
		LastSeenAt: rec.LastSeenAt,
		ExpiresAt:  rec.ExpiresAt,
	}
}

// hash returns the hash of token that's stored and looked up. It's deterministic, so that a token can be
// found by it, which is safe without a slow password hash because tokens can't be guessed.
func hash(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

// generateToken returns a new token: the prefix and at least 256 random bits, since rand.Text holds at least
// 128.
func generateToken() string {
	return tokenPrefix + rand.Text() + rand.Text()
}
