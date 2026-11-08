package service

import (
	"context"
	"encoding/hex"
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
	"pkgsilo/core/pagination"
	"pkgsilo/feature/apikey/repository"
	"pkgsilo/feature/apikey/repository/mocks"
	"pkgsilo/test/servicetest"
)

var (
	now        = time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	actor      = uuid.MustParse("6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b")
	identityID = uuid.MustParse("7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c")
	pepper     = []byte("test-pepper-of-at-least-32-characters")
	expiresAt  = now.Add(24 * time.Hour)
	lastUsedAt = now.Add(-30 * time.Second)
	// stored is a record as the repository returns it, with a distinct value in every field.
	stored = repository.APIKeyRecord{
		ID:         uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		IdentityID: identityID,
		Name:       "ci",
		KeyHash:    []byte("hash"),
		KeySuffix:  "WXYZ",
		ExpiresAt:  &expiresAt,
		LastUsedAt: &lastUsedAt,
		CreatedBy:  uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"),
		CreatedAt:  now.Add(-time.Hour),
	}
	// storedKey is stored, translated to the domain model, with every field visible.
	storedKey = APIKey{
		ID:         stored.ID,
		IdentityID: authz.Visible(identityID),
		Name:       authz.Visible("ci"),
		KeySuffix:  authz.Visible("WXYZ"),
		ExpiresAt:  authz.Visible(&expiresAt),
		LastUsedAt: authz.Visible(&lastUsedAt),
		CreatedBy:  authz.Visible(stored.CreatedBy),
		CreatedAt:  authz.Visible(stored.CreatedAt),
	}

	// identityGroups are the groups that the identity of the API keys is a member of.
	identityGroups = []uuid.UUID{uuid.MustParse("5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b")}
	// identityResource is the identity of the API keys as a resource of decisions, with its groups.
	identityResource = authz.Resource{Type: "Identity", ID: identityID, Groups: identityGroups}
	// owner is the identity of the API keys as their owner, with its groups.
	owner = &authz.Owner{ID: identityID, Groups: identityGroups}
	// storedResource is stored as a resource of decisions.
	storedResource = authz.Resource{Type: "APIKey", ID: stored.ID, Owner: owner}
	// readIdentity is the action of reading an identity, as the identity feature declares it.
	readIdentity = authz.NewAction("identity:read", "read")

	errDB = errors.New("connection refused")
)

// newTestService returns a service that may do everything, on a mock repository.
func newTestService(t *testing.T) (*apiKeyService, *mocks.MockAPIKeyRepository) {
	t.Helper()

	return newTestServiceWith(t, authz.AllowAll())
}

// newTestServiceWith returns a service that asks authorizer, on a mock repository.
func newTestServiceWith(t *testing.T, authorizer authz.Authorizer) (*apiKeyService, *mocks.MockAPIKeyRepository) {
	t.Helper()

	repo := mocks.NewMockAPIKeyRepository(t)

	return newAPIKeyService(repo, authorizer, pepper, func() time.Time { return now }), repo
}

// newDeciding returns an authorizer that answers actor's requests with a mock decider.
func newDeciding(t *testing.T) (*authzmocks.MockAuthorizer, *authzmocks.MockDecider) {
	t.Helper()

	authorizer := authzmocks.NewMockAuthorizer(t)
	decider := authzmocks.NewMockDecider(t)
	authorizer.EXPECT().For(mock.Anything, actor).Return(decider, nil)

	return authorizer, decider
}

// expectOwner makes the repository return the groups of the API keys' identity.
func expectOwner(repo *mocks.MockAPIKeyRepository) {
	repo.EXPECT().IdentityGroups(mock.Anything, identityID).Return(identityGroups, nil)
}

// expectOwnerReadable makes the repository return the groups of the API keys' identity, and the decider
// allow reading it.
func expectOwnerReadable(repo *mocks.MockAPIKeyRepository, decider *authzmocks.MockDecider) {
	expectOwner(repo)
	decider.EXPECT().Can(readIdentity, identityResource).Return(true, nil)
}

