package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	passwordservice "pkgsilo/feature/password/service"
	passwordmocks "pkgsilo/feature/password/service/mocks"
	"pkgsilo/feature/session/repository"
	"pkgsilo/feature/session/repository/mocks"
	"pkgsilo/test/servicetest"
)

var (
	now        = time.Date(2026, time.November, 17, 12, 0, 0, 0, time.UTC)
	identityID = uuid.MustParse("7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c")
	sessionID  = uuid.MustParse("1b2c3d4e-5f60-4718-89ab-cdef01234567")
	lifetimes  = Lifetimes{IdleTimeout: time.Hour, MaxAge: 24 * time.Hour}
	errDB      = errors.New("connection refused")
)

const presentedToken = "pss_PRESENTEDTOKEN"

// newTestService returns a service on a mock repository and verifier, whose clock stands at now.
func newTestService(t *testing.T) (*sessionService, *mocks.MockSessionRepository, *passwordmocks.MockVerifier) {
	t.Helper()

	repo := mocks.NewMockSessionRepository(t)
	verifier := passwordmocks.NewMockVerifier(t)

	return newSessionService(repo, verifier, lifetimes, func() time.Time { return now }), repo, verifier
}

// hashOf returns the SHA-256 of token.
func hashOf(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}

// storedRecord returns the record of ada's session, created at the given time and last seen at lastSeenAt.
func storedRecord(createdAt, lastSeenAt time.Time) repository.SessionRecord {
	return repository.SessionRecord{
		ID:         sessionID,
		IdentityID: identityID,
		TokenHash:  hashOf(presentedToken),
		CreatedAt:  createdAt,
		LastSeenAt: lastSeenAt,
		ExpiresAt:  createdAt.Add(lifetimes.MaxAge),
		UserAgent:  "Mozilla/5.0",
	}
}

// expectLogin makes the verifier accept ada and the repository delete the expired sessions.
func expectLogin(repo *mocks.MockSessionRepository, verifier *passwordmocks.MockVerifier) {
	verifier.EXPECT().Verify(mock.Anything, "ada", "correct horse").Return(identityID, nil)
	repo.EXPECT().DeleteExpired(mock.Anything, now, now.Add(-time.Hour)).Return(nil)
}

// expectInsert makes the repository store any record, capturing it into rec, and return it as stored.
func expectInsert(repo *mocks.MockSessionRepository, rec *repository.SessionRecord) {
	repo.EXPECT().Insert(mock.Anything, servicetest.Capture(rec)).
		RunAndReturn(func(_ context.Context, r repository.SessionRecord) (repository.SessionRecord, error) {
			return r, nil
		})
}

func TestSessionService_Login(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		ip     netip.Addr
		wantIP *string
	}{
		{name: "known IP", ip: netip.MustParseAddr("192.0.2.1"), wantIP: new("192.0.2.1")},
		{name: "unknown IP", ip: netip.Addr{}, wantIP: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo, verifier := newTestService(t)
			expectLogin(repo, verifier)
			var rec repository.SessionRecord
			expectInsert(repo, &rec)

			got, err := svc.Login(t.Context(), LoginInput{
				Username: "ada", Password: "correct horse", ClientIP: tt.ip, UserAgent: "Mozilla/5.0",
			})
			require.NoError(t, err)

			assert.True(t, strings.HasPrefix(got.Token, "pss_"), "the token has the prefix")
			assert.GreaterOrEqual(t, len(got.Token)-len("pss_"), 52, "rand.Text holds 26 characters of 5 bits each")
			assert.Equal(t, hashOf(got.Token), rec.TokenHash, "only the token's hash is stored")
			assert.Equal(t, repository.SessionRecord{
				ID: rec.ID, IdentityID: identityID, TokenHash: rec.TokenHash, CreatedAt: now, LastSeenAt: now,
				ExpiresAt: now.Add(24 * time.Hour), ClientIP: tt.wantIP, UserAgent: "Mozilla/5.0",
			}, rec)
			assert.Equal(t, Session{
				ID: rec.ID, IdentityID: identityID, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(24 * time.Hour),
			}, got.Session)
		})
	}
}

