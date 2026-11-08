package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
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
	// storedGroup is stored, translated to the domain model, with every field visible.
	storedGroup = Group{
		ID:        stored.ID,
		Name:      authz.Visible("Reviewers"),
		CreatedBy: authz.Visible(stored.CreatedBy),
		UpdatedBy: authz.Visible(stored.UpdatedBy),
		CreatedAt: authz.Visible(stored.CreatedAt),
		UpdatedAt: authz.Visible(stored.UpdatedAt),
	}
	// storedResource is stored as a resource of decisions.
	storedResource = authz.Resource{Type: "Group", ID: stored.ID}

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

	// identityGroup is a group that identity is a member of.
	identityGroup = uuid.MustParse("5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b")
	// identityResource is identity as a resource of decisions, with its group.
	identityResource = authz.Resource{Type: "Identity", ID: identity, Groups: []uuid.UUID{identityGroup}}
	// readIdentity is the action of reading an identity, as the identity feature declares it.
	readIdentity = authz.NewAction("identity:read", "read")

	errDB = errors.New("connection refused")
)

// newTestService returns a service that may do everything, on a mock repository.
func newTestService(t *testing.T) (*groupService, *mocks.MockGroupRepository) {
	t.Helper()

	return newTestServiceWith(t, authz.AllowAll())
}

// newTestServiceWith returns a service that asks authorizer, on a mock repository.
func newTestServiceWith(t *testing.T, authorizer authz.Authorizer) (*groupService, *mocks.MockGroupRepository) {
	t.Helper()

	repo := mocks.NewMockGroupRepository(t)

	return newGroupService(repo, authorizer, func() time.Time { return now }), repo
}

// newDeciding returns an authorizer that answers actor's requests with a mock decider.
func newDeciding(t *testing.T) (*authzmocks.MockAuthorizer, *authzmocks.MockDecider) {
	t.Helper()

	authorizer := authzmocks.NewMockAuthorizer(t)
	decider := authzmocks.NewMockDecider(t)
	authorizer.EXPECT().For(mock.Anything, actor).Return(decider, nil)

	return authorizer, decider
}

// expectReadable makes the repository return stored and the decider allow reading it.
func expectReadable(repo *mocks.MockGroupRepository, decider *authzmocks.MockDecider) {
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
	decider.EXPECT().Can(actionRead, storedResource).Return(true, nil)
}

// expectAllFieldsReadable makes the decider allow reading every field of the resource, once each.
func expectAllFieldsReadable(decider *authzmocks.MockDecider, resource authz.Resource) {
	servicetest.ExpectReadableFields(decider, readFields, resource, readFields.Fields()...)
}

// isReadField reports whether the action is one of reading a field of a group.
func isReadField(action authz.Action) bool {
	return slices.ContainsFunc(readFields.Actions(), func(a authz.Action) bool {
		return assert.ObjectsAreEqual(a, action)
	})
}

// expectAllowed makes the repository return stored and the decider allow reading it and the action on it.
func expectAllowed(repo *mocks.MockGroupRepository, decider *authzmocks.MockDecider, action authz.Action) {
	expectReadable(repo, decider)
	decider.EXPECT().Can(action, storedResource).Return(true, nil)
}

// newRecords returns stored records with the given IDs' last bytes, in order.
func newRecords(lastBytes ...byte) []repository.GroupRecord {
	recs := make([]repository.GroupRecord, len(lastBytes))
	for i, lastByte := range lastBytes {
		recs[i] = stored
		recs[i].ID = uuid.UUID{15: lastByte}
	}

	return recs
}

// idsOf returns the IDs' last bytes of the groups, in order.
func idsOf(groups []Group) []byte {
	ids := make([]byte, len(groups))
	for i, group := range groups {
		ids[i] = group.ID[15]
	}

	return ids
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

// TestGroupService_Create_ChecksFields checks that creating needs the action of each field, on the group to
// be created, and that a denial creates nothing.
func TestGroupService_Create_ChecksFields(t *testing.T) {
	t.Parallel()
	for _, allowed := range []bool{true, false} {
		t.Run(fmt.Sprintf("allowed %t", allowed), func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo := newTestServiceWith(t, authorizer)
			var checked authz.Resource
			decider.EXPECT().Can(authz.NewAction("group:create:name", "group:create"), mock.Anything).RunAndReturn(
				func(_ authz.Action, resource authz.Resource) (bool, error) {
					checked = resource
					return allowed, nil
				},
			)
			var inserted repository.GroupRecord
			if allowed {
				decider.EXPECT().Can(mock.Anything, mock.Anything).Return(true, nil)
				repo.EXPECT().Insert(mock.Anything, servicetest.Capture(&inserted)).Return(stored, nil)
			}

			_, err := svc.Create(t.Context(), actor, input)

			if allowed {
				require.NoError(t, err)
				assert.Equal(t, authz.Resource{Type: "Group", ID: inserted.ID}, checked,
					"the check must be on the group to be created")
				return
			}
			// The mock fails the test if the repository is called.
			var denied *authz.FieldsDeniedError
			require.ErrorAs(t, err, &denied)
			assert.Equal(t, []string{"name"}, denied.Fields)
		})
	}
}