// expectReadable makes the repository return the API keys' identity and stored, and the decider allow
// reading both.
func expectReadable(repo *mocks.MockAPIKeyRepository, decider *authzmocks.MockDecider) {
	expectOwnerReadable(repo, decider)
	repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(stored, nil)
	decider.EXPECT().Can(actions.Read(), storedResource).Return(true, nil)
}

// expectAllFieldsReadable makes the decider allow reading every field of the resource, once each.
func expectAllFieldsReadable(decider *authzmocks.MockDecider, resource authz.Resource) {
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), resource, actions.ReadFields().Fields()...)
}

// isReadField reports whether the action is one of reading a field of an API key.
func isReadField(action authz.Action) bool {
	return slices.ContainsFunc(actions.ReadFields().Actions(), func(a authz.Action) bool {
		return assert.ObjectsAreEqual(a, action)
	})
}

// newRecords returns stored records with the given IDs' last bytes, in order.
func newRecords(lastBytes ...byte) []repository.APIKeyRecord {
	recs := make([]repository.APIKeyRecord, len(lastBytes))
	for i, lastByte := range lastBytes {
		recs[i] = stored
		recs[i].ID = uuid.UUID{15: lastByte}
	}

	return recs
}

// idsOf returns the IDs' last bytes of the API keys, in order.
func idsOf(keys []APIKey) []byte {
	ids := make([]byte, len(keys))
	for i, key := range keys {
		ids[i] = key.ID[15]
	}

	return ids
}

func TestAPIKeyService_Create(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	expectOwner(repo)
	var inserted repository.APIKeyRecord
	repo.EXPECT().Insert(mock.Anything, servicetest.Capture(&inserted)).Return(stored, nil)

	got, err := svc.Create(t.Context(), actor, identityID, APIKeyInput{Name: "ci", ExpiresAt: &expiresAt})
	require.NoError(t, err)

	assert.Equal(t, byte(4), inserted.ID[6]>>4, "ID must be a version 4 UUID")
	assert.Regexp(t, `^psk_[A-Z2-7]{52}$`, got.Key)
	assert.Equal(t, repository.APIKeyRecord{
		ID:         inserted.ID,
		IdentityID: identityID,
		Name:       "ci",
		KeyHash:    svc.hash(got.Key),
		KeySuffix:  got.Key[len(got.Key)-4:],
		ExpiresAt:  &expiresAt,
		CreatedBy:  actor,
		CreatedAt:  now,
	}, inserted)
	// The API key as stored is returned, not the one sent to the repository.
	assert.Equal(t, storedKey, got.APIKey)
}

func TestAPIKeyService_Create_GeneratesUniqueKeysAndIDs(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	expectOwner(repo)
	ids, keys := map[uuid.UUID]bool{}, map[string]bool{}
	repo.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, rec repository.APIKeyRecord) (repository.APIKeyRecord, error) {
			ids[rec.ID] = true
			return rec, nil
		},
	).Times(2)

	for range 2 {
		got, err := svc.Create(t.Context(), actor, identityID, APIKeyInput{Name: "ci"})
		require.NoError(t, err)
		keys[got.Key] = true
	}

	assert.Len(t, ids, 2)
	assert.Len(t, keys, 2)
}

// expectFieldChecks makes the decider allow creating an API key with the given fields, and deny the others.
// It returns the fields that the decider checked and the resource of each check, in order.
func expectFieldChecks(decider *authzmocks.MockDecider, allowed []string) (*[]string, *[]authz.Resource) {
	var checked []string
	var resources []authz.Resource
	for _, field := range []string{"name", "expiresAt"} {
		decider.EXPECT().Can(authz.NewAction("apikey:create:"+field, "apikey:create"), mock.Anything).
			RunAndReturn(func(_ authz.Action, resource authz.Resource) (bool, error) {
				checked = append(checked, field)
				resources = append(resources, resource)

				return slices.Contains(allowed, field), nil
			}).Maybe()
	}

	return &checked, &resources
}