func TestSessionService_Login_TokensDiffer(t *testing.T) {
	t.Parallel()
	svc, repo, verifier := newTestService(t)
	verifier.EXPECT().Verify(mock.Anything, "ada", "correct horse").Return(identityID, nil).Times(2)
	repo.EXPECT().DeleteExpired(mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)
	var rec repository.SessionRecord
	expectInsert(repo, &rec)
	in := LoginInput{Username: "ada", Password: "correct horse"}

	first, err := svc.Login(t.Context(), in)
	require.NoError(t, err)
	second, err := svc.Login(t.Context(), in)
	require.NoError(t, err)

	assert.NotEqual(t, first.Token, second.Token)
	assert.NotEqual(t, first.ID, second.ID)
}

// TestSessionService_Login_EndsPresentedSession checks that a login ends the session of the token that the
// request carries, so that a planted token can't become logged in, and ignores a value that's no token.
func TestSessionService_Login_EndsPresentedSession(t *testing.T) {
	t.Parallel()
	t.Run("token", func(t *testing.T) {
		t.Parallel()
		svc, repo, verifier := newTestService(t)
		expectLogin(repo, verifier)
		repo.EXPECT().DeleteByHash(mock.Anything, hashOf(presentedToken)).Return(nil)
		var rec repository.SessionRecord
		expectInsert(repo, &rec)

		_, err := svc.Login(t.Context(), LoginInput{
			Username: "ada", Password: "correct horse", PresentedToken: presentedToken,
		})

		require.NoError(t, err)
	})
	t.Run("not a token", func(t *testing.T) {
		t.Parallel()
		// The mock fails the test on a DeleteByHash.
		svc, repo, verifier := newTestService(t)
		expectLogin(repo, verifier)
		var rec repository.SessionRecord
		expectInsert(repo, &rec)

		_, err := svc.Login(t.Context(), LoginInput{
			Username: "ada", Password: "correct horse", PresentedToken: "psk_APIKEY",
		})

		require.NoError(t, err)
	})
}

// TestSessionService_Login_Rejected checks that a rejection of the verifier becomes the service's own, with
// its reason and identity, and that it writes nothing.
func TestSessionService_Login_Rejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		cause *passwordservice.RejectedError
		want  RejectedError
	}{
		{
			name:  "unknown",
			cause: &passwordservice.RejectedError{Reason: passwordservice.ErrUnknownCredentials},
			want:  RejectedError{Reason: ErrUnknownCredentials},
		},
		{
			name:  "invalid",
			cause: &passwordservice.RejectedError{IdentityID: identityID, Reason: passwordservice.ErrInvalidPassword},
			want:  RejectedError{IdentityID: identityID, Reason: ErrInvalidPassword},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The repository mock fails the test on any call.
			svc, _, verifier := newTestService(t)
			verifier.EXPECT().Verify(mock.Anything, "ada", "wrong").
				Return(uuid.Nil(), fmt.Errorf("verify: %w", tt.cause))

			_, err := svc.Login(t.Context(), LoginInput{Username: "ada", Password: "wrong", PresentedToken: presentedToken})

			rejected, ok := errors.AsType[*RejectedError](err)
			require.True(t, ok, "a *RejectedError: %v", err)
			assert.Equal(t, tt.want, *rejected)
			servicetest.AssertTranslated(t, err, tt.want.Reason, tt.cause.Reason)
		})
	}
}

// TestSessionService_Login_IdentityDeleted checks that a login whose identity is deleted after its password
// was verified is rejected like an unknown identity.
func TestSessionService_Login_IdentityDeleted(t *testing.T) {
	t.Parallel()
	svc, repo, verifier := newTestService(t)
	expectLogin(repo, verifier)
	repo.EXPECT().Insert(mock.Anything, mock.Anything).
		Return(repository.SessionRecord{}, fmt.Errorf("insert: %w", repository.ErrIdentityNotFound))

	_, err := svc.Login(t.Context(), LoginInput{Username: "ada", Password: "correct horse"})

	rejected, ok := errors.AsType[*RejectedError](err)
	require.True(t, ok, "a *RejectedError: %v", err)
	assert.Equal(t, RejectedError{IdentityID: identityID, Reason: ErrUnknownCredentials}, *rejected)
	servicetest.AssertTranslated(t, err, ErrUnknownCredentials, repository.ErrIdentityNotFound)
}