// TestGroupService_Create_ReadableFields checks that the new group is returned with the fields that the
// actor may read, and with only its ID if it may not read the group.
func TestGroupService_Create_ReadableFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		canRead  bool
		readable []string
		want     Group
	}{
		{name: "all", canRead: true, readable: readFields.Fields(), want: storedGroup},
		{
			name: "some", canRead: true, readable: []string{"name"},
			want: Group{ID: stored.ID, Name: authz.Visible("Reviewers")},
		},
		{name: "unreadable group", canRead: false, want: Group{ID: stored.ID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo := newTestServiceWith(t, authorizer)
			// The service generates the ID, so the checks match any resource. That they're on the new group
			// is checked by TestGroupService_Create_ChecksFields.
			decider.EXPECT().Can(authz.NewAction("group:create:name", "group:create"), mock.Anything).
				Return(true, nil).Once()
			decider.EXPECT().Can(actionRead, mock.Anything).Return(tt.canRead, nil).Once()
			if tt.canRead {
				for i, action := range readFields.Actions() {
					decider.EXPECT().Can(action, mock.Anything).
						Return(slices.Contains(tt.readable, readFields.Fields()[i]), nil).Once()
				}
			}
			repo.EXPECT().Insert(mock.Anything, mock.Anything).Return(stored, nil)

			got, err := svc.Create(t.Context(), actor, input)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestGroupService_ReadFieldActions checks that every field of Group but the ID has an action for reading
// it, so that no field is revealed unchecked or hidden from everyone.
func TestGroupService_ReadFieldActions(t *testing.T) {
	t.Parallel()
	var fields []string
	for field := range reflect.TypeFor[Group]().Fields() {
		if field.Name == "ID" {
			continue
		}
		name := []rune(field.Name)
		name[0] = unicode.ToLower(name[0])
		fields = append(fields, string(name))
	}

	assert.ElementsMatch(t, fields, ReadableFields())
}

// TestGroupService_FieldActions checks that every field of GroupInput, which creating and replacing set,
// has an action of its own for each, so that no field is written unchecked.
func TestGroupService_FieldActions(t *testing.T) {
	t.Parallel()
	var fields []string
	for field := range reflect.TypeFor[GroupInput]().Fields() {
		name := []rune(field.Name)
		name[0] = unicode.ToLower(name[0])
		fields = append(fields, string(name))
	}

	assert.ElementsMatch(t, fields, CreatableFields())
	assert.ElementsMatch(t, fields, UpdatableFields())
}

func TestGroupService_Get(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	expectAllFieldsReadable(decider, storedResource)

	got, err := svc.Get(t.Context(), actor, stored.ID)
	require.NoError(t, err)

	assert.Equal(t, storedGroup, got)
}

// TestGroupService_Get_HiddenFields checks that only the fields that the actor may read are visible, and
// the ID always.
func TestGroupService_Get_HiddenFields(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	servicetest.ExpectReadableFields(decider, readFields, storedResource, "name")

	got, err := svc.Get(t.Context(), actor, stored.ID)
	require.NoError(t, err)

	assert.Equal(t, Group{ID: stored.ID, Name: authz.Visible("Reviewers")}, got)
}

// TestGroupService_List_Pages cuts the pages out of what the actor may read, with hidden rows between them,
// so that only the last page can be short and HasMore counts readable rows only.
func TestGroupService_List_Pages(t *testing.T) {
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
			decider.EXPECT().Can(actionRead, mock.Anything).RunAndReturn(
				func(_ authz.Action, resource authz.Resource) (bool, error) {
					assert.Equal(t, "Group", resource.Type)
					return resource.ID[15]%2 == 0, nil
				},
			).Times(9)
			fieldChecks := map[byte]int{}
			decider.EXPECT().Can(mock.MatchedBy(isReadField), mock.Anything).RunAndReturn(
				func(_ authz.Action, resource authz.Resource) (bool, error) {
					fieldChecks[resource.ID[15]]++
					return true, nil
				},
			).Maybe()

			got, err := svc.List(t.Context(), actor, tt.offset, tt.limit)
			require.NoError(t, err)

			assert.Equal(t, tt.wantIDs, idsOf(got.Items))
			assert.Equal(t, tt.wantHasMore, got.HasMore)
			wantFieldChecks := map[byte]int{}
			for _, id := range tt.wantIDs {
				wantFieldChecks[id] = len(readFields.Fields())
			}
			assert.Equal(t, wantFieldChecks, fieldChecks, "the fields of the rows on the page must be checked, and only those")
		})
	}
}