// TestAPIKeyService_Create_ChecksFields checks that creating checks the fields that the input sets, on the
// API key to be created with its owner, and nothing else: leaving out the expiry needs no access to it.
func TestAPIKeyService_Create_ChecksFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		in          APIKeyInput
		allowed     []string
		wantChecked []string
		wantDenied  []string
	}{
		{name: "never expires", in: APIKeyInput{Name: "ci"}, allowed: []string{"name"}, wantChecked: []string{"name"}},
		{
			name:        "expires",
			in:          APIKeyInput{Name: "ci", ExpiresAt: &expiresAt},
			allowed:     []string{"name", "expiresAt"},
			wantChecked: []string{"name", "expiresAt"},
		},
		{
			name:        "expiry denied",
			in:          APIKeyInput{Name: "ci", ExpiresAt: &expiresAt},
			allowed:     []string{"name"},
			wantChecked: []string{"name", "expiresAt"},
			wantDenied:  []string{"expiresAt"},
		},
		{
			name:        "all denied",
			in:          APIKeyInput{Name: "ci", ExpiresAt: &expiresAt},
			wantChecked: []string{"name", "expiresAt"},
			wantDenied:  []string{"name", "expiresAt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo := newTestServiceWith(t, authorizer)
			expectOwnerReadable(repo, decider)
			checked, resources := expectFieldChecks(decider, tt.allowed)
			var inserted repository.APIKeyRecord
			var readResources []authz.Resource
			if tt.wantDenied == nil {
				decider.EXPECT().Can(mock.Anything, mock.Anything).RunAndReturn(
					func(_ authz.Action, resource authz.Resource) (bool, error) {
						readResources = append(readResources, resource)
						return true, nil
					},
				)
				repo.EXPECT().Insert(mock.Anything, servicetest.Capture(&inserted)).Return(stored, nil)
			}

			_, err := svc.Create(t.Context(), actor, identityID, tt.in)

			assert.Equal(t, tt.wantChecked, *checked)
			if tt.wantDenied == nil {
				require.NoError(t, err)
				for _, resource := range append(*resources, readResources...) {
					assert.Equal(t, authz.Resource{Type: "APIKey", ID: inserted.ID, Owner: owner}, resource,
						"the check must be on the API key to be created, with its owner")
				}
				return
			}
			// The mock fails the test if the repository is called.
			var denied *authz.FieldsDeniedError
			require.ErrorAs(t, err, &denied)
			assert.Equal(t, tt.wantDenied, denied.Fields)
		})
	}
}

// TestAPIKeyService_Create_ReadableFields checks that the new API key is returned with the fields that the
// actor may read, and with only its ID if it may not read the API key, but always with its key, which can't
// be retrieved later.
func TestAPIKeyService_Create_ReadableFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		canRead  bool
		readable []string
		want     APIKey
	}{
		{name: "all", canRead: true, readable: actions.ReadFields().Fields(), want: storedKey},
		{
			name: "some", canRead: true, readable: []string{"name", "expiresAt"},
			want: APIKey{ID: stored.ID, Name: authz.Visible("ci"), ExpiresAt: authz.Visible(&expiresAt)},
		},
		{name: "unreadable api key", canRead: false, want: APIKey{ID: stored.ID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo := newTestServiceWith(t, authorizer)
			expectOwnerReadable(repo, decider)
			// The service generates the ID, so the checks match any resource. That they're on the new API
			// key is checked by TestAPIKeyService_Create_ChecksFields.
			expectFieldChecks(decider, []string{"name"})
			decider.EXPECT().Can(actions.Read(), mock.Anything).Return(tt.canRead, nil).Once()
			if tt.canRead {
				for i, action := range actions.ReadFields().Actions() {
					decider.EXPECT().Can(action, mock.Anything).
						Return(slices.Contains(tt.readable, actions.ReadFields().Fields()[i]), nil).Once()
				}
			}
			repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(stored, nil)

			got, err := svc.Create(t.Context(), actor, identityID, APIKeyInput{Name: "ci"})
			require.NoError(t, err)

			assert.Equal(t, tt.want, got.APIKey)
			assert.Regexp(t, `^psk_[A-Z2-7]{52}$`, got.Key)
		})
	}
}

func TestAPIKeyService_Declaration(t *testing.T) {
	t.Parallel()

	declaration := Actions().Declaration()

	assert.Equal(t, "APIKey", declaration.Type)
	assert.True(t, declaration.Owned, "policies must be able to refer to the owner of an API key")
}

