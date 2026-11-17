package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/feature/password/repository"
	"pkgsilo/feature/password/repository/mocks"
	"pkgsilo/test/servicetest"
)

// newTestVerifier returns a verifier on a mock repository.
func newTestVerifier(t *testing.T) (Verifier, *mocks.MockPasswordRepository) {
	t.Helper()

	repo := mocks.NewMockPasswordRepository(t)

	return NewVerifier(repo), repo
}

// expectCredentials makes the repository return the identity with the username ada and the given hash.
func expectCredentials(repo *mocks.MockPasswordRepository, hash *string) {
	repo.EXPECT().GetCredentials(mock.Anything, "ada").
		Return(repository.CredentialsRecord{IdentityID: identityID, Hash: hash}, nil)
}

func TestVerifier_Verify(t *testing.T) {
	t.Parallel()
	// The mock fails the test on a rehash, which a current hash doesn't need.
	verifier, repo := newTestVerifier(t)
	expectCredentials(repo, new(storedHash))

	got, err := verifier.Verify(t.Context(), "ada", currentPassword)
	require.NoError(t, err)

	assert.Equal(t, identityID, got)
}

// TestVerifier_Verify_Rejected checks that a rejection tells its reason and the identity, if one has the
// username, and returns no identity to authenticate as.
func TestVerifier_Verify_Rejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expect   func(repo *mocks.MockPasswordRepository)
		password string
		want     RejectedError
	}{
		{
			name: "unknown identity",
			expect: func(repo *mocks.MockPasswordRepository) {
				repo.EXPECT().GetCredentials(mock.Anything, "ada").Return(repository.CredentialsRecord{},
					fmt.Errorf("get credentials: %w", repository.ErrIdentityNotFound))
			},
			password: currentPassword,
			want:     RejectedError{IdentityID: uuid.Nil(), Reason: ErrUnknownCredentials},
		},
		{
			name:     "no password",
			expect:   func(repo *mocks.MockPasswordRepository) { expectCredentials(repo, nil) },
			password: currentPassword,
			want:     RejectedError{IdentityID: identityID, Reason: ErrUnknownCredentials},
		},
		{
			name:     "wrong password",
			expect:   func(repo *mocks.MockPasswordRepository) { expectCredentials(repo, new(storedHash)) },
			password: "correct horse ",
			want:     RejectedError{IdentityID: identityID, Reason: ErrInvalidPassword},
		},
		{
			name:     "empty password",
			expect:   func(repo *mocks.MockPasswordRepository) { expectCredentials(repo, new(storedHash)) },
			password: "",
			want:     RejectedError{IdentityID: identityID, Reason: ErrInvalidPassword},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on a rehash.
			verifier, repo := newTestVerifier(t)
			tt.expect(repo)

			got, err := verifier.Verify(t.Context(), "ada", tt.password)

			assert.Equal(t, uuid.Nil(), got)
			rejected, ok := errors.AsType[*RejectedError](err)
			require.True(t, ok, "%v is no *RejectedError", err)
			assert.Equal(t, tt.want, *rejected)
			require.ErrorIs(t, err, tt.want.Reason)
			assert.NotContains(t, err.Error(), "ada")
			assert.NotContains(t, err.Error(), "horse")
		})
	}
}

// TestVerifier_Verify_UnknownStillHashes checks that a username without a password is checked with a hash too,
// so that it takes as long as a wrong password: a canceled request fails before it hashes, instead of being
// rejected.
func TestVerifier_Verify_UnknownStillHashes(t *testing.T) {
	t.Parallel()
	tests := map[string]func(repo *mocks.MockPasswordRepository){
		"unknown identity": func(repo *mocks.MockPasswordRepository) {
			repo.EXPECT().GetCredentials(mock.Anything, "ada").Return(repository.CredentialsRecord{},
				fmt.Errorf("get credentials: %w", repository.ErrIdentityNotFound))
		},
		"no password": func(repo *mocks.MockPasswordRepository) { expectCredentials(repo, nil) },
	}
	for name, expect := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			verifier, repo := newTestVerifier(t)
			expect(repo)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			_, err := verifier.Verify(ctx, "ada", currentPassword)

			require.ErrorIs(t, err, context.Canceled)
			require.NotErrorIs(t, err, ErrUnknownCredentials)
		})
	}
}

// TestVerifier_Verify_Rehash checks that a password whose hash has outdated parameters is rehashed with the
// current ones once it's verified, in place of the old hash only.
func TestVerifier_Verify_Rehash(t *testing.T) {
	t.Parallel()
	verifier, repo := newTestVerifier(t)
	outdated := hashWith(outdatedParams)
	expectCredentials(repo, &outdated)
	var newHash string
	repo.EXPECT().UpdateHash(mock.Anything, identityID, outdated, servicetest.Capture(&newHash)).Return(true, nil)

	got, err := verifier.Verify(t.Context(), "ada", currentPassword)
	require.NoError(t, err)

	assert.Equal(t, identityID, got)
	assertHashOf(t, currentPassword, newHash)
}

// TestVerifier_Verify_ChangedMeanwhile checks that a password that changed between being read and rehashed
// still verifies: the change wins, and the login used the password that was current when it was read.
func TestVerifier_Verify_ChangedMeanwhile(t *testing.T) {
	t.Parallel()
	verifier, repo := newTestVerifier(t)
	outdated := hashWith(outdatedParams)
	expectCredentials(repo, &outdated)
	repo.EXPECT().UpdateHash(mock.Anything, identityID, outdated, mock.Anything).Return(false, nil)

	got, err := verifier.Verify(t.Context(), "ada", currentPassword)
	require.NoError(t, err)

	assert.Equal(t, identityID, got)
}

// TestVerifier_Verify_WrongPasswordIsNotRehashed checks that only a verified password is rehashed.
func TestVerifier_Verify_WrongPasswordIsNotRehashed(t *testing.T) {
	t.Parallel()
	// The mock fails the test on a rehash.
	verifier, repo := newTestVerifier(t)
	outdated := hashWith(outdatedParams)
	expectCredentials(repo, &outdated)

	_, err := verifier.Verify(t.Context(), "ada", "wrong")

	assert.ErrorIs(t, err, ErrInvalidPassword)
}

func TestVerifier_Verify_Errors(t *testing.T) {
	t.Parallel()
	outdated := hashWith(outdatedParams)
	tests := []struct {
		name   string
		expect func(repo *mocks.MockPasswordRepository)
		want   error
	}{
		{
			name: "lookup fails",
			expect: func(repo *mocks.MockPasswordRepository) {
				repo.EXPECT().GetCredentials(mock.Anything, "ada").Return(repository.CredentialsRecord{}, errDB)
			},
			want: errDB,
		},
		{
			name:   "malformed hash",
			expect: func(repo *mocks.MockPasswordRepository) { expectCredentials(repo, new("$2a$10$bcrypt")) },
			want:   errMalformedHash,
		},
		{
			name: "rehash fails",
			expect: func(repo *mocks.MockPasswordRepository) {
				expectCredentials(repo, &outdated)
				repo.EXPECT().UpdateHash(mock.Anything, identityID, outdated, mock.Anything).Return(false, errDB)
			},
			want: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			verifier, repo := newTestVerifier(t)
			tt.expect(repo)

			got, err := verifier.Verify(t.Context(), "ada", currentPassword)

			assert.Equal(t, uuid.Nil(), got)
			require.ErrorIs(t, err, tt.want)
			require.NotErrorIs(t, err, ErrUnknownCredentials)
			assert.NotErrorIs(t, err, ErrInvalidPassword)
		})
	}
}