func TestGroupService_List_TranslatesRecords(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().ListAll(mock.Anything).Return([]repository.GroupRecord{stored}, nil)

	got, err := svc.List(t.Context(), actor, 0, 10)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Group]{Items: []Group{storedGroup}}, got)
}

func TestGroupService_List_NothingReadable(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	repo.EXPECT().ListAll(mock.Anything).Return(newRecords(1, 2), nil)
	decider.EXPECT().Can(actionRead, mock.Anything).Return(false, nil)

	got, err := svc.List(t.Context(), actor, 0, 5)
	require.NoError(t, err)

	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
}

func TestGroupService_List_Empty(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().ListAll(mock.Anything).Return(nil, nil)

	got, err := svc.List(t.Context(), actor, 0, 5)
	require.NoError(t, err)

	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
}

func TestGroupService_List_ChecksContextBetweenRows(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	repo.EXPECT().ListAll(mock.Anything).Return(newRecords(1, 2, 3), nil)
	decider.EXPECT().Can(actionRead, mock.Anything).RunAndReturn(
		func(authz.Action, authz.Resource) (bool, error) {
			cancel()
			return true, nil
		},
	).Once()

	_, err := svc.List(ctx, actor, 0, 5)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestGroupService_Replace(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectAllowed(repo, decider, authz.NewAction("group:update:name", "group:update"))
	servicetest.ExpectReadableFields(decider, readFields, storedResource, "name")
	repo.EXPECT().Update(mock.Anything, repository.GroupRecord{
		ID:        stored.ID,
		Name:      "Maintainers",
		UpdatedBy: actor,
		UpdatedAt: now,
	}).Return(stored, nil)

	got, err := svc.Replace(t.Context(), actor, stored.ID, input)
	require.NoError(t, err)

	// The response to a write is masked like a read.
	assert.Equal(t, Group{ID: stored.ID, Name: authz.Visible("Reviewers")}, got)
}

// TestGroupService_Replace_FieldsDenied checks that replacing checks every field, even one whose value
// doesn't change, and that a denial writes nothing.
func TestGroupService_Replace_FieldsDenied(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mock fails the test if the repository updates anything.
	svc, repo := newTestServiceWith(t, authorizer)
	expectReadable(repo, decider)
	decider.EXPECT().Can(authz.NewAction("group:update:name", "group:update"), storedResource).Return(false, nil)

	_, err := svc.Replace(t.Context(), actor, stored.ID, GroupInput{Name: stored.Name})

	var denied *authz.FieldsDeniedError
	require.ErrorAs(t, err, &denied)
	assert.Equal(t, []string{"name"}, denied.Fields)
}

func TestGroupService_Delete(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectAllowed(repo, decider, actionDelete)
	repo.EXPECT().Delete(mock.Anything, stored.ID).Return(nil)

	assert.NoError(t, svc.Delete(t.Context(), actor, stored.ID))
}

func TestGroupService_AddMember(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectAllowed(repo, decider, actionAddMember)
	repo.EXPECT().ListIdentityGroups(mock.Anything, identity).Return([]uuid.UUID{identityGroup}, nil)
	// The identity carries its groups, so that a policy on them applies.
	decider.EXPECT().Can(readIdentity, identityResource).Return(true, nil)
	repo.EXPECT().InsertMember(mock.Anything, repository.MemberRecord{
		GroupID:    stored.ID,
		IdentityID: identity,
		CreatedBy:  actor,
		CreatedAt:  now,
	}).Return(nil)

	assert.NoError(t, svc.AddMember(t.Context(), actor, stored.ID, identity))
}

// TestGroupService_AddMember_HiddenIdentity checks that adding an identity that the actor may not read is
// reported like adding a missing one, without writing anything, so that it can't reveal that the identity
// exists.
func TestGroupService_AddMember_HiddenIdentity(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	// The mock fails the test if the repository writes anything.
	svc, repo := newTestServiceWith(t, authorizer)
	expectAllowed(repo, decider, actionAddMember)
	repo.EXPECT().ListIdentityGroups(mock.Anything, identity).Return([]uuid.UUID{identityGroup}, nil)
	decider.EXPECT().Can(readIdentity, identityResource).Return(false, nil)

	err := svc.AddMember(t.Context(), actor, stored.ID, identity)

	require.ErrorIs(t, err, ErrIdentityNotFound)
	assert.NotErrorIs(t, err, authz.ErrDenied)
}

func TestGroupService_ListMembers(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	// The members aren't checked one by one: reading the members covers them all.
	expectAllowed(repo, decider, actionReadMembers)
	// The repository is asked for one more record than the page holds.
	repo.EXPECT().ListMembers(mock.Anything, stored.ID, 10, 3).
		Return([]repository.MemberRecord{storedMember, storedMember}, nil)

	got, err := svc.ListMembers(t.Context(), actor, stored.ID, 10, 2)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Member]{Items: []Member{member, member}}, got)
}

