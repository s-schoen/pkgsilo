package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/authz"
	authzmocks "pkgsilo/core/authz/mocks"
	"pkgsilo/feature/password/repository"
	"pkgsilo/feature/password/repository/mocks"
	"pkgsilo/test/servicetest"
)

var (
	now        = time.Date(2026, time.November, 17, 12, 0, 0, 0, time.UTC)
	actor      = uuid.MustParse("6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b")
	identityID = uuid.MustParse("7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c")

	// identityGroups are the groups that the identity of the password is a member of.
	identityGroups = []uuid.UUID{uuid.MustParse("5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b")}
	// passwordResource is the identity's password as a resource of decisions.
	passwordResource = authz.Resource{
		Type: "Password", ID: identityID, Owner: &authz.Owner{ID: identityID, Groups: identityGroups},
	}
	// allActions are the actions on a password that AllowAll allows.
	allActions = authz.AllowedActions(actions.Allowable())

	errDB = errors.New("connection refused")
)

// currentPassword is the password of stored, whose hash is storedHash.
const currentPassword = "correct horse"

// storedHash is the hash of currentPassword, hashed once for all tests since hashing is slow on purpose.
var storedHash = func() string {
	hash, err := hashPassword(context.Background(), currentPassword)
	if err != nil {
		panic(err)
	}

	return hash
}()

// stored is a record as the repository returns it.
var stored = repository.PasswordRecord{IdentityID: identityID, Hash: storedHash, SetAt: now.Add(-time.Hour)}

// newTestService returns a service that may do everything, on a mock repository and a mock session
// terminator.
func newTestService(
	t *testing.T,
) (*passwordService, *mocks.MockPasswordRepository, *MockSessionTerminator) {
	t.Helper()

	return newTestServiceWith(t, authz.AllowAll())
}

// newTestServiceWith returns a service that asks authorizer, on a mock repository and a mock session
// terminator, whose owner loader finds the password's identity readable. The terminator fails the test if
// sessions end without being expected.
func newTestServiceWith(
	t *testing.T, authorizer authz.Authorizer,
) (*passwordService, *mocks.MockPasswordRepository, *MockSessionTerminator) {
	t.Helper()

	repo := mocks.NewMockPasswordRepository(t)
	terminator := NewMockSessionTerminator(t)
	owners := authzmocks.NewMockOwnerLoader(t)
	owners.EXPECT().ReadableOwner(mock.Anything, mock.Anything, identityID).
		Return(&authz.Owner{ID: identityID, Groups: identityGroups}, nil).Maybe()

	return newPasswordService(repo, authorizer, owners, terminator, func() time.Time { return now }), repo, terminator
}

// failOwner makes the service's owner loader fail with err.
func failOwner(t *testing.T, svc *passwordService, err error) {
	t.Helper()

	owners := authzmocks.NewMockOwnerLoader(t)
	owners.EXPECT().ReadableOwner(mock.Anything, mock.Anything, identityID).Return(nil, err)
	svc.owners = owners
}

// newDeciding returns an authorizer that answers actor's requests with a mock decider.
func newDeciding(t *testing.T) (*authzmocks.MockAuthorizer, *authzmocks.MockDecider) {
	t.Helper()

	authorizer := authzmocks.NewMockAuthorizer(t)
	decider := authzmocks.NewMockDecider(t)
	authorizer.EXPECT().For(mock.Anything, actor).Return(decider, nil)

	return authorizer, decider
}

// expectStored makes the repository return stored, or no password if exists is false.
func expectStored(repo *mocks.MockPasswordRepository, exists bool) {
	if exists {
		repo.EXPECT().Get(mock.Anything, identityID).Return(stored, nil)
		return
	}
	repo.EXPECT().Get(mock.Anything, identityID).
		Return(repository.PasswordRecord{}, fmt.Errorf("get password: %w", repository.ErrNotFound))
}

// expectCan makes the decider answer the action on the password.
func expectCan(decider *authzmocks.MockDecider, action authz.Action, can bool) {
	decider.EXPECT().Can(action, passwordResource).Return(can, nil)
}

// expectSet makes the repository store a password, returning it as stored, and returns the stored record.
func expectSet(repo *mocks.MockPasswordRepository) *repository.PasswordRecord {
	var set repository.PasswordRecord
	repo.EXPECT().Set(mock.Anything, servicetest.Capture(&set)).Return(
		repository.PasswordRecord{IdentityID: identityID, Hash: "stored", SetAt: now}, nil)

	return &set
}

