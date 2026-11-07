package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
	"unicode"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/authz"
	authzmocks "pkgsilo/core/authz/mocks"
	"pkgsilo/core/pagination"
	"pkgsilo/feature/identity/repository"
	"pkgsilo/feature/identity/repository/mocks"
	"pkgsilo/test/servicetest"
)

var (
	now   = time.Date(2026, time.October, 24, 12, 0, 0, 0, time.UTC)
	actor = uuid.MustParse("6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b")
	input = IdentityInput{Username: "ada", DisplayName: "Ada Lovelace"}

	// stored is a record as the repository returns it, with a distinct value in every field.
	stored = repository.IdentityRecord{
		ID:          uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		Username:    "grace",
		DisplayName: "Grace Hopper",
		CreatedBy:   uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"),
		UpdatedBy:   uuid.MustParse("2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e"),
		CreatedAt:   now.Add(-time.Hour),
		UpdatedAt:   now.Add(-time.Minute),
	}
	// storedIdentity is stored, translated to the domain model.
	storedIdentity = Identity{
		ID:          stored.ID,
		Username:    "grace",
		DisplayName: "Grace Hopper",
		CreatedBy:   stored.CreatedBy,
		UpdatedBy:   stored.UpdatedBy,
		CreatedAt:   stored.CreatedAt,
		UpdatedAt:   stored.UpdatedAt,
	}

	// storedResource is stored as a resource of decisions, in no group.
	storedResource = authz.Resource{Type: "Identity", ID: stored.ID}

	maintainers = uuid.MustParse("3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f")
	reviewers   = uuid.MustParse("4d5e6f7a-8b9c-4d0e-9f1a-2b3c4d5e6f7a")

	errDB = errors.New("connection refused")
)

// newTestService returns a service that may do everything, on a mock repository.
func newTestService(t *testing.T) (*identityService, *mocks.MockIdentityRepository) {
	t.Helper()

	return newTestServiceWith(t, authz.AllowAll())
}

// newTestServiceWith returns a service that asks authorizer, on a mock repository.
func newTestServiceWith(
	t *testing.T, authorizer authz.Authorizer,
) (*identityService, *mocks.MockIdentityRepository) {
	t.Helper()

	repo := mocks.NewMockIdentityRepository(t)

	return newIdentityService(repo, authorizer, func() time.Time { return now }), repo
}

// newDeciding returns an authorizer that answers actor's requests with a mock decider.
func newDeciding(t *testing.T) (*authzmocks.MockAuthorizer, *authzmocks.MockDecider) {
	t.Helper()

	authorizer := authzmocks.NewMockAuthorizer(t)
	decider := authzmocks.NewMockDecider(t)
	authorizer.EXPECT().For(mock.Anything, actor).Return(decider, nil)

	return authorizer, decider
}

// expectReadable makes the repository return stored, in no group, and the decider allow reading it.
func expectReadable(repo *mocks.MockIdentityRepository, decider *authzmocks.MockDecider) {
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
	repo.EXPECT().ListMemberships(mock.Anything, []uuid.UUID{stored.ID}).Return(nil, nil)
	decider.EXPECT().Can(actionRead, storedResource).Return(true, nil)
}

// newRecords returns stored records with the given IDs' last bytes, in order.
func newRecords(lastBytes ...byte) []repository.IdentityRecord {
	recs := make([]repository.IdentityRecord, len(lastBytes))
	for i, lastByte := range lastBytes {
		recs[i] = stored
		recs[i].ID = uuid.UUID{15: lastByte}
	}

	return recs
}

// idsOf returns the IDs' last bytes of the identities, in order.
func idsOf(identities []Identity) []byte {
	ids := make([]byte, len(identities))
	for i, identity := range identities {
		ids[i] = identity.ID[15]
	}

	return ids
}

func TestIdentityService_Create(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	var inserted repository.IdentityRecord
	repo.EXPECT().Insert(mock.Anything, servicetest.Capture(&inserted)).Return(stored, nil)

	got, err := svc.Create(t.Context(), actor, input)
	require.NoError(t, err)

	assert.Equal(t, byte(4), inserted.ID[6]>>4, "ID must be a version 4 UUID")
	assert.Equal(t, repository.IdentityRecord{
		ID:          inserted.ID,
		Username:    "ada",
		DisplayName: "Ada Lovelace",
		CreatedBy:   actor,
		UpdatedBy:   actor,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, inserted)
	// The identity as stored is returned, not the one sent to the repository.
	assert.Equal(t, storedIdentity, got)
}

func TestIdentityService_Create_GeneratesUniqueIDs(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	ids := map[uuid.UUID]bool{}
	repo.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, rec repository.IdentityRecord) (repository.IdentityRecord, error) {
			ids[rec.ID] = true
			return rec, nil
		},
	).Times(2)

	for range 2 {
		_, err := svc.Create(t.Context(), actor, input)
		require.NoError(t, err)
	}

	assert.Len(t, ids, 2)
}

