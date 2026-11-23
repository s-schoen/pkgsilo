package tools

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"pkgsilo/core/authz"
	"pkgsilo/core/mcpserver"
	identitytools "pkgsilo/feature/identity/tools"
	"pkgsilo/feature/permission/service"
	"pkgsilo/feature/permission/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/mcptest"
)

var (
	// ownerID is the identity that owns the resources to list the actions on.
	ownerID     = uuid.MustParse("7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c")
	permissions = service.Permissions{
		ID: apitest.Actor, AllowedActions: authz.AllowedActions{"identity:read", "identity:create:username"},
	}
	permissionsJSON = `{"id":"` + apitest.Actor.String() +
		`","allowedActions":["identity:read","identity:create:username"]}`
)

// newTestClient returns a client for the permission tools, backed by a mock service.
func newTestClient(t *testing.T) (*mcptest.Client, *mocks.MockPermissionService) {
	t.Helper()

	svc := mocks.NewMockPermissionService(t)

	return mcptest.New(t, New(svc)), svc
}

// TestTools_ReadableMembers checks that every member of the output but id is an envelope.Field, and that it
// has no readable field besides allowedActions, which is always visible: the permissions are the actor's own.
func TestTools_ReadableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertReadableMembers(t, reflect.TypeFor[PermissionsOutput](), nil)
}

// TestTools_Hints checks what the annotations of the tools tell clients.
func TestTools_Hints(t *testing.T) {
	t.Parallel()
	mcp, _ := newTestClient(t)

	assert.Equal(t, map[string]string{"permission_list": "read-only"}, mcp.Hints())
}

// TestTools_List checks that the tool lists the actor's own permissions, for an owner if it's given.
func TestTools_List(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      string
		wantOwner *uuid.UUID
	}{
		{name: "without owner", args: `{}`},
		{name: "with owner", args: `{"owner":"` + ownerID.String() + `"}`, wantOwner: &ownerID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().List(mock.Anything, apitest.Actor, apitest.Actor, tt.wantOwner).Return(permissions, nil)

			mcp.Call("permission_list", tt.args).AssertOutput(permissionsJSON)
		})
	}
}

// TestTools_List_NothingAllowed checks that an actor that may do nothing gets an empty list.
func TestTools_List_NothingAllowed(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().List(mock.Anything, apitest.Actor, apitest.Actor, (*uuid.UUID)(nil)).
		Return(service.Permissions{ID: apitest.Actor, AllowedActions: authz.AllowedActions{}}, nil)

	mcp.Call("permission_list", `{}`).AssertOutput(`{"id":"` + apitest.Actor.String() + `","allowedActions":[]}`)
}

// TestTools_ArgumentsNotMatchingTheSchema checks that a call whose arguments don't match the input schema is
// answered with invalid_arguments before the tool is called.
func TestTools_ArgumentsNotMatchingTheSchema(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args string
		want validation.Errors
	}{
		{name: "malformed owner", args: `{"owner":"ada"}`, want: validation.Errors{"owner": errors.New("must be a UUID")}},
		{name: "me as owner", args: `{"owner":"me"}`, want: validation.Errors{"owner": errors.New("must be a UUID")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on any call.
			mcp, _ := newTestClient(t)

			mcp.Call("permission_list", tt.args).AssertToolError(mcpserver.InvalidArgumentsError(tt.want))
		})
	}
}

// TestTools_ServiceErrors checks that the tool answers the errors of the service with its tool errors, and
// leaves unexpected errors to mcpserver.
func TestTools_ServiceErrors(t *testing.T) {
	t.Parallel()

	t.Run("identity not found", func(t *testing.T) {
		t.Parallel()
		mcp, svc := newTestClient(t)
		svc.EXPECT().List(mock.Anything, apitest.Actor, apitest.Actor, &ownerID).
			Return(service.Permissions{}, fmt.Errorf("list permissions: %w", service.ErrIdentityNotFound))

		mcp.Call("permission_list", `{"owner":"`+ownerID.String()+`"}`).
			AssertToolError(identitytools.ErrIdentityNotFound)
	})

	t.Run("unexpected", func(t *testing.T) {
		t.Parallel()
		mcp, svc := newTestClient(t)
		svc.EXPECT().List(mock.Anything, apitest.Actor, apitest.Actor, (*uuid.UUID)(nil)).
			Return(service.Permissions{}, errors.New("connection refused"))

		mcp.Call("permission_list", `{}`).AssertInternalError()
	})
}