func TestAPIKeyService_Get(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	expectAllFieldsReadable(decider, storedResource)

	got, err := svc.Get(t.Context(), actor, identityID, stored.ID)
	require.NoError(t, err)

	assert.Equal(t, storedKey, got)
}

// TestAPIKeyService_Get_HiddenFields checks that only the fields that the actor may read are visible, and
// the ID always: a hidden expiry differs from one that's visible and nil.
func TestAPIKeyService_Get_HiddenFields(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), storedResource, "name", "keySuffix")

	got, err := svc.Get(t.Context(), actor, identityID, stored.ID)
	require.NoError(t, err)

	assert.Equal(t, APIKey{ID: stored.ID, Name: authz.Visible("ci"), KeySuffix: authz.Visible("WXYZ")}, got)
}

// TestAPIKeyService_Get_NeverExpires checks that a visible expiry of an API key that never expires is nil,
// unlike a hidden one.
func TestAPIKeyService_Get_NeverExpires(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectOwnerReadable(repo, decider)
	rec := stored
	rec.ExpiresAt = nil
	repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(rec, nil)
	decider.EXPECT().Can(actions.Read(), storedResource).Return(true, nil)
	servicetest.ExpectReadableFields(decider, actions.ReadFields(), storedResource, "expiresAt")

	got, err := svc.Get(t.Context(), actor, identityID, stored.ID)
	require.NoError(t, err)

	assert.Equal(t, APIKey{ID: stored.ID, ExpiresAt: authz.Visible[*time.Time](nil)}, got)
}

// TestAPIKeyService_List_Pages cuts the pages out of what the actor may read, with hidden API keys between
// them, so that only the last page can be short and HasMore counts readable API keys only.
func TestAPIKeyService_List_Pages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		offset, limit int
		wantIDs       []byte
		wantHasMore   bool
	}{
		{name: "first page", offset: 0, limit: 2, wantIDs: []byte{2, 4}, wantHasMore: true},
		{name: "middle page", offset: 1, limit: 2, wantIDs: []byte{4, 6}, wantHasMore: true},
		{name: "full last page, with hidden rows after it", offset: 2, limit: 2, wantIDs: []byte{6, 8}},
		{name: "short last page", offset: 3, limit: 2, wantIDs: []byte{8}},
		{name: "all", offset: 0, limit: 50, wantIDs: []byte{2, 4, 6, 8}},
		{name: "past the end", offset: 4, limit: 2, wantIDs: []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo := newTestServiceWith(t, authorizer)
			expectOwnerReadable(repo, decider)
			repo.EXPECT().ListAll(mock.Anything, identityID).Return(newRecords(1, 2, 3, 4, 5, 6, 7, 8, 9), nil)
			decider.EXPECT().Can(actions.Read(), mock.Anything).RunAndReturn(
				func(_ authz.Action, resource authz.Resource) (bool, error) {
					assert.Equal(t, "APIKey", resource.Type)
					assert.Equal(t, owner, resource.Owner)
					return resource.ID[15]%2 == 0, nil
				},
			).Times(9)
			fieldChecks := map[byte]int{}
			decider.EXPECT().Can(mock.MatchedBy(isReadField), mock.Anything).RunAndReturn(
				func(_ authz.Action, resource authz.Resource) (bool, error) {
					assert.Equal(t, owner, resource.Owner)
					fieldChecks[resource.ID[15]]++
					return true, nil
				},
			).Maybe()

			got, err := svc.List(t.Context(), actor, identityID, tt.offset, tt.limit)
			require.NoError(t, err)

			assert.Equal(t, tt.wantIDs, idsOf(got.Items))
			assert.Equal(t, tt.wantHasMore, got.HasMore)
			wantFieldChecks := map[byte]int{}
			for _, id := range tt.wantIDs {
				wantFieldChecks[id] = len(actions.ReadFields().Fields())
			}
			assert.Equal(t, wantFieldChecks, fieldChecks, "the fields of the rows on the page must be checked, and only those")
		})
	}
}