// TestIdentityService_Create_ChecksEveryField checks that creating needs the action of each field, on the
// identity to be created, and that a denial lists every denied field and creates nothing.
func TestIdentityService_Create_ChecksEveryField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		allowed    []string
		wantDenied []string
	}{
		{name: "all allowed", allowed: []string{"username", "displayName"}},
		{name: "username denied", allowed: []string{"displayName"}, wantDenied: []string{"username"}},
		{name: "all denied", wantDenied: []string{"username", "displayName"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo := newTestServiceWith(t, authorizer)
			var checked []authz.Resource
			decider.EXPECT().Can(mock.Anything, mock.Anything).RunAndReturn(
				func(action authz.Action, resource authz.Resource) (bool, error) {
					checked = append(checked, resource)
					for _, field := range tt.allowed {
						if assert.ObjectsAreEqual(authz.NewAction("identity:create:"+field, "identity:create"), action) {
							return true, nil
						}
					}

					return false, nil
				},
			).Times(2)
			var inserted repository.IdentityRecord
			if tt.wantDenied == nil {
				repo.EXPECT().Insert(mock.Anything, servicetest.Capture(&inserted)).Return(stored, nil)
			}

			_, err := svc.Create(t.Context(), actor, input)

			if tt.wantDenied == nil {
				require.NoError(t, err)
				want := authz.Resource{Type: "Identity", ID: inserted.ID}
				assert.Equal(t, []authz.Resource{want, want}, checked, "the checks must be on the identity to be created")
				return
			}
			// The mock fails the test if the repository is called.
			var denied *authz.FieldsDeniedError
			require.ErrorAs(t, err, &denied)
			assert.Equal(t, tt.wantDenied, denied.Fields)
		})
	}
}

// TestIdentityService_FieldActions checks that every field of IdentityInput, which creating and replacing
// set, has an action of its own for each, so that no field is written unchecked.
func TestIdentityService_FieldActions(t *testing.T) {
	t.Parallel()
	var fields []string
	for field := range reflect.TypeFor[IdentityInput]().Fields() {
		name := []rune(field.Name)
		name[0] = unicode.ToLower(name[0])
		fields = append(fields, string(name))
	}

	assert.ElementsMatch(t, fields, CreatableFields())
	assert.ElementsMatch(t, fields, UpdatableFields())
}

func TestIdentityService_Get(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
	repo.EXPECT().ListMemberships(mock.Anything, []uuid.UUID{stored.ID}).Return([]repository.MembershipRecord{
		{IdentityID: stored.ID, GroupID: maintainers},
		{IdentityID: stored.ID, GroupID: reviewers},
	}, nil)
	decider.EXPECT().Can(actionRead, authz.Resource{
		Type: "Identity", ID: stored.ID, Groups: []uuid.UUID{maintainers, reviewers},
	}).Return(true, nil)

	got, err := svc.Get(t.Context(), actor, stored.ID)
	require.NoError(t, err)

	assert.Equal(t, storedIdentity, got)
}

func TestIdentityService_Get_Hidden(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
	repo.EXPECT().ListMemberships(mock.Anything, []uuid.UUID{stored.ID}).Return(nil, nil)
	decider.EXPECT().Can(actionRead, authz.Resource{Type: "Identity", ID: stored.ID}).Return(false, nil)

	got, err := svc.Get(t.Context(), actor, stored.ID)

	require.ErrorIs(t, err, ErrIdentityNotFound, "a hidden identity must look like a missing one")
	assert.Equal(t, Identity{}, got)
}

