package api

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/feature/permission/service"
	"pkgsilo/feature/permission/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

var (
	// otherID is an identity other than the actor.
	otherID = uuid.MustParse("7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c")

	permissions = service.Permissions{
		ID: apitest.Actor, AllowedActions: authz.AllowedActions{"identity:read", "identity:create:username"},
	}
	permissionsJSON = `{"id":"` + apitest.Actor.String() +
		`","allowedActions":["identity:read","identity:create:username"]}`

	actorPath = "/identities/" + apitest.Actor.String() + "/permissions"
)

// newTestClient returns a client for a handler that is backed by a mock service and mounted on the
// application router, which checks every response against Docs.
func newTestClient(t *testing.T, logger *slog.Logger) (*apitest.Client, *mocks.MockPermissionService) {
	t.Helper()

	svc := mocks.NewMockPermissionService(t)
	routes := NewHandler(svc, logger).Routes()

	return apitest.NewClient(t, apitest.Mount("/identities/{identityID}/permissions", routes)).
		WithDocs("/identities/{identityID}/permissions", Docs()), svc
}

func TestHandler_List(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		target    string
		wantOwner *uuid.UUID
	}{
		{name: "ID", target: actorPath},
		{name: "me", target: "/identities/me/permissions"},
		{name: "owner", target: actorPath + "?owner=" + otherID.String(), wantOwner: &otherID},
		{name: "owner me", target: "/identities/me/permissions?owner=me", wantOwner: &apitest.Actor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api, svc := newTestClient(t, loggertest.Discard())
			svc.EXPECT().List(mock.Anything, apitest.Actor, apitest.Actor, tt.wantOwner).Return(permissions, nil)

			api.Get(tt.target).AssertResource(permissionsJSON)
		})
	}
}

// TestHandler_List_NothingAllowed checks that an actor that may do nothing gets an empty list.
func TestHandler_List_NothingAllowed(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().List(mock.Anything, apitest.Actor, apitest.Actor, (*uuid.UUID)(nil)).
		Return(service.Permissions{ID: apitest.Actor, AllowedActions: authz.AllowedActions{}}, nil)

	api.Get(actorPath).AssertResource(`{"id":"` + apitest.Actor.String() + `","allowedActions":[]}`)
}

func TestHandler_List_InvalidIdentityID(t *testing.T) {
	t.Parallel()
	// The mock fails the test on any call.
	api, _ := newTestClient(t, loggertest.Discard())

	api.Get("/identities/not-a-uuid/permissions").AssertProblem(identityapi.IdentityNotFoundProblem)
}

func TestHandler_List_InvalidOwner(t *testing.T) {
	t.Parallel()
	// The mock fails the test on any call.
	api, _ := newTestClient(t, loggertest.Discard())

	api.Get(actorPath + "?owner=not-a-uuid").
		AssertInvalidQuery(envelope.ValidationError{Parameter: "owner", Detail: `must be a UUID or "me"`})
}

func TestHandler_List_ServiceErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want envelope.Problem
	}{
		{
			name: "identity not found", err: fmt.Errorf("list: %w", service.ErrIdentityNotFound),
			want: identityapi.IdentityNotFoundProblem,
		},
		{name: "denied", err: fmt.Errorf("list: %w", authz.ErrDenied), want: forbiddenProblem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api, svc := newTestClient(t, loggertest.Discard())
			svc.EXPECT().List(mock.Anything, apitest.Actor, otherID, (*uuid.UUID)(nil)).
				Return(service.Permissions{}, tt.err)

			api.Get("/identities/" + otherID.String() + "/permissions").AssertProblem(tt.want)
		})
	}
}

// TestHandler_UnexpectedErrorIsLoggedButNotExposed checks that an unexpected error is logged and answered
// with a 500 without details.
func TestHandler_UnexpectedErrorIsLoggedButNotExposed(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	api, svc := newTestClient(t, logger)
	svc.EXPECT().List(mock.Anything, apitest.Actor, apitest.Actor, (*uuid.UUID)(nil)).
		Return(service.Permissions{}, errors.New("connection refused"))

	resp := api.Get(actorPath)

	resp.AssertInternalError()
	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "permission request failed", entries[0]["msg"])
	assert.Equal(t, "connection refused", entries[0]["error"])
	assert.NotContains(t, resp.Body.String(), "connection refused")
}

// TestHandler_ReadableMembers checks that every member of the response but id is an envelope.Field, and that
// it has no readable field besides allowedActions, which is always visible: the permissions are the caller's
// own.
func TestHandler_ReadableMembers(t *testing.T) {
	t.Parallel()

	apitest.AssertReadableMembers(t, reflect.TypeFor[PermissionsResponse](), nil)
}