func TestAPIKeyService_List_TranslatesRecords(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	expectOwner(repo)
	repo.EXPECT().ListAll(mock.Anything, identityID).Return([]repository.APIKeyRecord{stored}, nil)

	got, err := svc.List(t.Context(), actor, identityID, 0, 10)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[APIKey]{Items: []APIKey{storedKey}}, got)
}

// TestAPIKeyService_List_Empty checks that an identity without API keys, or without any that the actor may
// read, has an empty list, which the identity's existence decides and not the API keys.
func TestAPIKeyService_List_Empty(t *testing.T) {
	t.Parallel()
	for name, recs := range map[string][]repository.APIKeyRecord{"none": nil, "none readable": newRecords(1, 2)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo := newTestServiceWith(t, authorizer)
			expectOwnerReadable(repo, decider)
			repo.EXPECT().ListAll(mock.Anything, identityID).Return(recs, nil)
			decider.EXPECT().Can(actions.Read(), mock.Anything).Return(false, nil).Maybe()

			got, err := svc.List(t.Context(), actor, identityID, 0, 5)
			require.NoError(t, err)

			assert.Empty(t, got.Items)
			assert.False(t, got.HasMore)
		})
	}
}

func TestAPIKeyService_List_ChecksContextBetweenRows(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	expectOwnerReadable(repo, decider)
	repo.EXPECT().ListAll(mock.Anything, identityID).Return(newRecords(1, 2, 3), nil)
	decider.EXPECT().Can(actions.Read(), mock.Anything).RunAndReturn(
		func(authz.Action, authz.Resource) (bool, error) {
			cancel()
			return true, nil
		},
	).Once()

	_, err := svc.List(ctx, actor, identityID, 0, 5)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestAPIKeyService_List_Errors(t *testing.T) {
	t.Parallel()
	for name, repoErr := range map[string]error{
		"list fails": errDB,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			expectOwner(repo)
			repo.EXPECT().ListAll(mock.Anything, identityID).Return(nil, repoErr)

			_, err := svc.List(t.Context(), actor, identityID, 0, 50)

			servicetest.AssertTranslated(t, err, repoErr, repoErr)
			assert.NotErrorIs(t, err, ErrIdentityNotFound)
		})
	}
}

func TestAPIKeyService_Delete(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(actions.Delete(), storedResource).Return(true, nil)
	repo.EXPECT().Delete(mock.Anything, identityID, stored.ID).Return(nil)

	assert.NoError(t, svc.Delete(t.Context(), actor, identityID, stored.ID))
}

// calls returns a call of each authorized method of the service on stored or its identity.
func calls(t *testing.T) map[string]func(*apiKeyService) error {
	t.Helper()

	return map[string]func(*apiKeyService) error{
		"create": func(svc *apiKeyService) error {
			_, err := svc.Create(t.Context(), actor, identityID, APIKeyInput{Name: "ci"})
			return err
		},
		"get": func(svc *apiKeyService) error {
			_, err := svc.Get(t.Context(), actor, identityID, stored.ID)
			return err
		},
		"list": func(svc *apiKeyService) error {
			_, err := svc.List(t.Context(), actor, identityID, 0, 5)
			return err
		},
		"delete": func(svc *apiKeyService) error {
			return svc.Delete(t.Context(), actor, identityID, stored.ID)
		},
	}
}

// TestAPIKeyService_IdentityHidden checks that every method reports an identity that the actor may not read
// like a missing one, before anything else is checked or loaded: anything else would reveal that the
// identity exists.
func TestAPIKeyService_IdentityHidden(t *testing.T) {
	t.Parallel()
	for name, call := range calls(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mocks fail the test on any other decision or repository call.
			svc, repo := newTestServiceWith(t, authorizer)
			expectOwner(repo)
			decider.EXPECT().Can(readIdentity, identityResource).Return(false, nil)

			err := call(svc)

			require.ErrorIs(t, err, ErrIdentityNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
	}
}

// TestAPIKeyService_IdentityNotFound checks that every method reports a missing identity in the same way as
// a hidden one, without deciding anything.
func TestAPIKeyService_IdentityNotFound(t *testing.T) {
	t.Parallel()
	for name, call := range calls(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, _ := newDeciding(t)
			// The mocks fail the test on any decision or other repository call.
			svc, repo := newTestServiceWith(t, authorizer)
			repoErr := fmt.Errorf("list api key identity groups: %w", repository.ErrIdentityNotFound)
			repo.EXPECT().IdentityGroups(mock.Anything, identityID).Return(nil, repoErr)

			err := call(svc)

			servicetest.AssertTranslated(t, err, ErrIdentityNotFound, repository.ErrIdentityNotFound)
		})
	}
}

