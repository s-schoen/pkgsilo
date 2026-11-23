package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/authz"
	authzmocks "pkgsilo/core/authz/mocks"
	"pkgsilo/test/servicetest"
)

// The test types: one that isn't owned, one owned many times per owner and one owned once per owner, each with
// a create field, a read field and a further action where it matters.
type (
	testModel struct {
		ID             uuid.UUID
		Name           authz.Field[string] `authz:"name"`
		AllowedActions authz.AllowedActions
	}
	testInput struct {
		Name string `authz:"name"`
	}
)

var (
	widgets = authz.DeclareType(authz.TypeSpec{
		Type: "Widget", Model: reflect.TypeFor[testModel](), Create: reflect.TypeFor[testInput](),
		Update: reflect.TypeFor[testInput](), Actions: []authz.Action{authz.NewAction("widget:spin")},
	})
	keys = authz.DeclareType(authz.TypeSpec{
		Type: "Key", Owned: true, Model: reflect.TypeFor[testModel](), Create: reflect.TypeFor[testInput](),
	})
	secrets = authz.DeclareType(authz.TypeSpec{
		Type: "Secret", Owned: true, OnePerOwner: true, Model: reflect.TypeFor[testModel](),
		Actions: []authz.Action{authz.NewAction("secret:reset")},
	})
	types = []authz.TypeActions{widgets, keys, secrets}

	actor   = uuid.MustParse("6f1c2a3b-4d5e-4f60-8a7b-9c0d1e2f3a4b")
	otherID = uuid.MustParse("7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c")
	// owner is otherID as the owner of keys and secrets.
	owner = &authz.Owner{ID: otherID, Groups: []uuid.UUID{uuid.MustParse("5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b")}}

	errDB = errors.New("connection refused")
)

// newTestService returns a service on the test types that asks authorizer, with a mock owner loader.
func newTestService(t *testing.T, authorizer authz.Authorizer) (PermissionService, *authzmocks.MockOwnerLoader) {
	t.Helper()
	owners := authzmocks.NewMockOwnerLoader(t)

	return NewPermissionService(types, authorizer, owners), owners
}

// newDeciding returns an authorizer that answers actor's requests with a mock decider.
func newDeciding(t *testing.T) (*authzmocks.MockAuthorizer, *authzmocks.MockDecider) {
	t.Helper()
	authorizer := authzmocks.NewMockAuthorizer(t)
	decider := authzmocks.NewMockDecider(t)
	authorizer.EXPECT().For(mock.Anything, actor).Return(decider, nil)

	return authorizer, decider
}

// named matches the action with the given name.
func named(name string) any {
	return mock.MatchedBy(func(action authz.Action) bool { return action.Name() == name })
}

// fresh matches a new resource of the type: a fresh ID, in no group and without an owner.
func fresh(resourceType string) any {
	return mock.MatchedBy(func(r authz.Resource) bool {
		return r.Type == resourceType && r.ID != uuid.UUID{} && r.Groups == nil && r.Owner == nil
	})
}

// ownedBy matches a resource of the type that belongs to owner: the owner's one resource for a OnePerOwner
// type, a resource with a fresh ID otherwise.
func ownedBy(resourceType string, onePerOwner bool) any {
	return mock.MatchedBy(func(r authz.Resource) bool {
		return r.Type == resourceType && r.Owner == owner && (r.ID == owner.ID) == onePerOwner
	})
}

// TestList_AllowAll checks that an actor that may do everything gets every declared action of every type,
// in the order of the types and their actions, and the owned types' actions for an owner.
func TestList_AllowAll(t *testing.T) {
	t.Parallel()

	t.Run("without owner", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestService(t, authz.AllowAll())

		got, err := svc.List(t.Context(), actor, actor, nil)

		require.NoError(t, err)
		assert.Equal(t, Permissions{ID: actor, AllowedActions: authz.AllowedActions{
			"widget:read", "widget:delete", "widget:spin", "widget:read:name", "widget:create:name",
			"widget:update:name",
			"key:read", "key:delete", "key:read:name", "key:create:name",
			"secret:read", "secret:delete", "secret:reset", "secret:read:name",
		}}, got)
	})

	t.Run("with owner", func(t *testing.T) {
		t.Parallel()
		svc, owners := newTestService(t, authz.AllowAll())
		owners.EXPECT().ReadableOwner(mock.Anything, mock.Anything, otherID).Return(owner, nil)

		got, err := svc.List(t.Context(), actor, actor, &otherID)

		require.NoError(t, err)
		assert.Equal(t, Permissions{ID: actor, AllowedActions: authz.AllowedActions{
			"key:create:name",
			"secret:read", "secret:delete", "secret:reset", "secret:read:name",
		}}, got, "the creates of an owned type, every action of a OnePerOwner one")
	})
}

// TestList_WithoutOwner checks that the creates of a type that isn't owned are decided exactly on a new
// resource, and every other action, the creates of owned types too, on some resource, all as hints.
func TestList_WithoutOwner(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, _ := newTestService(t, authorizer)
	allowed := map[string]bool{"widget:spin": true, "widget:create:name": true, "key:create:name": true}
	decider.EXPECT().Hint(named("widget:create:name"), fresh("Widget")).Return(true, nil).Once()
	for _, t := range types {
		for _, action := range t.Actions() {
			if action.Name() == "widget:create:name" {
				continue
			}
			decider.EXPECT().CanSome(named(action.Name()), t.Type()).Return(allowed[action.Name()], nil).Once()
		}
	}

	got, err := svc.List(t.Context(), actor, actor, nil)

	require.NoError(t, err)
	assert.Equal(t, Permissions{ID: actor, AllowedActions: authz.AllowedActions{
		"widget:spin", "widget:create:name", "key:create:name",
	}}, got)
}