func TestGroupService_ListMembers_HasMore(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
	repo.EXPECT().ListMembers(mock.Anything, stored.ID, 10, 3).
		Return([]repository.MemberRecord{storedMember, storedMember, storedMember}, nil)

	got, err := svc.ListMembers(t.Context(), actor, stored.ID, 10, 2)
	require.NoError(t, err)

	assert.Equal(t, pagination.Page[Member]{Items: []Member{member, member}, HasMore: true}, got)
}

func TestGroupService_ListMembers_Empty(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService(t)
	repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
	repo.EXPECT().ListMembers(mock.Anything, stored.ID, 0, 6).Return(nil, nil)

	got, err := svc.ListMembers(t.Context(), actor, stored.ID, 0, 5)
	require.NoError(t, err)

	assert.Empty(t, got.Items)
	assert.False(t, got.HasMore)
}

func TestGroupService_RemoveMember(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, repo := newTestServiceWith(t, authorizer)
	expectAllowed(repo, decider, actionRemoveMember)
	repo.EXPECT().DeleteMember(mock.Anything, stored.ID, identity).Return(nil)

	assert.NoError(t, svc.RemoveMember(t.Context(), actor, stored.ID, identity))
}

// calls returns a call of each method on stored that checks reading it first, by name.
func calls(t *testing.T) map[string]func(*groupService) error {
	t.Helper()

	return map[string]func(*groupService) error{
		"get": func(svc *groupService) error {
			_, err := svc.Get(t.Context(), actor, stored.ID)
			return err
		},
		"replace": func(svc *groupService) error {
			_, err := svc.Replace(t.Context(), actor, stored.ID, input)
			return err
		},
		"delete": func(svc *groupService) error {
			return svc.Delete(t.Context(), actor, stored.ID)
		},
		"add member": func(svc *groupService) error {
			return svc.AddMember(t.Context(), actor, stored.ID, identity)
		},
		"list members": func(svc *groupService) error {
			_, err := svc.ListMembers(t.Context(), actor, stored.ID, 0, 5)
			return err
		},
		"remove member": func(svc *groupService) error {
			return svc.RemoveMember(t.Context(), actor, stored.ID, identity)
		},
	}
}

// TestGroupService_Hidden checks that every method on a group that the actor may not read reports it like a
// missing group, before anything else is checked: whether the actor may do the action would otherwise reveal
// that the group exists.
func TestGroupService_Hidden(t *testing.T) {
	t.Parallel()
	for name, call := range calls(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mocks fail the test on any other decision or on a write.
			svc, repo := newTestServiceWith(t, authorizer)
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
			decider.EXPECT().Can(actionRead, storedResource).Return(false, nil)

			err := call(svc)

			require.ErrorIs(t, err, ErrGroupNotFound)
			assert.NotErrorIs(t, err, authz.ErrDenied)
		})
	}
}

