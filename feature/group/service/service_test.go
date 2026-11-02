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
	"pkgsilo/feature/group/repository"
	"pkgsilo/feature/group/repository/mocks"
	"pkgsilo/test/servicetest"
)

var (
	now      = time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)
	actor    = uuid.MustParse("6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b")
	input    = GroupInput{Name: "Maintainers"}
	identity = uuid.MustParse("3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f")

	// stored is a record as the repository returns it, with a distinct value in every field.
	stored = repository.GroupRecord{
		ID:        uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		Name:      "Reviewers",
		CreatedBy: uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"),
		UpdatedBy: uuid.MustParse("2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e"),
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-time.Minute),
	}
	// storedGroup is stored, translated to the domain model.
	storedGroup = Group{
		ID:        stored.ID,
		Name:      "Reviewers",
		CreatedBy: stored.CreatedBy,
		UpdatedBy: stored.UpdatedBy,
		CreatedAt: stored.CreatedAt,
		UpdatedAt: stored.UpdatedAt,
	}

	// storedMember is a member record of stored as the repository returns it.
	storedMember = repository.MemberRecord{
		GroupID:    stored.ID,
		IdentityID: identity,
		CreatedBy:  uuid.MustParse("4d5e6f7a-8b9c-4d0e-9f1a-2b3c4d5e6f7a"),
		CreatedAt:  now.Add(-time.Second),
	}
	// member is storedMember, translated to the domain model.
	member = Member{
		IdentityID: identity,
		CreatedBy:  storedMember.CreatedBy,
		CreatedAt:  storedMember.CreatedAt,
	}

	errDB = errors.New("connection refused")
)

func newTestService(t *testing.T) (*groupService, *mocks.MockGroupRepository) {
	t.Helper()

	repo := mocks.NewMockGroupRepository(t)

	return newGroupService(repo, func() time.Time { return now }), repo
}

func TestGroupService_Create(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	var inserted repository.GroupRecord
	repo.EXPECT().Insert(mock.Anything, servicetest.Capture(&inserted)).Return(stored, nil)

	got, err := svc.Create(t.Context(), actor, input)
	require.NoError(t, err)

	assert.Equal(t, byte(4), inserted.ID[6]>>4, "ID must be a version 4 UUID")
	assert.Equal(t, repository.GroupRecord{
		ID:        inserted.ID,
		Name:      "Maintainers",
		CreatedBy: actor,
		UpdatedBy: actor,
		CreatedAt: now,
		UpdatedAt: now,
	}, inserted)
	// The group as stored is returned, not the one sent to the repository.
	assert.Equal(t, storedGroup, got)
}

func TestGroupService_Create_GeneratesUniqueIDs(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	ids := map[uuid.UUID]bool{}
	repo.EXPECT().Insert(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, rec repository.GroupRecord) (repository.GroupRecord, error) {
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

func TestGroupService_Get(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)

	got, err := svc.Get(t.Context(), stored.ID)
	require.NoError(t, err)

	assert.Equal(t, storedGroup, got)
}

func TestGroupService_List(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	// The repository is asked for one more record than the page holds.
	repo.EXPECT().List(mock.Anything, 10, 3).Return([]repository.GroupRecord{stored, stored}, nil)

	got, err := svc.List(t.Context(), 10, 2)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Group]{Items: []Group{storedGroup, storedGroup}}, got)
}

func TestGroupService_List_HasMore(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().List(mock.Anything, 10, 3).Return([]repository.GroupRecord{stored, stored, stored}, nil)

	got, err := svc.List(t.Context(), 10, 2)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Group]{Items: []Group{storedGroup, storedGroup}, HasMore: true}, got)
}

func TestGroupService_List_Empty(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().List(mock.Anything, 0, 6).Return(nil, nil)

	got, err := svc.List(t.Context(), 0, 5)
	require.NoError(t, err)

	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
}

