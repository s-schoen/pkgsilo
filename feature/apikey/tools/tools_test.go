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
	"pkgsilo/core/pagination"
	"pkgsilo/feature/apikey/service"
	"pkgsilo/feature/apikey/service/mocks"
	identitytools "pkgsilo/feature/identity/tools"
	"pkgsilo/test/apitest"
	"pkgsilo/test/mcptest"
)

var (
	adaID = uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")
	// ci is an API key that expires and has not been used yet, with every field visible.
	ci = service.APIKey{
		ID:         uuid.MustParse("3f4a5b6c-7d8e-4f9a-8b0c-1d2e3f4a5b6c"),
		IdentityID: authz.Visible(adaID),
		Name:       authz.Visible("ci"),
		KeySuffix:  authz.Visible("AB2C"),
		ExpiresAt:  authz.Visible(new(time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC))),
		LastUsedAt: authz.Visible[*time.Time](nil),
		CreatedBy:  authz.Visible(apitest.Actor),
		CreatedAt:  authz.Visible(time.Date(2026, time.October, 24, 10, 0, 0, 123456000, time.UTC)),
	}
	ciJSON = `{
		"id": "3f4a5b6c-7d8e-4f9a-8b0c-1d2e3f4a5b6c",
		"identityId": "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
		"name": "ci",
		"keySuffix": "AB2C",
		"expiresAt": "2027-01-01T00:00:00Z",
		"lastUsedAt": null,
		"createdBy": "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b",
		"createdAt": "2026-10-24T10:00:00.123456Z"
	}`
	// bootstrap is an API key that never expires and that the actor may read only the name and expiry of: its
	// expiry is a visible null, its last use is hidden.
	bootstrap = service.APIKey{
		ID:        uuid.MustParse("9a8b7c6d-5e4f-4a3b-9c2d-1e0f9a8b7c6d"),
		Name:      authz.Visible("bootstrap"),
		ExpiresAt: authz.Visible[*time.Time](nil),
	}
	bootstrapJSON = `{"id":"9a8b7c6d-5e4f-4a3b-9c2d-1e0f9a8b7c6d","name":"bootstrap","expiresAt":null}`

	identityArg = `"identityId":"` + adaID.String() + `"`
	keyArgs     = `{` + identityArg + `,"id":"` + ci.ID.String() + `"}`
)

// newTestClient returns a client for the API key tools, backed by a mock service.
func newTestClient(t *testing.T) (*mcptest.Client, *mocks.MockAPIKeyService) {
	t.Helper()

	svc := mocks.NewMockAPIKeyService(t)

	return mcptest.New(t, New(svc)), svc
}

// TestTools_ReadableMembers checks that every member of the output but the ID can be hidden and has an
// action for reading it.
func TestTools_ReadableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertReadableMembers(t, reflect.TypeFor[APIKeyOutput](), service.Actions().ReadFields().Fields())
}

// TestTools_NoCreate checks that no tool creates an API key, whose key would pass through the model's
// context.
func TestTools_NoCreate(t *testing.T) {
	t.Parallel()
	mcp, _ := newTestClient(t)

	assert.Equal(t, []string{"apikey_delete", "apikey_get", "apikey_list"}, mcp.ToolNames())
}

// TestTools_List checks that apikey_list pages through the API keys of the identity, with a nextOffset while
// more follow, each masked like apikey_get.
func TestTools_List(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		args          string
		offset, limit int
		page          pagination.Page[service.APIKey]
		want          string
	}{
		{
			name: "defaults", args: `{` + identityArg + `}`, offset: 0, limit: 50,
			page: pagination.Page[service.APIKey]{Items: []service.APIKey{ci, bootstrap}},
			want: `{"items":[` + ciJSON + `,` + bootstrapJSON + `]}`,
		},
		{
			name: "more follow", args: `{` + identityArg + `,"limit":1}`, offset: 0, limit: 1,
			page: pagination.Page[service.APIKey]{Items: []service.APIKey{ci}, HasMore: true},
			want: `{"items":[` + ciJSON + `],"nextOffset":1}`,
		},
		{
			name: "last page", args: `{` + identityArg + `,"offset":1,"limit":1}`, offset: 1, limit: 1,
			page: pagination.Page[service.APIKey]{Items: []service.APIKey{bootstrap}},
			want: `{"items":[` + bootstrapJSON + `]}`,
		},
		{
			name: "empty", args: `{` + identityArg + `,"offset":5}`, offset: 5, limit: 50,
			want: `{"items":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().List(mock.Anything, apitest.Actor, adaID, tt.offset, tt.limit).Return(tt.page, nil)

			mcp.Call("apikey_list", tt.args).AssertOutput(tt.want)
		})
	}
}

// TestTools_Get checks that apikey_get returns the API key masked: a visible null, such as an API key's
// that never expires or was never used, stays null, and a hidden field is absent.
func TestTools_Get(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		key  service.APIKey
		want string
	}{
		{name: "readable", key: ci, want: ciJSON},
		{name: "partly hidden", key: bootstrap, want: bootstrapJSON},
		{
			name: "expiry hidden",
			key:  service.APIKey{ID: ci.ID, Name: ci.Name, LastUsedAt: ci.LastUsedAt},
			want: `{"id":"3f4a5b6c-7d8e-4f9a-8b0c-1d2e3f4a5b6c","name":"ci","lastUsedAt":null}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().Get(mock.Anything, apitest.Actor, adaID, ci.ID).Return(tt.key, nil)

			mcp.Call("apikey_get", keyArgs).AssertOutput(tt.want)
		})
	}
}