// expectChange makes the repository store a changed password while it still has the hash of stored,
// returning it as stored, and returns the stored record.
func expectChange(repo *mocks.MockPasswordRepository) *repository.PasswordRecord {
	var changed repository.PasswordRecord
	repo.EXPECT().Change(mock.Anything, servicetest.Capture(&changed), storedHash).Return(
		repository.PasswordRecord{IdentityID: identityID, Hash: "changed", SetAt: now}, nil)

	return &changed
}

// expectSessionsEnded makes the terminator end the identity's sessions, and fails the test if that happens
// before the password is written: otherwise someone with the old password could log in before the write and
// keep their session.
func expectSessionsEnded(
	t *testing.T, repo *mocks.MockPasswordRepository, terminator *MockSessionTerminator,
) {
	t.Helper()

	terminator.EXPECT().EndSessions(mock.Anything, identityID).Return(nil).Run(
		func(context.Context, uuid.UUID) {
			writes := []string{"Set", "Change", "Delete"}
			assert.True(t, slices.ContainsFunc(repo.Calls, func(call mock.Call) bool {
				return slices.Contains(writes, call.Method)
			}), "sessions must end after the password is written")
		})
}

// assertHashOf asserts that hash is a hash of password with the current parameters.
func assertHashOf(t *testing.T, password, hash string) {
	t.Helper()

	match, outdated, err := verifyPassword(t.Context(), password, hash)
	require.NoError(t, err)
	assert.True(t, match, "the stored hash must be the new password's")
	assert.False(t, outdated)
}

func TestPasswordService_Declaration(t *testing.T) {
	t.Parallel()

	declaration := Actions().Declaration()

	assert.Equal(t, "Password", declaration.Type)
	assert.True(t, declaration.Owned, "policies must be able to refer to the owner of a password")
	assert.Equal(t, []string{"setAt"}, Actions().ReadFields().Fields())
}

func TestPasswordService_Get(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	expectStored(repo, true)

	got, err := svc.Get(t.Context(), actor, identityID)
	require.NoError(t, err)

	assert.Equal(t, Password{SetAt: authz.Visible(stored.SetAt), AllowedActions: allActions}, got)
}

func TestPasswordService_Get_HiddenFields(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectStored(repo, true)
	expectCan(decider, actions.Read(), true)
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), passwordResource)
	servicetest.ExpectAllowedActions(decider, actions, passwordResource, "password:change")

	got, err := svc.Get(t.Context(), actor, identityID)
	require.NoError(t, err)

	assert.Equal(t, Password{AllowedActions: authz.AllowedActions{"password:change"}}, got)
}

// TestPasswordService_Get_NothingAllowed checks that a password that the actor may read but do nothing to
// tells so with an empty list of allowed actions.
func TestPasswordService_Get_NothingAllowed(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectStored(repo, true)
	expectCan(decider, actions.Read(), true)
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), passwordResource, "setAt")
	servicetest.ExpectAllowedActions(decider, actions, passwordResource)

	got, err := svc.Get(t.Context(), actor, identityID)
	require.NoError(t, err)

	assert.Equal(t, Password{SetAt: authz.Visible(stored.SetAt), AllowedActions: authz.AllowedActions{}}, got)
}

func TestPasswordService_Get_NotFound(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	repoErr := fmt.Errorf("get password: %w", repository.ErrNotFound)
	repo.EXPECT().Get(mock.Anything, identityID).Return(repository.PasswordRecord{}, repoErr)

	_, err := svc.Get(t.Context(), actor, identityID)

	servicetest.AssertTranslated(t, err, ErrPasswordNotFound, repository.ErrNotFound)
}