// TestIdentityService_List_Pages cuts the pages out of what the actor may read, with hidden rows between
// them, so that only the last page can be short and HasMore counts readable rows only.
func TestIdentityService_List_Pages(t *testing.T) {
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
			repo.EXPECT().ListAll(mock.Anything).Return(newRecords(1, 2, 3, 4, 5, 6, 7, 8, 9), nil)
			repo.EXPECT().ListMemberships(mock.Anything, mock.Anything).Return(nil, nil)
			decider.EXPECT().Can(actionRead, mock.Anything).RunAndReturn(
				func(_ authz.Action, resource authz.Resource) (bool, error) {
					return resource.ID[15]%2 == 0, nil
				},
			)

			got, err := svc.List(t.Context(), actor, tt.offset, tt.limit)
			require.NoError(t, err)

			assert.Equal(t, tt.wantIDs, idsOf(got.Items))
			assert.Equal(t, tt.wantHasMore, got.HasMore)
			decider.AssertNumberOfCalls(t, "Can", 9)
		})
	}
}

func TestIdentityService_List_TranslatesRecords(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().ListAll(mock.Anything).Return([]repository.IdentityRecord{stored}, nil)
	repo.EXPECT().ListMemberships(mock.Anything, []uuid.UUID{stored.ID}).Return(nil, nil)

	got, err := svc.List(t.Context(), actor, 0, 10)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Identity]{Items: []Identity{storedIdentity}}, got)
}

func TestIdentityService_List_NothingReadable(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	repo.EXPECT().ListAll(mock.Anything).Return(newRecords(1, 2), nil)
	repo.EXPECT().ListMemberships(mock.Anything, mock.Anything).Return(nil, nil)
	decider.EXPECT().Can(actionRead, mock.Anything).Return(false, nil)

	got, err := svc.List(t.Context(), actor, 0, 5)
	require.NoError(t, err)

	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
}

func TestIdentityService_List_Empty(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().ListAll(mock.Anything).Return(nil, nil)
	repo.EXPECT().ListMemberships(mock.Anything, mock.Anything).Return(nil, nil)

	got, err := svc.List(t.Context(), actor, 0, 5)
	require.NoError(t, err)

	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
}

// TestIdentityService_List_LoadsGroupsInBulk checks that one query serves all rows, each of which gets
// its own groups.
func TestIdentityService_List_LoadsGroupsInBulk(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	recs := newRecords(1, 2)
	repo.EXPECT().ListAll(mock.Anything).Return(recs, nil)
	repo.EXPECT().ListMemberships(mock.Anything, []uuid.UUID{recs[0].ID, recs[1].ID}).
		Return([]repository.MembershipRecord{
			{IdentityID: recs[0].ID, GroupID: maintainers},
			{IdentityID: recs[0].ID, GroupID: reviewers},
		}, nil).
		Once()
	decider.EXPECT().Can(actionRead, authz.Resource{
		Type: "Identity", ID: recs[0].ID, Groups: []uuid.UUID{maintainers, reviewers},
	}).Return(true, nil)
	decider.EXPECT().Can(actionRead, authz.Resource{Type: "Identity", ID: recs[1].ID}).Return(true, nil)

	got, err := svc.List(t.Context(), actor, 0, 5)
	require.NoError(t, err)

	assert.Len(t, got.Items, 2)
}