// TestGroupService_ActionDenied checks that an action on a readable group that the actor may not do is
// denied, without anything else being checked or written.
func TestGroupService_ActionDenied(t *testing.T) {
	t.Parallel()
	actions := map[string]authz.Action{
		"delete":        actionDelete,
		"add member":    actionAddMember,
		"list members":  actionReadMembers,
		"remove member": actionRemoveMember,
	}
	for name, call := range calls(t) {
		action, ok := actions[name]
		if !ok {
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			// The mocks fail the test on any other decision or on a write.
			svc, repo := newTestServiceWith(t, authorizer)
			expectReadable(repo, decider)
			decider.EXPECT().Can(action, storedResource).Return(false, nil)

			err := call(svc)

			require.ErrorIs(t, err, authz.ErrDenied)
			assert.NotErrorIs(t, err, ErrGroupNotFound)
		})
	}
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
		_, err := svc.Get(ctx, actor, stored.ID)
		return err
	}
	list := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().ListAll(mock.Anything).Return(nil, repoErr)
		_, err := svc.List(ctx, actor, 0, 5)
		return err
	}
	replaceGet := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.GroupRecord{}, repoErr)
		_, err := svc.Replace(ctx, actor, stored.ID, input)
		return err
	}
	replace := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
		repo.EXPECT().Update(mock.Anything, mock.Anything).Return(repository.GroupRecord{}, repoErr)
		_, err := svc.Replace(ctx, actor, stored.ID, input)
		return err
	}
	delGet := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.GroupRecord{}, repoErr)
		return svc.Delete(ctx, actor, stored.ID)
	}
	del := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
		repo.EXPECT().Delete(mock.Anything, stored.ID).Return(repoErr)
		return svc.Delete(ctx, actor, stored.ID)
	}
	addMemberGet := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.GroupRecord{}, repoErr)
		return svc.AddMember(ctx, actor, stored.ID, identity)
	}
	addMemberGroups := func(
		ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error,
	) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
		repo.EXPECT().ListIdentityGroups(mock.Anything, identity).Return(nil, repoErr)
		return svc.AddMember(ctx, actor, stored.ID, identity)
	}
	addMember := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
		repo.EXPECT().ListIdentityGroups(mock.Anything, identity).Return(nil, nil)
		repo.EXPECT().InsertMember(mock.Anything, mock.Anything).Return(repoErr)
		return svc.AddMember(ctx, actor, stored.ID, identity)
	}
	listMembersGet := func(
		ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error,
	) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.GroupRecord{}, repoErr)
		_, err := svc.ListMembers(ctx, actor, stored.ID, 0, 5)
		return err
	}
	listMembers := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
		repo.EXPECT().ListMembers(mock.Anything, stored.ID, 0, 6).Return(nil, repoErr)
		_, err := svc.ListMembers(ctx, actor, stored.ID, 0, 5)
		return err
	}
	removeMemberGet := func(
		ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error,
	) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(repository.GroupRecord{}, repoErr)
		return svc.RemoveMember(ctx, actor, stored.ID, identity)
	}
	removeMember := func(ctx context.Context, svc *groupService, repo *mocks.MockGroupRepository, repoErr error) error {
		repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil)
		repo.EXPECT().DeleteMember(mock.Anything, stored.ID, identity).Return(repoErr)
		return svc.RemoveMember(ctx, actor, stored.ID, identity)
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
		// Answered with a logged 500 like any other unexpected error.
		{name: "list over the ceiling", call: list, repoErr: repository.ErrTooManyRows, wantErr: repository.ErrTooManyRows},
		{name: "replace get not found", call: replaceGet, repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound},
		{name: "replace get unexpected", call: replaceGet, repoErr: errDB, wantErr: errDB},
		{name: "replace not found", call: replace, repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound},
		{name: "replace name taken", call: replace, repoErr: repository.ErrDuplicateName, wantErr: ErrGroupNameTaken},
		{name: "replace unexpected", call: replace, repoErr: errDB, wantErr: errDB},
		{name: "delete get not found", call: delGet, repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound},
		{name: "delete get unexpected", call: delGet, repoErr: errDB, wantErr: errDB},
		{name: "delete not found", call: del, repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound},
		{name: "delete unexpected", call: del, repoErr: errDB, wantErr: errDB},
		{
			name: "add member get not found", call: addMemberGet,
			repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound,
		},
		{name: "add member get unexpected", call: addMemberGet, repoErr: errDB, wantErr: errDB},
		{name: "add member identity groups unexpected", call: addMemberGroups, repoErr: errDB, wantErr: errDB},
		// The group was deleted after it was read.
		{name: "add member group not found", call: addMember, repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound},
		{
			name: "add member identity not found", call: addMember,
			repoErr: repository.ErrIdentityNotFound, wantErr: ErrIdentityNotFound,
		},
		{name: "add member unexpected", call: addMember, repoErr: errDB, wantErr: errDB},
		{
			name: "list members get not found", call: listMembersGet,
			repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound,
		},
		{name: "list members get unexpected", call: listMembersGet, repoErr: errDB, wantErr: errDB},
		{name: "list members unexpected", call: listMembers, repoErr: errDB, wantErr: errDB},
		{
			name: "remove member get not found", call: removeMemberGet,
			repoErr: repository.ErrNotFound, wantErr: ErrGroupNotFound,
		},
		{name: "remove member get unexpected", call: removeMemberGet, repoErr: errDB, wantErr: errDB},
		{
			name: "remove member not found", call: removeMember,
			repoErr: repository.ErrMemberNotFound, wantErr: ErrMemberNotFound,
		},
		{name: "remove member unexpected", call: removeMember, repoErr: errDB, wantErr: errDB},
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