// TestPasswordService_Set_Reset checks that an actor that may reset a password sets it without the current
// one, whether the identity has a password or not, and ignores a current password that it gives.
func TestPasswordService_Set_Reset(t *testing.T) {
	t.Parallel()
	wrong := "wrong"
	tests := []struct {
		name    string
		exists  bool
		current *string
	}{
		{name: "first password", exists: false},
		{name: "replace", exists: true},
		{name: "wrong current password ignored", exists: true, current: &wrong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mock fails the test if change is checked.
			svc, repo, terminator := newTestServiceWith(t, authorizer)
			expectStored(repo, tt.exists)
			expectCan(decider, actions.Read(), true)
			expectCan(decider, resetAction, true)
			servicetest.ExpectReadableFields(decider, actions.ReadFields(), passwordResource, "setAt")
			servicetest.ExpectAllowedActions(decider, actions, passwordResource, "password:reset")
			set := expectSet(repo)
			expectSessionsEnded(t, repo, terminator)

			got, err := svc.Set(t.Context(), actor, identityID,
				PasswordInput{NewPassword: "battery staple", CurrentPassword: tt.current})
			require.NoError(t, err)

			assert.Equal(t, identityID, set.IdentityID)
			assert.Equal(t, now, set.SetAt)
			assertHashOf(t, "battery staple", set.Hash)
			// The password as stored is returned, with the actions as they were before the write.
			assert.Equal(t, Password{
				SetAt: authz.Visible(now), AllowedActions: authz.AllowedActions{"password:reset"},
			}, got)
		})
	}
}

// TestPasswordService_Set_Change checks that an actor that may change but not reset a password sets it with
// the current one.
func TestPasswordService_Set_Change(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, terminator := newTestServiceWith(t, authorizer)
	expectStored(repo, true)
	expectCan(decider, actions.Read(), true)
	expectCan(decider, resetAction, false)
	expectCan(decider, changeAction, true)
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), passwordResource, "setAt")
	servicetest.ExpectAllowedActions(decider, actions, passwordResource, "password:change")
	changed := expectChange(repo)
	expectSessionsEnded(t, repo, terminator)

	got, err := svc.Set(t.Context(), actor, identityID,
		PasswordInput{NewPassword: "battery staple", CurrentPassword: new(currentPassword)})
	require.NoError(t, err)

	assert.Equal(t, identityID, changed.IdentityID)
	assert.Equal(t, now, changed.SetAt)
	assertHashOf(t, "battery staple", changed.Hash)
	assert.Equal(t, Password{SetAt: authz.Visible(now), AllowedActions: authz.AllowedActions{"password:change"}}, got)
}

// TestPasswordService_Set_ChangedMeanwhile checks that a change fails like a wrong current password, and ends
// no session, when the password is reset or deleted while the current password is checked, so that a change
// never overwrites a reset.
func TestPasswordService_Set_ChangedMeanwhile(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The terminator mock fails the test if sessions end.
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectStored(repo, true)
	expectCan(decider, actions.Read(), true)
	expectCan(decider, resetAction, false)
	expectCan(decider, changeAction, true)
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), passwordResource, "setAt")
	servicetest.ExpectAllowedActions(decider, actions, passwordResource, "password:change")
	repoErr := fmt.Errorf("change password: %w", repository.ErrHashChanged)
	repo.EXPECT().Change(mock.Anything, mock.Anything, storedHash).Return(repository.PasswordRecord{}, repoErr)

	_, err := svc.Set(t.Context(), actor, identityID,
		PasswordInput{NewPassword: "battery staple", CurrentPassword: new(currentPassword)})

	servicetest.AssertTranslated(t, err, ErrWrongCurrentPassword, repository.ErrHashChanged)
}

// TestPasswordService_Set_ChangeRejected checks that a change without the right current password, or of a
// password that doesn't exist, writes nothing.
func TestPasswordService_Set_ChangeRejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		exists  bool
		current *string
		want    error
	}{
		{name: "current password missing", exists: true, want: ErrCurrentPasswordRequired},
		{name: "wrong current password", exists: true, current: new("correct horse "), want: ErrWrongCurrentPassword},
		{name: "empty current password", exists: true, current: new(""), want: ErrWrongCurrentPassword},
		{name: "no password", exists: false, current: new(currentPassword), want: ErrPasswordNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mock fails the test on a write.
			svc, repo, _ := newTestServiceWith(t, authorizer)
			expectStored(repo, tt.exists)
			expectCan(decider, actions.Read(), true)
			expectCan(decider, resetAction, false)
			decider.EXPECT().Can(changeAction, passwordResource).Return(true, nil).Maybe()
			decider.EXPECT().Can(mock.Anything, passwordResource).Return(true, nil).Maybe()
			decider.EXPECT().Hint(mock.Anything, passwordResource).Return(true, nil).Maybe()

			_, err := svc.Set(t.Context(), actor, identityID,
				PasswordInput{NewPassword: "battery staple", CurrentPassword: tt.current})

			assert.ErrorIs(t, err, tt.want)
		})
	}
}