func TestGroupService_Replace(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().Update(mock.Anything, repository.GroupRecord{
		ID:        stored.ID,
		Name:      "Maintainers",
		UpdatedBy: actor,
		UpdatedAt: now,
	}).Return(stored, nil)

	got, err := svc.Replace(t.Context(), actor, stored.ID, input)
	require.NoError(t, err)

	assert.Equal(t, storedGroup, got)
}

func TestGroupService_Delete(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().Delete(mock.Anything, stored.ID).Return(nil)

	assert.NoError(t, svc.Delete(t.Context(), stored.ID))
}

func TestGroupService_AddMember(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().InsertMember(mock.Anything, repository.MemberRecord{
		GroupID:    stored.ID,
		IdentityID: identity,
		CreatedBy:  actor,
		CreatedAt:  now,
	}).Return(nil)

	assert.NoError(t, svc.AddMember(t.Context(), actor, stored.ID, identity))
}

func TestGroupService_ListMembers(t *testing.T) {
	t.Parallel()
	// The mock fails the test if the group is looked up: its member records prove it exists.
	svc, repo := newTestService(t)
	// The repository is asked for one more record than the page holds.
	repo.EXPECT().ListMembers(mock.Anything, stored.ID, 10, 3).
		Return([]repository.MemberRecord{storedMember, storedMember}, nil)

	got, err := svc.ListMembers(t.Context(), stored.ID, 10, 2)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Member]{Items: []Member{member, member}}, got)
}

func TestGroupService_ListMembers_HasMore(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().ListMembers(mock.Anything, stored.ID, 10, 3).
		Return([]repository.MemberRecord{storedMember, storedMember, storedMember}, nil)

	got, err := svc.ListMembers(t.Context(), stored.ID, 10, 2)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Member]{Items: []Member{member, member}, HasMore: true}, got)
}

func TestGroupService_ListMembers_Empty(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	list := repo.EXPECT().ListMembers(mock.Anything, stored.ID, 0, 6).Return(nil, nil)
	// Only after the member records, so that a group deleted in between is reported as missing.
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil).NotBefore(list.Call)

	got, err := svc.ListMembers(t.Context(), stored.ID, 0, 5)
	require.NoError(t, err)

	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
}

func TestGroupService_RemoveMember(t *testing.T) {
	t.Parallel()
	// The mock fails the test if the group is looked up: the removed member record proves it exists.
	svc, repo := newTestService(t)
	repo.EXPECT().DeleteMember(mock.Anything, stored.ID, identity).Return(nil)

	assert.NoError(t, svc.RemoveMember(t.Context(), stored.ID, identity))
}

