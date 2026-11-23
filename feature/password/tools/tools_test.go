package tools

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"pkgsilo/core/authz"
	"pkgsilo/core/mcpserver"
	identitytools "pkgsilo/feature/identity/tools"
	"pkgsilo/feature/password/service"
	"pkgsilo/feature/password/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/mcptest"
)

var (
	adaID       = uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")
	adaPassword = service.Password{
		SetAt:          authz.Visible(time.Date(2026, time.November, 17, 10, 0, 0, 123456000, time.UTC)),
		AllowedActions: authz.AllowedActions{"password:delete", "password:reset"},
	}
	adaJSON  = `{"setAt":"2026-11-17T10:00:00.123456Z","allowedActions":["password:delete","password:reset"]}`
	adaArgs  = `{"identityId":"` + adaID.String() + `"}`
	allTools = []string{"password_delete", "password_get"}
)

// newTestClient returns a client for the password tools, backed by a mock service.
func newTestClient(t *testing.T) (*mcptest.Client, *mocks.MockPasswordService) {
	t.Helper()

	svc := mocks.NewMockPasswordService(t)

	return mcptest.New(t, New(svc)), svc
}

// TestTools_ReadableMembers checks that every member of the output can be hidden and has an action for
// reading it.
func TestTools_ReadableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertReadableMembers(t, reflect.TypeFor[PasswordOutput](), service.Actions().ReadFields().Fields())
}

// TestTools_AllowedActions checks that the output lists the allowed actions, with the actions on passwords
// as the enum in the output schema of every tool that returns a password.
func TestTools_AllowedActions(t *testing.T) {
	t.Parallel()
	mcptest.AssertAllowedActionsMember(t, reflect.TypeFor[PasswordOutput](), service.Actions().Allowable())

	mcp, _ := newTestClient(t)
	for _, name := range []string{"password_get"} {
		schema := mcp.OutputSchema(name)
		assert.Equal(t, []any{"password:delete", "password:change", "password:reset"},
			schema.Properties["allowedActions"].Items.Enum, name)
	}
}

// TestTools_NoPasswords checks that no tool sets a password, which would pass through the model's context,
// and that every tool says so.
func TestTools_NoPasswords(t *testing.T) {
	t.Parallel()
	mcp, _ := newTestClient(t)

	assert.Equal(t, allTools, mcp.ToolNames())
	for _, tool := range mcp.Tools() {
		assert.Contains(t, tool.Description, "a plaintext password must not pass through a model's context",
			tool.Name)
	}
}

func TestTools_Get(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		password service.Password
		want     string
	}{
		{name: "readable", password: adaPassword, want: adaJSON},
		{name: "hidden", password: service.Password{}, want: `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().Get(mock.Anything, apitest.Actor, adaID).Return(tt.password, nil)

			mcp.Call("password_get", adaArgs).AssertOutput(tt.want)
		})
	}
}

func TestTools_Delete(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().Delete(mock.Anything, apitest.Actor, adaID).Return(nil)

	mcp.Call("password_delete", adaArgs).AssertOutput(`{}`)
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
		{name: "missing", args: `{}`, want: validation.Errors{"identityId": errors.New("is required")}},
		{
			name: "malformed", args: `{"identityId":"ada"}`,
			want: validation.Errors{"identityId": errors.New("must be a UUID")},
		},
	}
	for _, tool := range allTools {
		for _, tt := range tests {
			t.Run(tool+" "+tt.name, func(t *testing.T) {
				t.Parallel()
				// The mock fails the test on any call.
				mcp, _ := newTestClient(t)

				mcp.Call(tool, tt.args).AssertToolError(mcpserver.InvalidArgumentsError(tt.want))
			})
		}
	}
}

// TestTools_Hints checks what the annotations of the tools tell clients.
func TestTools_Hints(t *testing.T) {
	t.Parallel()
	mcp, _ := newTestClient(t)

	assert.Equal(t, map[string]string{
		"password_get":    "read-only",
		"password_delete": "destructive, idempotent",
	}, mcp.Hints())
}

// TestTools_ServiceErrors checks that each tool answers the errors of the service with its tool errors, and
// leaves denials and unexpected errors to mcpserver.
func TestTools_ServiceErrors(t *testing.T) {
	t.Parallel()
	var (
		identityNotFound = fmt.Errorf("get password: %w", service.ErrIdentityNotFound)
		passwordNotFound = fmt.Errorf("get password: %w", service.ErrPasswordNotFound)
		denied           = fmt.Errorf("delete password: %w", authz.ErrDenied)
		unexpected       = errors.New("connection refused")

		wantIdentityNotFound = func(r *mcptest.Result) { r.AssertToolError(identitytools.ErrIdentityNotFound) }
		wantPasswordNotFound = func(r *mcptest.Result) { r.AssertToolError(errPasswordNotFound) }
		wantForbidden        = func(r *mcptest.Result) { r.AssertToolError(mcpserver.ErrForbidden) }
		wantInternal         = func(r *mcptest.Result) { r.AssertInternalError() }

		expectGet = func(svc *mocks.MockPasswordService, err error) {
			svc.EXPECT().Get(mock.Anything, apitest.Actor, adaID).Return(service.Password{}, err)
		}
		expectDelete = func(svc *mocks.MockPasswordService, err error) {
			svc.EXPECT().Delete(mock.Anything, apitest.Actor, adaID).Return(err)
		}
	)
	tests := []struct {
		name   string
		tool   string
		expect func(svc *mocks.MockPasswordService, err error)
		err    error
		assert func(r *mcptest.Result)
	}{
		{
			name: "get identity not found", tool: "password_get", expect: expectGet, err: identityNotFound,
			assert: wantIdentityNotFound,
		},
		{
			name: "get not found", tool: "password_get", expect: expectGet, err: passwordNotFound,
			assert: wantPasswordNotFound,
		},
		{name: "get unexpected", tool: "password_get", expect: expectGet, err: unexpected, assert: wantInternal},
		{
			name: "delete identity not found", tool: "password_delete", expect: expectDelete, err: identityNotFound,
			assert: wantIdentityNotFound,
		},
		{
			name: "delete not found", tool: "password_delete", expect: expectDelete, err: passwordNotFound,
			assert: wantPasswordNotFound,
		},
		{name: "delete forbidden", tool: "password_delete", expect: expectDelete, err: denied, assert: wantForbidden},
		{name: "delete unexpected", tool: "password_delete", expect: expectDelete, err: unexpected, assert: wantInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			tt.expect(svc, tt.err)

			tt.assert(mcp.Call(tt.tool, adaArgs))
		})
	}
}