func TestIdentityService_List_ChecksContextBetweenRows(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	repo.EXPECT().ListAll(mock.Anything).Return(newRecords(1, 2, 3), nil)
	repo.EXPECT().ListMemberships(mock.Anything, mock.Anything).Return(nil, nil)
	decider.EXPECT().Can(actionRead, mock.Anything).RunAndReturn(
		func(authz.Action, authz.Resource) (bool, error) {
			cancel()
			return true, nil
		},
	).Once()

	_, err := svc.List(ctx, actor, 0, 5)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestIdentityService_Replace(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	for _, field := range []string{"username", "displayName"} {
		decider.EXPECT().Can(authz.NewAction("identity:update:"+field, "identity:update"), storedResource).
			Return(true, nil).Once()
	}
	repo.EXPECT().Update(mock.Anything, repository.IdentityRecord{
		ID:          stored.ID,
		Username:    "ada",
		DisplayName: "Ada Lovelace",
		UpdatedBy:   actor,
		UpdatedAt:   now,
	}).Return(stored, nil)

	got, err := svc.Replace(t.Context(), actor, stored.ID, input)
	require.NoError(t, err)

	assert.Equal(t, storedIdentity, got)
}

// TestIdentityService_Replace_FieldsDenied checks that replacing checks every field, even one whose value
// doesn't change, and that a denial lists every denied field and writes nothing.
func TestIdentityService_Replace_FieldsDenied(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mock fails the test if the repository updates anything.
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(authz.NewAction("identity:update:username", "identity:update"), storedResource).
		Return(false, nil)
	decider.EXPECT().Can(authz.NewAction("identity:update:displayName", "identity:update"), storedResource).
		Return(true, nil)
	unchanged := IdentityInput{Username: stored.Username, DisplayName: "Rear Admiral Hopper"}

	_, err := svc.Replace(t.Context(), actor, stored.ID, unchanged)

	var denied *authz.FieldsDeniedError
	require.ErrorAs(t, err, &denied)
	assert.Equal(t, []string{"username"}, denied.Fields)
}

func TestIdentityService_Delete(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(actionDelete, storedResource).Return(true, nil)
	repo.EXPECT().Delete(mock.Anything, stored.ID).Return(nil)

	assert.NoError(t, svc.Delete(t.Context(), actor, stored.ID))
}

func TestIdentityService_Delete_Denied(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mock fails the test if the repository deletes anything.
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(actionDelete, storedResource).Return(false, nil)

	err := svc.Delete(t.Context(), actor, stored.ID)

	require.ErrorIs(t, err, authz.ErrDenied)
	assert.NotErrorIs(t, err, ErrIdentityNotFound)
}

// TestIdentityService_Writes_Hidden checks that a write to an identity that the actor may not read is
// reported like a missing identity, before any write is checked: whether the actor may write it would
// otherwise reveal that it exists.
func TestIdentityService_Writes_Hidden(t *testing.T) {
	t.Parallel()
	calls := map[string]func(*identityService) error{
		"replace": func(svc *identityService) error {
			_, err := svc.Replace(t.Context(), actor, stored.ID, input)
			return err
		},
		"delete": func(svc *identityService) error {
			return svc.Delete(t.Context(), actor, stored.ID)
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mocks fail the test on any other decision or on a write.
			svc, repo := newTestServiceWith(t, authorizer)
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
			repo.EXPECT().ListMemberships(mock.Anything, []uuid.UUID{stored.ID}).Return(nil, nil)
			decider.EXPECT().Can(actionRead, storedResource).Return(false, nil)

			err := call(svc)

			require.ErrorIs(t, err, ErrIdentityNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
	}
}

func TestIdentityService_TranslatesRepositoryErrors(t *testing.T) {
	t.Parallel()
	type call func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error

	create := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(repository.IdentityRecord{}, repoErr)
		_, err := svc.Create(ctx, actor, input)
		return err
	}
	get := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.IdentityRecord{}, repoErr)
		_, err := svc.Get(ctx, actor, stored.ID)
		return err
	}
	getGroups := func(
		ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error,
	) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
		repo.EXPECT().ListMemberships(mock.Anything, mock.Anything).Return(nil, repoErr)
		_, err := svc.Get(ctx, actor, stored.ID)
		return err
	}
	list := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().ListAll(mock.Anything).Return(nil, repoErr)
		_, err := svc.List(ctx, actor, 0, 5)
		return err
	}
	listGroups := func(
		ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error,
	) error {
		repo.EXPECT().ListAll(mock.Anything).Return([]repository.IdentityRecord{stored}, nil)
		repo.EXPECT().ListMemberships(mock.Anything, mock.Anything).Return(nil, repoErr)
		_, err := svc.List(ctx, actor, 0, 5)
		return err
	}
	replaceGet := func(
		ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error,
	) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.IdentityRecord{}, repoErr)
		_, err := svc.Replace(ctx, actor, stored.ID, input)
		return err
	}
	replace := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
		repo.EXPECT().ListMemberships(mock.Anything, mock.Anything).Return(nil, nil)
		repo.EXPECT().Update(mock.Anything, mock.Anything).Return(repository.IdentityRecord{}, repoErr)
		_, err := svc.Replace(ctx, actor, stored.ID, input)
		return err
	}
	delGet := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.IdentityRecord{}, repoErr)
		return svc.Delete(ctx, actor, stored.ID)
	}
	del := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
		repo.EXPECT().ListMemberships(mock.Anything, mock.Anything).Return(nil, nil)
		repo.EXPECT().Delete(mock.Anything, stored.ID).Return(repoErr)
		return svc.Delete(ctx, actor, stored.ID)
	}

	tests := []struct {
		name    string
		call    call
		repoErr error
		wantErr error
	}{
		{name: "create username taken", call: create, repoErr: repository.ErrDuplicateUsername, wantErr: ErrUsernameTaken},
		{name: "create unexpected", call: create, repoErr: errDB, wantErr: errDB},
		{name: "get not found", call: get, repoErr: repository.ErrNotFound, wantErr: ErrIdentityNotFound},
		{name: "get unexpected", call: get, repoErr: errDB, wantErr: errDB},
		{name: "get groups unexpected", call: getGroups, repoErr: errDB, wantErr: errDB},
		{name: "list unexpected", call: list, repoErr: errDB, wantErr: errDB},
		// Answered with a logged 500 like any other unexpected error.
		{name: "list over the ceiling", call: list, repoErr: repository.ErrTooManyRows, wantErr: repository.ErrTooManyRows},
		{name: "list groups unexpected", call: listGroups, repoErr: errDB, wantErr: errDB},
		{name: "replace get not found", call: replaceGet, repoErr: repository.ErrNotFound, wantErr: ErrIdentityNotFound},
		{name: "replace get unexpected", call: replaceGet, repoErr: errDB, wantErr: errDB},
		{name: "replace not found", call: replace, repoErr: repository.ErrNotFound, wantErr: ErrIdentityNotFound},
		{name: "replace username taken", call: replace, repoErr: repository.ErrDuplicateUsername, wantErr: ErrUsernameTaken},
		{name: "replace unexpected", call: replace, repoErr: errDB, wantErr: errDB},
		{name: "delete get not found", call: delGet, repoErr: repository.ErrNotFound, wantErr: ErrIdentityNotFound},
		{name: "delete get unexpected", call: delGet, repoErr: errDB, wantErr: errDB},
		{name: "delete not found", call: del, repoErr: repository.ErrNotFound, wantErr: ErrIdentityNotFound},
		{name: "delete unexpected", call: del, repoErr: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, repo := newTestService(t)
			// The repository wraps its errors with context.
			err := tt.call(t.Context(), svc, repo, fmt.Errorf("query: %w", tt.repoErr))

			servicetest.AssertTranslated(t, err, tt.wantErr, tt.repoErr)
		})
	}
}