// TestPasswordService_Set_Denied checks that an actor that may neither reset nor change a password it may
// read is denied, before the current password is verified.
func TestPasswordService_Set_Denied(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mock fails the test on a write.
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectStored(repo, true)
	expectCan(decider, actions.Read(), true)
	expectCan(decider, resetAction, false)
	expectCan(decider, changeAction, false)

	_, err := svc.Set(t.Context(), actor, identityID,
		PasswordInput{NewPassword: "battery staple", CurrentPassword: new(currentPassword)})

	assert.ErrorIs(t, err, authz.ErrDenied)
}

// TestPasswordService_Set_Unreadable checks that a password that the actor may not read is reported like a
// missing one, before reset or change is checked, and that a first password that an actor may reset but not
// read is set and returned with no field and no allowed actions.
func TestPasswordService_Set_Unreadable(t *testing.T) {
	t.Parallel()

	t.Run("existing", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		// The mocks fail the test on any other decision or on a write.
		svc, repo, _ := newTestServiceWith(t, authorizer)
		expectStored(repo, true)
		expectCan(decider, actions.Read(), false)

		_, err := svc.Set(t.Context(), actor, identityID, PasswordInput{NewPassword: "battery staple"})

		assert.ErrorIs(t, err, ErrPasswordNotFound)
	})
	t.Run("first password", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		// The mock fails the test if a field or an allowed action is checked.
		svc, repo, terminator := newTestServiceWith(t, authorizer)
		expectStored(repo, false)
		expectCan(decider, actions.Read(), false)
		expectCan(decider, resetAction, true)
		expectSet(repo)
		expectSessionsEnded(t, repo, terminator)

		got, err := svc.Set(t.Context(), actor, identityID, PasswordInput{NewPassword: "battery staple"})
		require.NoError(t, err)

		assert.Equal(t, Password{}, got)
	})
}

func TestPasswordService_Set_IdentityDeletedMeanwhile(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newTestService(t)
	expectStored(repo, false)
	repoErr := fmt.Errorf("set password: %w", repository.ErrIdentityNotFound)
	repo.EXPECT().Set(mock.Anything, mock.Anything).Return(repository.PasswordRecord{}, repoErr)

	_, err := svc.Set(t.Context(), actor, identityID, PasswordInput{NewPassword: "battery staple"})

	servicetest.AssertTranslated(t, err, ErrIdentityNotFound, repository.ErrIdentityNotFound)
}

// TestPasswordService_Set_Canceled checks that a request that's canceled before it hashes writes nothing.
func TestPasswordService_Set_Canceled(t *testing.T) {
	t.Parallel()
	// The mock fails the test on a write.
	svc, repo, _ := newTestService(t)
	expectStored(repo, false)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := svc.Set(ctx, actor, identityID, PasswordInput{NewPassword: "battery staple"})

	assert.ErrorIs(t, err, context.Canceled)
}

func TestPasswordService_Delete(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo, terminator := newTestServiceWith(t, authorizer)
	expectStored(repo, true)
	expectCan(decider, actions.Read(), true)
	expectCan(decider, actions.Delete(), true)
	repo.EXPECT().Delete(mock.Anything, identityID).Return(nil)
	expectSessionsEnded(t, repo, terminator)

	assert.NoError(t, svc.Delete(t.Context(), actor, identityID))
}

// TestPasswordService_EndSessionsFails checks that a change, reset or delete fails when the identity's
// sessions can't be ended, after the password was written, so that the request can be retried.
func TestPasswordService_EndSessionsFails(t *testing.T) {
	t.Parallel()
	errEnd := errors.New("sessions not ended")
	tests := map[string]struct {
		expectWrite func(*mocks.MockPasswordRepository)
		call        func(*passwordService) error
	}{
		"set": {
			expectWrite: func(repo *mocks.MockPasswordRepository) { expectSet(repo) },
			call: func(svc *passwordService) error {
				_, err := svc.Set(t.Context(), actor, identityID, PasswordInput{NewPassword: "battery staple"})
				return err
			},
		},
		"delete": {
			expectWrite: func(repo *mocks.MockPasswordRepository) {
				repo.EXPECT().Delete(mock.Anything, identityID).Return(nil)
			},
			call: func(svc *passwordService) error { return svc.Delete(t.Context(), actor, identityID) },
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, repo, terminator := newTestService(t)
			expectStored(repo, true)
			tt.expectWrite(repo)
			terminator.EXPECT().EndSessions(mock.Anything, identityID).Return(errEnd)

			assert.ErrorIs(t, tt.call(svc), errEnd)
		})
	}
}