// TestAPIKeyService_KeyHidden checks that an API key that the actor may not read is reported like a missing
// one, before its action is checked.
func TestAPIKeyService_KeyHidden(t *testing.T) {
	t.Parallel()
	all := calls(t)
	for _, name := range []string{"get", "delete"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mocks fail the test on any other decision or on a write.
			svc, repo := newTestServiceWith(t, authorizer)
			expectOwnerReadable(repo, decider)
			repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(stored, nil)
			decider.EXPECT().Can(actions.Read(), storedResource).Return(false, nil)

			err := all[name](svc)

			require.ErrorIs(t, err, ErrAPIKeyNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
	}
}

// TestAPIKeyService_Delete_Denied checks that deleting a readable API key that the actor may not delete is
// denied, without anything being written.
func TestAPIKeyService_Delete_Denied(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mock fails the test on a write.
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(actions.Delete(), storedResource).Return(false, nil)

	err := svc.Delete(t.Context(), actor, identityID, stored.ID)

	require.ErrorIs(t, err, authz.ErrDenied)
	assert.NotErrorIs(t, err, ErrAPIKeyNotFound)
}

func TestAPIKeyService_TranslatesRepositoryErrors(t *testing.T) {
	t.Parallel()
	type call func(svc *apiKeyService, repo *mocks.MockAPIKeyRepository, repoErr error) error

	create := func(svc *apiKeyService, repo *mocks.MockAPIKeyRepository, repoErr error) error {
		repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(repository.APIKeyRecord{}, repoErr)
		_, err := svc.Create(t.Context(), actor, identityID, APIKeyInput{Name: "ci"})
		return err
	}
	get := func(svc *apiKeyService, repo *mocks.MockAPIKeyRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(repository.APIKeyRecord{}, repoErr)
		_, err := svc.Get(t.Context(), actor, identityID, stored.ID)
		return err
	}
	deleteMissing := func(svc *apiKeyService, repo *mocks.MockAPIKeyRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(repository.APIKeyRecord{}, repoErr)
		return svc.Delete(t.Context(), actor, identityID, stored.ID)
	}
	// Deleted by another request between reading and deleting it.
	deleteGone := func(svc *apiKeyService, repo *mocks.MockAPIKeyRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(stored, nil)
		repo.EXPECT().Delete(mock.Anything, identityID, stored.ID).Return(repoErr)
		return svc.Delete(t.Context(), actor, identityID, stored.ID)
	}

	tests := []struct {
		name    string
		call    call
		repoErr error
		wantErr error
	}{
		// The identity was deleted between reading it and inserting the API key.
		{"create identity not found", create, repository.ErrIdentityNotFound, ErrIdentityNotFound},
		{"create unexpected", create, errDB, errDB},
		{"get not found", get, repository.ErrNotFound, ErrAPIKeyNotFound},
		{"get unexpected", get, errDB, errDB},
		{"delete not found", deleteMissing, repository.ErrNotFound, ErrAPIKeyNotFound},
		{"delete gone", deleteGone, repository.ErrNotFound, ErrAPIKeyNotFound},
		{"delete unexpected", deleteGone, errDB, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			expectOwner(repo)

			err := tt.call(svc, repo, fmt.Errorf("query: %w", tt.repoErr))

			servicetest.AssertTranslated(t, err, tt.wantErr, tt.repoErr)
		})
	}
}

func TestAPIKeyService_IdentityLookupFails(t *testing.T) {
	t.Parallel()
	for name, call := range calls(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			repo.EXPECT().IdentityGroups(mock.Anything, identityID).Return(nil, errDB)

			err := call(svc)

			require.ErrorIs(t, err, errDB)
			assert.NotErrorIs(t, err, ErrIdentityNotFound)
		})
	}
}