func TestTools_Delete(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().Delete(mock.Anything, apitest.Actor, adaID, ci.ID).Return(nil)

	mcp.Call("apikey_delete", keyArgs).AssertOutput(`{}`)
}

// TestTools_InvalidArguments checks that apikey_list validates its page.
func TestTools_InvalidArguments(t *testing.T) {
	t.Parallel()
	mcp, _ := newTestClient(t)

	mcp.Call("apikey_list", `{`+identityArg+`,"offset":-1,"limit":101}`).
		AssertToolError(mcpserver.InvalidArgumentsError(validation.Errors{
			"offset": errors.New("must be no less than 0"),
			"limit":  errors.New("must be no greater than 100"),
		}))
}

// TestTools_ArgumentsNotMatchingTheSchema checks that a call whose arguments don't match the input schema,
// such as one without a required argument, is answered with invalid_arguments before the tool is called.
func TestTools_ArgumentsNotMatchingTheSchema(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tool string
		args string
		want validation.Errors
	}{
		{
			name: "list",
			tool: "apikey_list",
			args: `{"limit":1}`,
			want: validation.Errors{"identityId": errors.New("is required")},
		},
		{
			name: "get",
			tool: "apikey_get",
			args: `{"id":"` + ci.ID.String() + `"}`,
			want: validation.Errors{"identityId": errors.New("is required")},
		},
		{
			name: "delete",
			tool: "apikey_delete",
			args: `{` + identityArg + `}`,
			want: validation.Errors{"id": errors.New("is required")},
		},
		{
			name: "malformed ID",
			tool: "apikey_delete",
			args: `{` + identityArg + `,"id":"ci"}`,
			want: validation.Errors{"id": errors.New("must be a UUID")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, _ := newTestClient(t)

			mcp.Call(tt.tool, tt.args).AssertToolError(mcpserver.InvalidArgumentsError(tt.want))
		})
	}
}

// TestTools_Hints checks what the annotations of the tools tell clients.
func TestTools_Hints(t *testing.T) {
	t.Parallel()
	mcp, _ := newTestClient(t)

	assert.Equal(t, map[string]string{
		"apikey_list":   "read-only",
		"apikey_get":    "read-only",
		"apikey_delete": "destructive, idempotent",
	}, mcp.Hints())
}

// TestTools_ServiceErrors checks that each tool answers the errors of the service with its tool errors, and
// leaves denials and unexpected errors to mcpserver.
func TestTools_ServiceErrors(t *testing.T) {
	t.Parallel()
	var (
		identityNotFound = fmt.Errorf("get api key: %w", service.ErrIdentityNotFound)
		keyNotFound      = fmt.Errorf("get api key: %w", service.ErrAPIKeyNotFound)
		denied           = fmt.Errorf("delete api key: %w", authz.ErrDenied)
		unexpected       = errors.New("connection refused")
		listArgs         = `{` + identityArg + `}`

		wantIdentityNotFound = func(r *mcptest.Result) { r.AssertToolError(identitytools.ErrIdentityNotFound) }
		wantKeyNotFound      = func(r *mcptest.Result) { r.AssertToolError(errAPIKeyNotFound) }
		wantForbidden        = func(r *mcptest.Result) { r.AssertToolError(mcpserver.ErrForbidden) }
		wantInternal         = func(r *mcptest.Result) { r.AssertInternalError() }

		expectList = func(svc *mocks.MockAPIKeyService, err error) {
			svc.EXPECT().List(mock.Anything, apitest.Actor, adaID, 0, 50).Return(pagination.Page[service.APIKey]{}, err)
		}
		expectGet = func(svc *mocks.MockAPIKeyService, err error) {
			svc.EXPECT().Get(mock.Anything, apitest.Actor, adaID, ci.ID).Return(service.APIKey{}, err)
		}
		expectDelete = func(svc *mocks.MockAPIKeyService, err error) {
			svc.EXPECT().Delete(mock.Anything, apitest.Actor, adaID, ci.ID).Return(err)
		}
	)
	tests := []struct {
		name   string
		tool   string
		args   string
		expect func(svc *mocks.MockAPIKeyService, err error)
		err    error
		assert func(r *mcptest.Result)
	}{
		{
			name: "list identity not found", tool: "apikey_list", args: listArgs, expect: expectList,
			err: identityNotFound, assert: wantIdentityNotFound,
		},
		{
			name: "list unexpected", tool: "apikey_list", args: listArgs, expect: expectList, err: unexpected,
			assert: wantInternal,
		},
		{
			name: "get identity not found", tool: "apikey_get", args: keyArgs, expect: expectGet,
			err: identityNotFound, assert: wantIdentityNotFound,
		},
		{
			name: "get not found", tool: "apikey_get", args: keyArgs, expect: expectGet, err: keyNotFound,
			assert: wantKeyNotFound,
		},
		{
			name: "get unexpected", tool: "apikey_get", args: keyArgs, expect: expectGet, err: unexpected,
			assert: wantInternal,
		},
		{
			name: "delete identity not found", tool: "apikey_delete", args: keyArgs, expect: expectDelete,
			err: identityNotFound, assert: wantIdentityNotFound,
		},
		{
			name: "delete not found", tool: "apikey_delete", args: keyArgs, expect: expectDelete, err: keyNotFound,
			assert: wantKeyNotFound,
		},
		{
			name: "delete forbidden", tool: "apikey_delete", args: keyArgs, expect: expectDelete, err: denied,
			assert: wantForbidden,
		},
		{
			name: "delete unexpected", tool: "apikey_delete", args: keyArgs, expect: expectDelete, err: unexpected,
			assert: wantInternal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			tt.expect(svc, tt.err)

			tt.assert(mcp.Call(tt.tool, tt.args))
		})
	}
}