func TestPasswordService_Delete_Denied(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mock fails the test on a write.
	svc, repo, _ := newTestServiceWith(t, authorizer)
	expectStored(repo, true)
	expectCan(decider, actions.Read(), true)
	expectCan(decider, actions.Delete(), false)

	assert.ErrorIs(t, svc.Delete(t.Context(), actor, identityID), authz.ErrDenied)
}

func TestPasswordService_Delete_NotFound(t *testing.T) {
	t.Parallel()

	t.Run("no password", func(t *testing.T) {
		t.Parallel()
		svc, repo, _ := newTestService(t)
		expectStored(repo, false)

		err := svc.Delete(t.Context(), actor, identityID)

		servicetest.AssertTranslated(t, err, ErrPasswordNotFound, repository.ErrNotFound)
	})
	t.Run("deleted meanwhile", func(t *testing.T) {
		t.Parallel()
		svc, repo, _ := newTestService(t)
		expectStored(repo, true)
		repo.EXPECT().Delete(mock.Anything, identityID).Return(fmt.Errorf("delete password: %w", repository.ErrNotFound))

		err := svc.Delete(t.Context(), actor, identityID)

		servicetest.AssertTranslated(t, err, ErrPasswordNotFound, repository.ErrNotFound)
	})
}

// calls returns a call of each method of the service on the identity's password.
func calls(t *testing.T) map[string]func(*passwordService) error {
	t.Helper()

	return map[string]func(*passwordService) error{
		"get": func(svc *passwordService) error {
			_, err := svc.Get(t.Context(), actor, identityID)
			return err
		},
		"set": func(svc *passwordService) error {
			_, err := svc.Set(t.Context(), actor, identityID,
				PasswordInput{NewPassword: "battery staple", CurrentPassword: new(currentPassword)})
			return err
		},
		"delete": func(svc *passwordService) error {
			return svc.Delete(t.Context(), actor, identityID)
		},
	}
}