// TestIdentityService_AuthorizationFails checks that failing to decide is an error and never an answer:
// not access, and not a missing identity, which would hide a broken policy from the logs.
func TestIdentityService_AuthorizationFails(t *testing.T) {
	t.Parallel()
	errDecision := errors.New("policy failed")

	calls := map[string]func(*identityService) error{
		"create": func(svc *identityService) error {
			_, err := svc.Create(t.Context(), actor, input)
			return err
		},
		"get": func(svc *identityService) error {
			_, err := svc.Get(t.Context(), actor, stored.ID)
			return err
		},
		"list": func(svc *identityService) error {
			_, err := svc.List(t.Context(), actor, 0, 5)
			return err
		},
		"replace": func(svc *identityService) error {
			_, err := svc.Replace(t.Context(), actor, stored.ID, input)
			return err
		},
		"delete": func(svc *identityService) error {
			return svc.Delete(t.Context(), actor, stored.ID)
		},
	}
	for name, call := range calls {
		t.Run(name+" no decider", func(t *testing.T) {
			t.Parallel()
			authorizer := authzmocks.NewMockAuthorizer(t)
			authorizer.EXPECT().For(mock.Anything, actor).Return(nil, errDecision)
			// The mock fails the test if the repository is called.
			svc, _ := newTestServiceWith(t, authorizer)

			err := call(svc)

			require.ErrorIs(t, err, errDecision)
			require.NotErrorIs(t, err, ErrIdentityNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
		t.Run(name+" no decision", func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			decider.EXPECT().Can(mock.Anything, mock.Anything).Return(false, errDecision)
			// The mock fails the test if the repository writes anything.
			svc, repo := newTestServiceWith(t, authorizer)
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil).Maybe()
			repo.EXPECT().ListAll(mock.Anything).Return([]repository.IdentityRecord{stored}, nil).Maybe()
			repo.EXPECT().ListMemberships(mock.Anything, mock.Anything).Return(nil, nil).Maybe()

			err := call(svc)

			require.ErrorIs(t, err, errDecision)
			require.NotErrorIs(t, err, ErrIdentityNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
		// The write decisions, after the actor was allowed to read the identity.
		if name == "replace" || name == "delete" {
			t.Run(name+" no write decision", func(t *testing.T) {
				t.Parallel()
				authorizer, decider := newDeciding(t)
				svc, repo := newTestServiceWith(t, authorizer)
				expectReadable(repo, decider)
				decider.EXPECT().Can(mock.Anything, storedResource).Return(false, errDecision)

				err := call(svc)

				require.ErrorIs(t, err, errDecision)
				assert.NotErrorIs(t, err, authz.ErrDenied)
			})
		}
	}
}