// TestGroupService_AuthorizationFails checks that failing to decide is an error and never an answer: not
// access, and not a missing group or identity, which would hide a broken policy from the logs.
func TestGroupService_AuthorizationFails(t *testing.T) {
	t.Parallel()
	errDecision := errors.New("policy failed")

	all := calls(t)
	all["create"] = func(svc *groupService) error {
		_, err := svc.Create(t.Context(), actor, input)
		return err
	}
	all["list"] = func(svc *groupService) error {
		_, err := svc.List(t.Context(), actor, 0, 5)
		return err
	}
	assertFailed := func(t *testing.T, err error) {
		t.Helper()
		require.ErrorIs(t, err, errDecision)
		require.NotErrorIs(t, err, ErrGroupNotFound)
		require.NotErrorIs(t, err, ErrIdentityNotFound)
		assert.NotErrorIs(t, err, authz.ErrDenied)
	}

	for name, call := range all {
		t.Run(name+" no decider", func(t *testing.T) {
			t.Parallel()
			authorizer := authzmocks.NewMockAuthorizer(t)
			authorizer.EXPECT().For(mock.Anything, actor).Return(nil, errDecision)
			// The mock fails the test if the repository is called.
			svc, _ := newTestServiceWith(t, authorizer)

			assertFailed(t, call(svc))
		})
		t.Run(name+" no decision", func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			decider.EXPECT().Can(mock.Anything, mock.Anything).Return(false, errDecision)
			// The mock fails the test if the repository writes anything.
			svc, repo := newTestServiceWith(t, authorizer)
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil).Maybe()
			repo.EXPECT().ListAll(mock.Anything).Return([]repository.GroupRecord{stored}, nil).Maybe()

			assertFailed(t, call(svc))
		})
	}

	// The decisions after the actor was allowed to read the group.
	for name, call := range calls(t) {
		if name == "get" {
			continue
		}
		t.Run(name+" no action decision", func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			svc, repo := newTestServiceWith(t, authorizer)
			expectReadable(repo, decider)
			decider.EXPECT().Can(mock.Anything, storedResource).Return(false, errDecision)

			assertFailed(t, call(svc))
		})
	}

	// The field read decisions, after every other decision allowed the request. They come before any write,
	// so that a failed one can't fail a request whose write succeeded.
	for _, name := range []string{"create", "get", "list", "replace"} {
		t.Run(name+" no field decision", func(t *testing.T) {
			t.Parallel()
			authorizer, decider := newDeciding(t)
			decider.EXPECT().Can(mock.MatchedBy(isReadField), mock.Anything).Return(false, errDecision)
			decider.EXPECT().Can(mock.Anything, mock.Anything).Return(true, nil).Maybe()
			// The mock fails the test if the repository writes anything.
			svc, repo := newTestServiceWith(t, authorizer)
			repo.EXPECT().Get(mock.Anything, stored.ID).Return(stored, nil).Maybe()
			repo.EXPECT().ListAll(mock.Anything).Return([]repository.GroupRecord{stored}, nil).Maybe()

			assertFailed(t, all[name](svc))
		})
	}

	t.Run("add member no identity decision", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		// The mock fails the test if the repository writes anything.
		svc, repo := newTestServiceWith(t, authorizer)
		expectAllowed(repo, decider, actionAddMember)
		repo.EXPECT().ListIdentityGroups(mock.Anything, identity).Return(nil, nil)
		decider.EXPECT().Can(readIdentity, mock.Anything).Return(false, errDecision)

		assertFailed(t, svc.AddMember(t.Context(), actor, stored.ID, identity))
	})
}