// TestPasswordService_IdentityNotFound checks that every method reports an identity that the owner loader
// doesn't find, since it doesn't exist or the actor may not read it, with ErrIdentityNotFound, before anything
// else is looked up or decided.
func TestPasswordService_IdentityNotFound(t *testing.T) {
	t.Parallel()
	for name, call := range calls(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, _ := newDeciding(t)
			// The mocks fail the test on any decision or repository call.
			svc, _, _ := newTestServiceWith(t, authorizer)
			failOwner(t, svc, authz.ErrOwnerNotFound)

			err := call(svc)

			servicetest.AssertTranslated(t, err, ErrIdentityNotFound, authz.ErrOwnerNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
	}
}

// TestPasswordService_PasswordHidden checks that every method reports a password that the actor may not read
// like a missing one, before any other action is checked.
func TestPasswordService_PasswordHidden(t *testing.T) {
	t.Parallel()
	for name, call := range calls(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mocks fail the test on any other decision or on a write.
			svc, repo, _ := newTestServiceWith(t, authorizer)
			expectStored(repo, true)
			expectCan(decider, actions.Read(), false)

			err := call(svc)

			require.ErrorIs(t, err, ErrPasswordNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
	}
}

// TestPasswordService_RepositoryFails checks that every method wraps an unexpected repository error, without
// reporting it as a missing identity or password.
func TestPasswordService_RepositoryFails(t *testing.T) {
	t.Parallel()
	for name, call := range calls(t) {
		t.Run(name+" identity lookup", func(t *testing.T) {
			t.Parallel()
			svc, _, _ := newTestService(t)
			failOwner(t, svc, errDB)

			err := call(svc)

			require.ErrorIs(t, err, errDB)
			assert.NotErrorIs(t, err, ErrIdentityNotFound)
		})
		t.Run(name+" password lookup", func(t *testing.T) {
			t.Parallel()
			svc, repo, _ := newTestService(t)
			repo.EXPECT().Get(mock.Anything, identityID).Return(repository.PasswordRecord{}, errDB)

			err := call(svc)

			require.ErrorIs(t, err, errDB)
			assert.NotErrorIs(t, err, ErrPasswordNotFound)
		})
	}
}

// TestPasswordService_AuthorizationFails checks that failing to decide is an error and never an answer: not
// access, and not a missing identity or password, which would hide a broken policy from the logs.
func TestPasswordService_AuthorizationFails(t *testing.T) {
	t.Parallel()
	errDecision := errors.New("policy failed")
	assertFailed := func(t *testing.T, err error) {
		t.Helper()
		require.ErrorIs(t, err, errDecision)
		require.NotErrorIs(t, err, ErrIdentityNotFound)
		require.NotErrorIs(t, err, ErrPasswordNotFound)
		assert.NotErrorIs(t, err, authz.ErrDenied)
	}

	for name, call := range calls(t) {
		t.Run(name+" no decider", func(t *testing.T) {
			t.Parallel()
			authorizer := authzmocks.NewMockAuthorizer(t)
			authorizer.EXPECT().For(mock.Anything, actor).Return(nil, errDecision)
			// The mock fails the test if the repository is called.
			svc, _, _ := newTestServiceWith(t, authorizer)

			assertFailed(t, call(svc))
		})
		t.Run(name+" no identity decision", func(t *testing.T) {
			t.Parallel()
			authorizer, _ := newDeciding(t)
			// The mocks fail the test on any decision or repository call.
			svc, _, _ := newTestServiceWith(t, authorizer)
			failOwner(t, svc, fmt.Errorf("check owner read: %w", errDecision))

			assertFailed(t, call(svc))
		})
		// Every decision on the password in turn, after the ones before it allowed the request. They all come
		// before any write, so that a failed one can't fail a request whose write succeeded.
		for _, failing := range []authz.Action{
			actions.Read(), resetAction, changeAction, actions.Delete(), actions.ReadFields().Actions()[0],
		} {
			t.Run(fmt.Sprintf("%s no decision %v", name, failing), func(t *testing.T) {
				t.Parallel()
				authorizer, decider := newDeciding(t)
				svc, repo, terminator := newTestServiceWith(t, authorizer)
				expectStored(repo, true)
				terminator.EXPECT().EndSessions(mock.Anything, identityID).Return(nil).Maybe()
				decider.EXPECT().Can(failing, passwordResource).Return(false, errDecision).Maybe()
				// Reset is denied, so that set checks change too.
				decider.EXPECT().Can(resetAction, passwordResource).Return(false, nil).Maybe()
				decider.EXPECT().Can(mock.Anything, passwordResource).Return(true, nil).Maybe()
				decider.EXPECT().Hint(mock.Anything, passwordResource).Return(true, nil).Maybe()
				wrote := false
				repo.EXPECT().Set(mock.Anything, mock.Anything).RunAndReturn(
					func(context.Context, repository.PasswordRecord) (repository.PasswordRecord, error) {
						wrote = true
						return stored, nil
					}).Maybe()
				repo.EXPECT().Change(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
					func(context.Context, repository.PasswordRecord, string) (repository.PasswordRecord, error) {
						wrote = true
						return stored, nil
					}).Maybe()
				repo.EXPECT().Delete(mock.Anything, identityID).RunAndReturn(func(context.Context, uuid.UUID) error {
					wrote = true
					return nil
				}).Maybe()

				err := call(svc)
				if err == nil {
					// The method doesn't make this decision.
					return
				}
				assertFailed(t, err)
				assert.False(t, wrote, "nothing may be written after a failed decision")
			})
		}
	}

	// The allowed action decisions, after every other decision allowed the request. They come before any
	// write too.
	for _, name := range []string{"get", "set"} {
		t.Run(name+" no allowed action decision", func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mock fails the test on a write.
			svc, repo, _ := newTestServiceWith(t, authorizer)
			expectStored(repo, true)
			decider.EXPECT().Can(mock.Anything, passwordResource).Return(true, nil).Maybe()
			decider.EXPECT().Hint(mock.Anything, passwordResource).Return(false, errDecision)

			assertFailed(t, calls(t)[name](svc))
		})
	}
}