func TestSessionService_Login_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		expect func(repo *mocks.MockSessionRepository, verifier *passwordmocks.MockVerifier)
	}{
		{
			name: "verify",
			expect: func(_ *mocks.MockSessionRepository, verifier *passwordmocks.MockVerifier) {
				verifier.EXPECT().Verify(mock.Anything, "ada", "correct horse").Return(uuid.Nil(), errDB)
			},
		},
		{
			name: "delete expired",
			expect: func(repo *mocks.MockSessionRepository, verifier *passwordmocks.MockVerifier) {
				verifier.EXPECT().Verify(mock.Anything, "ada", "correct horse").Return(identityID, nil)
				repo.EXPECT().DeleteExpired(mock.Anything, mock.Anything, mock.Anything).Return(errDB)
			},
		},
		{
			name: "delete presented",
			expect: func(repo *mocks.MockSessionRepository, verifier *passwordmocks.MockVerifier) {
				expectLogin(repo, verifier)
				repo.EXPECT().DeleteByHash(mock.Anything, mock.Anything).Return(errDB)
			},
		},
		{
			name: "insert",
			expect: func(repo *mocks.MockSessionRepository, verifier *passwordmocks.MockVerifier) {
				expectLogin(repo, verifier)
				repo.EXPECT().DeleteByHash(mock.Anything, mock.Anything).Return(nil)
				repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(repository.SessionRecord{}, errDB)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo, verifier := newTestService(t)
			tt.expect(repo, verifier)

			_, err := svc.Login(t.Context(), LoginInput{
				Username: "ada", Password: "correct horse", PresentedToken: presentedToken,
			})

			require.ErrorIs(t, err, errDB)
			assert.NotErrorIs(t, err, ErrUnknownCredentials)
		})
	}
}

func TestSessionService_Authenticate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		lastSeenAt time.Time
		// updated tells that the last use is written.
		updated bool
	}{
		{name: "seen recently", lastSeenAt: now.Add(-lastSeenInterval + time.Microsecond), updated: false},
		{name: "seen a minute ago", lastSeenAt: now.Add(-lastSeenInterval), updated: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on an UpdateLastSeen that it doesn't expect.
			svc, repo, _ := newTestService(t)
			repo.EXPECT().GetByHash(mock.Anything, hashOf(presentedToken)).
				Return(storedRecord(now.Add(-time.Hour), tt.lastSeenAt), nil)
			if tt.updated {
				repo.EXPECT().UpdateLastSeen(mock.Anything, sessionID, now).Return(nil)
			}

			got, err := svc.Authenticate(t.Context(), presentedToken)
			require.NoError(t, err)

			assert.Equal(t, AuthenticatedSession{ID: sessionID, IdentityID: identityID}, got)
		})
	}
}

func TestSessionService_Authenticate_Malformed(t *testing.T) {
	t.Parallel()
	// The mock fails the test on any call.
	svc, _, _ := newTestService(t)

	for _, token := range []string{"", "psk_APIKEY", "PSS_TOKEN"} {
		_, err := svc.Authenticate(t.Context(), token)

		assert.ErrorIs(t, err, ErrMalformedToken, token)
	}
}

func TestSessionService_Authenticate_Unknown(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	repo.EXPECT().GetByHash(mock.Anything, hashOf(presentedToken)).
		Return(repository.SessionRecord{}, fmt.Errorf("get: %w", repository.ErrNotFound))

	_, err := svc.Authenticate(t.Context(), presentedToken)

	servicetest.AssertTranslated(t, err, ErrUnknownToken, repository.ErrNotFound)
}

// TestSessionService_Authenticate_Expired checks both limits at their bounds: a session has expired once
// either is reached.
func TestSessionService_Authenticate_Expired(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rec  repository.SessionRecord
	}{
		{name: "idle", rec: storedRecord(now.Add(-2*time.Hour), now.Add(-time.Hour))},
		{name: "too old", rec: storedRecord(now.Add(-24*time.Hour), now)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on an UpdateLastSeen.
			svc, repo, _ := newTestService(t)
			repo.EXPECT().GetByHash(mock.Anything, hashOf(presentedToken)).Return(tt.rec, nil)

			_, err := svc.Authenticate(t.Context(), presentedToken)

			assert.ErrorIs(t, err, ErrExpiredSession)
		})
	}
}