// TestAPIKeyService_AuthorizationFails checks that failing to decide is an error and never an answer: not
// access, and not a missing identity or API key, which would hide a broken policy from the logs.
func TestAPIKeyService_AuthorizationFails(t *testing.T) {
	t.Parallel()
	errDecision := errors.New("policy failed")
	assertFailed := func(t *testing.T, err error) {
		t.Helper()
		require.ErrorIs(t, err, errDecision)
		require.NotErrorIs(t, err, ErrIdentityNotFound)
		require.NotErrorIs(t, err, ErrAPIKeyNotFound)
		assert.NotErrorIs(t, err, authz.ErrDenied)
	}

	for name, call := range calls(t) {
		t.Run(name+" no decider", func(t *testing.T) {
			t.Parallel()
			authorizer := authzmocks.NewMockAuthorizer(t)
			authorizer.EXPECT().For(mock.Anything, actor).Return(nil, errDecision)
			// The mock fails the test if the repository is called.
			svc, _ := newTestServiceWith(t, authorizer)

			assertFailed(t, call(svc))
		})
		t.Run(name+" no identity decision", func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mock fails the test on any other repository call.
			svc, repo := newTestServiceWith(t, authorizer)
			expectOwner(repo)
			decider.EXPECT().Can(readIdentity, identityResource).Return(false, errDecision)

			assertFailed(t, call(svc))
		})
		t.Run(name+" no decision on the api key", func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mock fails the test if the repository writes anything.
			svc, repo := newTestServiceWith(t, authorizer)
			expectOwnerReadable(repo, decider)
			repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(stored, nil).Maybe()
			repo.EXPECT().ListAll(mock.Anything, identityID).Return([]repository.APIKeyRecord{stored}, nil).Maybe()
			decider.EXPECT().Can(mock.Anything, mock.Anything).Return(false, errDecision)

			assertFailed(t, call(svc))
		})
	}

	// The field read decisions, after every other decision allowed the request. They come before any write,
	// so that a failed one can't fail a request whose write succeeded.
	all := calls(t)
	for _, name := range []string{"create", "get", "list"} {
		t.Run(name+" no field decision", func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mock fails the test if the repository writes anything.
			svc, repo := newTestServiceWith(t, authorizer)
			expectOwner(repo)
			repo.EXPECT().Get(mock.Anything, identityID, stored.ID).Return(stored, nil).Maybe()
			repo.EXPECT().ListAll(mock.Anything, identityID).Return([]repository.APIKeyRecord{stored}, nil).Maybe()
			decider.EXPECT().Can(mock.MatchedBy(isReadField), mock.Anything).Return(false, errDecision)
			decider.EXPECT().Can(mock.Anything, mock.Anything).Return(true, nil).Maybe()

			assertFailed(t, all[name](svc))
		})
	}

	t.Run("delete no action decision", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		// The mock fails the test on a write.
		svc, repo := newTestServiceWith(t, authorizer)
		expectReadable(repo, decider)
		decider.EXPECT().Can(actions.Delete(), storedResource).Return(false, errDecision)

		assertFailed(t, svc.Delete(t.Context(), actor, identityID, stored.ID))
	})
}

func TestAPIKeyService_Hash(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService(t)

	// Computed with openssl. A changed algorithm invalidates every stored key, so it must fail here.
	assert.Equal(t,
		"5436c1cb28ef90614916fb6e6002b683bd716970e1cf0cfe7190b98213db5756",
		hex.EncodeToString(svc.hash("psk_TESTKEY")),
	)

	other := newAPIKeyService(mocks.NewMockAPIKeyRepository(t), authz.AllowAll(),
		[]byte("another-pepper-of-at-least-32-chars"), time.Now)
	assert.NotEqual(t, svc.hash("psk_TESTKEY"), other.hash("psk_TESTKEY"), "the hash must depend on the pepper")
}

const presentedKey = "psk_PRESENTEDKEY"