// TestList_WithOwner checks that the actions for an owner are decided exactly, as hints, on the resource that
// a create checks, and on the owner's one resource for a OnePerOwner type.
func TestList_WithOwner(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, owners := newTestService(t, authorizer)
	owners.EXPECT().ReadableOwner(mock.Anything, decider, otherID).Return(owner, nil)
	decider.EXPECT().Hint(named("key:create:name"), ownedBy("Key", false)).Return(true, nil).Once()
	for _, action := range secrets.Actions() {
		decider.EXPECT().Hint(named(action.Name()), ownedBy("Secret", true)).
			Return(action.Name() == "secret:reset", nil).Once()
	}

	got, err := svc.List(t.Context(), actor, actor, &otherID)

	require.NoError(t, err)
	assert.Equal(t, Permissions{ID: actor, AllowedActions: authz.AllowedActions{"key:create:name", "secret:reset"}},
		got)
}

// TestList_NothingAllowed checks that an actor that may do nothing gets an empty list, not nil.
func TestList_NothingAllowed(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, _ := newTestService(t, authorizer)
	decider.EXPECT().Hint(mock.Anything, mock.Anything).Return(false, nil)
	decider.EXPECT().CanSome(mock.Anything, mock.Anything).Return(false, nil)

	got, err := svc.List(t.Context(), actor, actor, nil)

	require.NoError(t, err)
	assert.Equal(t, Permissions{ID: actor, AllowedActions: authz.AllowedActions{}}, got)
}

// TestList_OtherIdentity checks that the permissions of another identity are denied, and that one which the
// actor may not read is reported like a missing one, without deciding anything else.
func TestList_OtherIdentity(t *testing.T) {
	t.Parallel()

	t.Run("readable", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		svc, owners := newTestService(t, authorizer)
		owners.EXPECT().ReadableOwner(mock.Anything, decider, otherID).Return(owner, nil)

		_, err := svc.List(t.Context(), actor, otherID, nil)

		require.ErrorIs(t, err, authz.ErrDenied)
	})

	t.Run("unreadable", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		svc, owners := newTestService(t, authorizer)
		owners.EXPECT().ReadableOwner(mock.Anything, decider, otherID).Return(nil, authz.ErrOwnerNotFound)

		_, err := svc.List(t.Context(), actor, otherID, nil)

		servicetest.AssertTranslated(t, err, ErrIdentityNotFound, authz.ErrOwnerNotFound)
	})

	t.Run("failing", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		svc, owners := newTestService(t, authorizer)
		owners.EXPECT().ReadableOwner(mock.Anything, decider, otherID).Return(nil, errDB)

		_, err := svc.List(t.Context(), actor, otherID, nil)

		servicetest.AssertTranslated(t, err, errDB, errDB)
	})
}

// TestList_UnreadableOwner checks that an owner that the actor may not read is reported like a missing one.
func TestList_UnreadableOwner(t *testing.T) {
	t.Parallel()
	authorizer, decider := newDeciding(t)
	svc, owners := newTestService(t, authorizer)
	owners.EXPECT().ReadableOwner(mock.Anything, decider, otherID).Return(nil, authz.ErrOwnerNotFound)

	_, err := svc.List(t.Context(), actor, actor, &otherID)

	servicetest.AssertTranslated(t, err, ErrIdentityNotFound, authz.ErrOwnerNotFound)
}

// TestList_Errors checks that a failing authorizer or decision fails the list, unchanged.
func TestList_Errors(t *testing.T) {
	t.Parallel()

	t.Run("authorizer", func(t *testing.T) {
		t.Parallel()
		authorizer := authzmocks.NewMockAuthorizer(t)
		authorizer.EXPECT().For(mock.Anything, actor).Return(nil, errDB)
		svc, _ := newTestService(t, authorizer)

		_, err := svc.List(t.Context(), actor, actor, nil)

		servicetest.AssertTranslated(t, err, errDB, errDB)
	})

	t.Run("some resource", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		svc, _ := newTestService(t, authorizer)
		decider.EXPECT().CanSome(mock.Anything, mock.Anything).Return(false, errDB).Once()

		_, err := svc.List(t.Context(), actor, actor, nil)

		servicetest.AssertTranslated(t, err, errDB, errDB)
	})

	t.Run("new resource", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		svc, _ := newTestService(t, authorizer)
		decider.EXPECT().CanSome(mock.Anything, mock.Anything).Return(false, nil)
		decider.EXPECT().Hint(mock.Anything, mock.Anything).Return(false, errDB).Once()

		_, err := svc.List(t.Context(), actor, actor, nil)

		servicetest.AssertTranslated(t, err, errDB, errDB)
	})

	t.Run("owner", func(t *testing.T) {
		t.Parallel()
		authorizer, decider := newDeciding(t)
		svc, owners := newTestService(t, authorizer)
		owners.EXPECT().ReadableOwner(mock.Anything, decider, otherID).Return(owner, nil)
		decider.EXPECT().Hint(mock.Anything, mock.Anything).Return(false, errDB).Once()

		_, err := svc.List(t.Context(), actor, actor, &otherID)

		servicetest.AssertTranslated(t, err, errDB, errDB)
	})

	t.Run("canceled", func(t *testing.T) {
		t.Parallel()
		svc, _ := newTestService(t, authz.AllowAll())
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := svc.List(ctx, actor, actor, nil)

		require.ErrorIs(t, err, context.Canceled)
	})
}