func TestSessionService_Authenticate_Errors(t *testing.T) {
	t.Parallel()
	t.Run("get", func(t *testing.T) {
		t.Parallel()
		svc, repo, _ := newTestService(t)
		repo.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(repository.SessionRecord{}, errDB)

		_, err := svc.Authenticate(t.Context(), presentedToken)

		require.ErrorIs(t, err, errDB)
		assert.NotErrorIs(t, err, ErrUnknownToken)
	})
	t.Run("update last seen", func(t *testing.T) {
		t.Parallel()
		svc, repo, _ := newTestService(t)
		repo.EXPECT().GetByHash(mock.Anything, mock.Anything).
			Return(storedRecord(now.Add(-time.Hour), now.Add(-time.Hour/2)), nil)
		repo.EXPECT().UpdateLastSeen(mock.Anything, sessionID, now).Return(errDB)

		_, err := svc.Authenticate(t.Context(), presentedToken)

		assert.ErrorIs(t, err, errDB)
	})
}

// TestSessionService_ErrorsHoldNoSecrets checks that no error holds the username, the password or a token.
func TestSessionService_ErrorsHoldNoSecrets(t *testing.T) {
	t.Parallel()
	svc, repo, verifier := newTestService(t)
	verifier.EXPECT().Verify(mock.Anything, "ada", "correct horse").
		Return(uuid.Nil(), &passwordservice.RejectedError{Reason: passwordservice.ErrUnknownCredentials})
	repo.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(storedRecord(now.Add(-48*time.Hour), now), nil)

	_, loginErr := svc.Login(t.Context(), LoginInput{Username: "ada", Password: "correct horse"})
	_, authErr := svc.Authenticate(t.Context(), presentedToken)

	for _, err := range []error{loginErr, authErr} {
		require.Error(t, err)
		for _, secret := range []string{"ada", "horse", "PRESENTEDTOKEN"} {
			assert.NotContains(t, err.Error(), secret)
		}
	}
}

func TestSessionService_Get(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	rec := storedRecord(now.Add(-time.Hour), now)
	repo.EXPECT().Get(mock.Anything, identityID, sessionID).Return(rec, nil)

	got, err := svc.Get(t.Context(), identityID, sessionID)
	require.NoError(t, err)

	assert.Equal(t, Session{
		ID: sessionID, IdentityID: identityID, CreatedAt: rec.CreatedAt, LastSeenAt: now, ExpiresAt: rec.ExpiresAt,
	}, got)
}

func TestSessionService_Get_NotFound(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	repo.EXPECT().Get(mock.Anything, identityID, sessionID).
		Return(repository.SessionRecord{}, fmt.Errorf("get: %w", repository.ErrNotFound))

	_, err := svc.Get(t.Context(), identityID, sessionID)

	servicetest.AssertTranslated(t, err, ErrSessionNotFound, repository.ErrNotFound)
}

func TestSessionService_Delete(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	repo.EXPECT().Delete(mock.Anything, identityID, sessionID).Return(nil)

	require.NoError(t, svc.Delete(t.Context(), identityID, sessionID))
}

func TestSessionService_Delete_NotFound(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	repo.EXPECT().Delete(mock.Anything, identityID, sessionID).Return(fmt.Errorf("delete: %w", repository.ErrNotFound))

	err := svc.Delete(t.Context(), identityID, sessionID)

	servicetest.AssertTranslated(t, err, ErrSessionNotFound, repository.ErrNotFound)
}

func TestSessionService_EndSessions(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	repo.EXPECT().DeleteByIdentity(mock.Anything, identityID).Return(nil)

	require.NoError(t, svc.EndSessions(t.Context(), identityID))
}

func TestSessionService_EndSessions_Fails(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	repo.EXPECT().DeleteByIdentity(mock.Anything, identityID).Return(errDB)

	assert.ErrorIs(t, svc.EndSessions(t.Context(), identityID), errDB)
}
