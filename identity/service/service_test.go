package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/pagination"
	"pkgsilo/identity/repository"
	"pkgsilo/identity/repository/mocks"
)

var (
	now   = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
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

	errDB = errors.New("connection refused")
)

func newTestService(t *testing.T) (*identityService, *mocks.MockIdentityRepository) {
	repo := mocks.NewMockIdentityRepository(t)

	return newIdentityService(repo, func() time.Time { return now }), repo
}

func TestIdentityService_Create(t *testing.T) {
	svc, repo := newTestService(t)
	var inserted repository.IdentityRecord
	repo.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, rec repository.IdentityRecord) (repository.IdentityRecord, error) {
			inserted = rec
			return stored, nil
		},
	)

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

func TestIdentityService_Get(t *testing.T) {
	svc, repo := newTestService(t)
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)

	got, err := svc.Get(t.Context(), stored.ID)
	require.NoError(t, err)

	assert.Equal(t, storedIdentity, got)
}

func TestIdentityService_List(t *testing.T) {
	svc, repo := newTestService(t)
	// The repository is asked for one more record than the page holds.
	repo.EXPECT().List(mock.Anything, 10, 3).Return([]repository.IdentityRecord{stored, stored}, nil)

	got, err := svc.List(t.Context(), 10, 2)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Identity]{Items: []Identity{storedIdentity, storedIdentity}}, got)
}

func TestIdentityService_List_HasMore(t *testing.T) {
	svc, repo := newTestService(t)
	repo.EXPECT().List(mock.Anything, 10, 3).Return([]repository.IdentityRecord{stored, stored, stored}, nil)

	got, err := svc.List(t.Context(), 10, 2)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Identity]{Items: []Identity{storedIdentity, storedIdentity}, HasMore: true}, got)
}

func TestIdentityService_List_Empty(t *testing.T) {
	svc, repo := newTestService(t)
	repo.EXPECT().List(mock.Anything, 0, 6).Return(nil, nil)

	got, err := svc.List(t.Context(), 0, 5)
	require.NoError(t, err)

	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
}

func TestIdentityService_Replace(t *testing.T) {
	svc, repo := newTestService(t)
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

func TestIdentityService_Delete(t *testing.T) {
	svc, repo := newTestService(t)
	repo.EXPECT().Delete(mock.Anything, stored.ID).Return(nil)

	assert.NoError(t, svc.Delete(t.Context(), stored.ID))
}

func TestIdentityService_TranslatesRepositoryErrors(t *testing.T) {
	type call func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error

	create := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(repository.IdentityRecord{}, repoErr)
		_, err := svc.Create(ctx, actor, input)
		return err
	}
	get := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.IdentityRecord{}, repoErr)
		_, err := svc.Get(ctx, stored.ID)
		return err
	}
	list := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().List(mock.Anything, 0, 6).Return(nil, repoErr)
		_, err := svc.List(ctx, 0, 5)
		return err
	}
	replace := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().Update(mock.Anything, mock.Anything).Return(repository.IdentityRecord{}, repoErr)
		_, err := svc.Replace(ctx, actor, stored.ID, input)
		return err
	}
	del := func(ctx context.Context, svc *identityService, repo *mocks.MockIdentityRepository, repoErr error) error {
		repo.EXPECT().Delete(mock.Anything, stored.ID).Return(repoErr)
		return svc.Delete(ctx, stored.ID)
	}

	tests := []struct {
		name    string
		call    call
		repoErr error
		wantErr error
	}{
		{name: "create duplicate username", call: create, repoErr: repository.ErrDuplicateUsername, wantErr: ErrUsernameTaken},
		{name: "create unexpected", call: create, repoErr: errDB, wantErr: errDB},
		{name: "get not found", call: get, repoErr: repository.ErrNotFound, wantErr: ErrIdentityNotFound},
		{name: "get unexpected", call: get, repoErr: errDB, wantErr: errDB},
		{name: "list unexpected", call: list, repoErr: errDB, wantErr: errDB},
		{name: "replace not found", call: replace, repoErr: repository.ErrNotFound, wantErr: ErrIdentityNotFound},
		{name: "replace duplicate username", call: replace, repoErr: repository.ErrDuplicateUsername, wantErr: ErrUsernameTaken},
		{name: "replace unexpected", call: replace, repoErr: errDB, wantErr: errDB},
		{name: "delete not found", call: del, repoErr: repository.ErrNotFound, wantErr: ErrIdentityNotFound},
		{name: "delete unexpected", call: del, repoErr: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService(t)
			// The repository wraps its errors with context.
			err := tt.call(t.Context(), svc, repo, fmt.Errorf("query: %w", tt.repoErr))

			assert.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr != tt.repoErr {
				assert.NotErrorIs(t, err, tt.repoErr, "repository errors must not leak into the domain")
			}
		})
	}
}