func TestGroupService_TranslatesRepositoryErrors(t *testing.T) {
	t.Parallel()
	type call func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error

	create := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(repository.GroupRecord{}, repoErr)
		_, err := svc.Create(ctx, actor, input)
		return err
	}
	get := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.GroupRecord{}, repoErr)
		_, err := svc.Get(ctx, stored.ID)
		return err
	}
	list := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().List(mock.Anything, 0, 6).Return(nil, repoErr)
		_, err := svc.List(ctx, 0, 5)
		return err
	}
	replace := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Update(mock.Anything, mock.Anything).Return(repository.GroupRecord{}, repoErr)
		_, err := svc.Replace(ctx, actor, stored.ID, input)
		return err
	}
	del := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Delete(mock.Anything, stored.ID).Return(repoErr)
		return svc.Delete(ctx, stored.ID)
	}
	addMember := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().InsertMember(mock.Anything, mock.Anything).Return(repoErr)
		return svc.AddMember(ctx, actor, stored.ID, identity)
	}
	addMissingIdentity := func(
		ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error,
	) error {
		insert := repo.EXPECT().InsertMember(mock.Anything, mock.Anything).Return(repoErr)
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil).NotBefore(insert.Call)
		return svc.AddMember(ctx, actor, stored.ID, identity)
	}
	// The database may report a missing identity although the group is missing too.
	addMissingIdentityToMissingGroup := func(
		ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error,
	) error {
		insert := repo.EXPECT().InsertMember(mock.Anything, mock.Anything).
			Return(fmt.Errorf("query: %w", repository.ErrIdentityNotFound))
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.GroupRecord{}, repoErr).NotBefore(insert.Call)
		return svc.AddMember(ctx, actor, stored.ID, identity)
	}
	listMembers := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().ListMembers(mock.Anything, stored.ID, 0, 6).Return(nil, repoErr)
		_, err := svc.ListMembers(ctx, stored.ID, 0, 5)
		return err
	}
	listMembersOfMissingGroup := func(
		ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error,
	) error {
		list := repo.EXPECT().ListMembers(mock.Anything, stored.ID, 0, 6).Return(nil, nil)
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.GroupRecord{}, repoErr).NotBefore(list.Call)
		_, err := svc.ListMembers(ctx, stored.ID, 0, 5)
		return err
	}
	removeMember := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().DeleteMember(mock.Anything, stored.ID, identity).Return(repoErr)
		return svc.RemoveMember(ctx, stored.ID, identity)
	}
	removeNonMember := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		del := repo.EXPECT().DeleteMember(mock.Anything, stored.ID, identity).Return(repoErr)
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil).NotBefore(del.Call)
		return svc.RemoveMember(ctx, stored.ID, identity)
	}
	removeMemberOfMissingGroup := func(
		ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error,
	) error {
		del := repo.EXPECT().DeleteMember(mock.Anything, stored.ID, identity).
			Return(fmt.Errorf("query: %w", repository.ErrMemberNotFound))
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.GroupRecord{}, repoErr).NotBefore(del.Call)
		return svc.RemoveMember(ctx, stored.ID, identity)
	}

	tests := []struct {
		name    string
		call    call
		repoErr error
		wantErr error
	}{
		{name: "create name taken", call: create, repoErr: repository.ErrDuplicateName, wantErr: ErrGroupNameTaken},
		{name: "create unexpected", call: create, repoErr: errDB, wantErr: errDB},
		{name: "get not found", call: get, repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound},
		{name: "get unexpected", call: get, repoErr: errDB, wantErr: errDB},
		{name: "list unexpected", call: list, repoErr: errDB, wantErr: errDB},
		{name: "replace not found", call: replace, repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound},
		{name: "replace name taken", call: replace, repoErr: repository.ErrDuplicateName, wantErr: ErrGroupNameTaken},
		{name: "replace unexpected", call: replace, repoErr: errDB, wantErr: errDB},
		{name: "delete not found", call: del, repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound},
		{name: "delete unexpected", call: del, repoErr: errDB, wantErr: errDB},
		{name: "add member group not found", call: addMember, repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound},
		{name: "add member unexpected", call: addMember, repoErr: errDB, wantErr: errDB},
		{
			name: "add member identity not found", call: addMissingIdentity,
			repoErr: repository.ErrIdentityNotFound, wantErr: ErrIdentityNotFound,
		},
		{
			name: "add member group and identity not found", call: addMissingIdentityToMissingGroup,
			repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound,
		},
		{
			name: "add member identity not found, group unexpected", call: addMissingIdentityToMissingGroup,
			repoErr: errDB, wantErr: errDB,
		},
		{name: "list members unexpected", call: listMembers, repoErr: errDB, wantErr: errDB},
		{
			name: "list members group not found", call: listMembersOfMissingGroup,
			repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound,
		},
		{name: "list members group unexpected", call: listMembersOfMissingGroup, repoErr: errDB, wantErr: errDB},
		{name: "remove member unexpected", call: removeMember, repoErr: errDB, wantErr: errDB},
		{
			name: "remove member not found", call: removeNonMember,
			repoErr: repository.ErrMemberNotFound, wantErr: ErrMemberNotFound,
		},
		{
			name: "remove member group not found", call: removeMemberOfMissingGroup,
			repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound,
		},
		{name: "remove member group unexpected", call: removeMemberOfMissingGroup, repoErr: errDB, wantErr: errDB},
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