func TestAPIKeyService_Authenticate(t *testing.T) {
	t.Parallel()
	recent := now.Add(-lastUsedInterval + time.Second)
	stale := now.Add(-lastUsedInterval)
	tests := []struct {
		name       string
		expiresAt  *time.Time
		lastUsedAt *time.Time
		wantUpdate bool
	}{
		{name: "first use", wantUpdate: true},
		{name: "used long ago", lastUsedAt: &stale, wantUpdate: true},
		{name: "used recently", lastUsedAt: &recent},
		{name: "not yet expired", expiresAt: new(now.Add(time.Second)), lastUsedAt: &recent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			rec := stored
			rec.ExpiresAt, rec.LastUsedAt = tt.expiresAt, tt.lastUsedAt
			repo.EXPECT().GetByHash(mock.Anything, svc.hash(presentedKey)).Return(rec, nil)
			if tt.wantUpdate {
				// The mock fails the test on an update that isn't expected.
				repo.EXPECT().UpdateLastUsed(mock.Anything, stored.ID, now).Return(nil)
			}

			got, err := svc.Authenticate(t.Context(), presentedKey)
			require.NoError(t, err)

			assert.Equal(t, AuthenticatedKey{ID: stored.ID, IdentityID: identityID}, got)
		})
	}
}

func TestAPIKeyService_Authenticate_MalformedKeyIsNotLookedUp(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"", "psk", "PSK_PRESENTEDKEY", "other_PRESENTEDKEY"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on any call.
			svc, _ := newTestService(t)

			_, err := svc.Authenticate(t.Context(), key)

			assert.ErrorIs(t, err, ErrMalformedKey)
		})
	}
}

func TestAPIKeyService_Authenticate_UnknownKey(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().GetByHash(mock.Anything, svc.hash(presentedKey)).
		Return(repository.APIKeyRecord{}, fmt.Errorf("get api key by hash: %w", repository.ErrNotFound))

	_, err := svc.Authenticate(t.Context(), presentedKey)

	servicetest.AssertTranslated(t, err, ErrUnknownKey, repository.ErrNotFound)
	assert.NotContains(t, err.Error(), presentedKey)
}

func TestAPIKeyService_Authenticate_ExpiredKey(t *testing.T) {
	t.Parallel()
	for name, expiry := range map[string]time.Time{"at the expiry": now, "after the expiry": now.Add(-time.Second)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			rec := stored
			rec.ExpiresAt = &expiry
			// An expired key's use isn't recorded: the mock fails the test on UpdateLastUsed.
			repo.EXPECT().GetByHash(mock.Anything, svc.hash(presentedKey)).Return(rec, nil)

			_, err := svc.Authenticate(t.Context(), presentedKey)

			require.ErrorIs(t, err, ErrExpiredKey)
			assert.Contains(t, err.Error(), stored.ID.String(), "the logs must tell which API key expired")
			assert.NotContains(t, err.Error(), presentedKey)
		})
	}
}

func TestAPIKeyService_Authenticate_Errors(t *testing.T) {
	t.Parallel()

	t.Run("lookup fails", func(t *testing.T) {
		t.Parallel()
		svc, repo := newTestService(t)
		repo.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(repository.APIKeyRecord{}, errDB)

		_, err := svc.Authenticate(t.Context(), presentedKey)

		assert.ErrorIs(t, err, errDB)
	})
	t.Run("recording the use fails", func(t *testing.T) {
		t.Parallel()
		svc, repo := newTestService(t)
		rec := stored
		rec.LastUsedAt = nil
		repo.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(rec, nil)
		repo.EXPECT().UpdateLastUsed(mock.Anything, stored.ID, now).Return(errDB)

		_, err := svc.Authenticate(t.Context(), presentedKey)

		assert.ErrorIs(t, err, errDB)
	})
}

// TestAPIKeyService_Authenticate_NotAuthorized checks that authenticating asks no authorizer, since a request
// has no actor before it's authenticated.
func TestAPIKeyService_Authenticate_NotAuthorized(t *testing.T) {
	t.Parallel()
	// The mock fails the test on any call.
	svc, repo := newTestServiceWith(t, authzmocks.NewMockAuthorizer(t))
	rec := stored
	rec.ExpiresAt, rec.LastUsedAt = nil, &now
	repo.EXPECT().GetByHash(mock.Anything, svc.hash(presentedKey)).Return(rec, nil)

	_, err := svc.Authenticate(t.Context(), presentedKey)

	assert.NoError(t, err)
}
