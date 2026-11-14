package tools

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/mock"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
	"pkgsilo/core/mcpserver"
	"pkgsilo/core/pagination"
	"pkgsilo/feature/identity/service"
	"pkgsilo/feature/identity/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/mcptest"
)

var (
	ada = service.Identity{
		ID:          apitest.Actor,
		Username:    authz.Visible("ada"),
		DisplayName: authz.Visible("Ada Lovelace"),
		CreatedBy:   authz.Visible(audit.SystemActor()),
		UpdatedBy:   authz.Visible(uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")),
		CreatedAt:   authz.Visible(time.Date(2026, time.October, 24, 10, 0, 0, 0, time.UTC)),
		UpdatedAt:   authz.Visible(time.Date(2026, time.October, 24, 11, 30, 0, 123456000, time.UTC)),
	}
	adaJSON = `{
		"id": "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b",
		"username": "ada",
		"displayName": "Ada Lovelace",
		"createdBy": "00000000-0000-0000-0000-000000000000",
		"updatedBy": "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
		"createdAt": "2026-10-24T10:00:00Z",
		"updatedAt": "2026-10-24T11:30:00.123456Z"
	}`
	// grace is an identity that the actor may read only the display name of.
	grace = service.Identity{
		ID:          uuid.MustParse("7c8d9e0f-1a2b-4c3d-9e4f-5a6b7c8d9e0f"),
		DisplayName: authz.Visible("Grace Hopper"),
	}
	graceJSON = `{"id":"7c8d9e0f-1a2b-4c3d-9e4f-5a6b7c8d9e0f","displayName":"Grace Hopper"}`
)

// newTestClient returns a client for the identity tools, backed by a mock service.
func newTestClient(t *testing.T) (*mcptest.Client, *mocks.MockIdentityService) {
	t.Helper()

	svc := mocks.NewMockIdentityService(t)

	return mcptest.New(t, New(svc)), svc
}

// TestTools_ReadableMembers checks that every member of the output but the ID can be hidden and has an
// action for reading it.
func TestTools_ReadableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertReadableMembers(t, reflect.TypeFor[IdentityOutput](), service.Actions().ReadFields().Fields())
}

// TestTools_SettableMembers checks that every member that identity_create and identity_update set has an
// action for setting it.
func TestTools_SettableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertSettableMembers(t, reflect.TypeFor[CreateInput](), service.Actions().CreateFields().Fields())
	mcptest.AssertSettableMembers(t, reflect.TypeFor[UpdateInput](), service.Actions().UpdateFields().Fields())
}

// TestTools_List checks that identity_list pages through the identities, with a nextOffset while more follow,
// each masked like identity_get.
func TestTools_List(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		args          string
		offset, limit int
		page          pagination.Page[service.Identity]
		want          string
	}{
		{
			name: "defaults", args: "", offset: 0, limit: 50,
			page: pagination.Page[service.Identity]{Items: []service.Identity{ada, grace}},
			want: `{"items":[` + adaJSON + `,` + graceJSON + `]}`,
		},
		{
			name: "more follow", args: `{"limit":1}`, offset: 0, limit: 1,
			page: pagination.Page[service.Identity]{Items: []service.Identity{ada}, HasMore: true},
			want: `{"items":[` + adaJSON + `],"nextOffset":1}`,
		},
		{
			name: "last page", args: `{"offset":1,"limit":1}`, offset: 1, limit: 1,
			page: pagination.Page[service.Identity]{Items: []service.Identity{grace}},
			want: `{"items":[` + graceJSON + `]}`,
		},
		{
			name: "empty", args: `{"offset":5}`, offset: 5, limit: 50,
			want: `{"items":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().List(mock.Anything, apitest.Actor, tt.offset, tt.limit).Return(tt.page, nil)

			mcp.Call("identity_list", tt.args).AssertOutput(tt.want)
		})
	}
}

func TestTools_List_InvalidArguments(t *testing.T) {
	t.Parallel()
	mcp, _ := newTestClient(t)

	mcp.Call("identity_list", `{"limit":101}`).AssertToolError(mcpserver.InvalidArgumentsError(
		validation.Errors{"limit": errors.New("must be no greater than 100")}))
}

func TestTools_Get(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().Get(mock.Anything, apitest.Actor, grace.ID).Return(grace, nil)

	mcp.Call("identity_get", `{"id":"`+grace.ID.String()+`"}`).AssertOutput(graceJSON)
}

func TestTools_Create(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		identity service.Identity
		want     string
	}{
		{name: "readable", identity: ada, want: adaJSON},
		{
			name:     "unreadable",
			identity: service.Identity{ID: ada.ID},
			want:     `{"id":"5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().Create(mock.Anything, apitest.Actor,
				service.IdentityInput{Username: "ada", DisplayName: "Ada Lovelace"}).Return(tt.identity, nil)

			mcp.Call("identity_create", `{"username":"ada","displayName":"Ada Lovelace"}`).AssertOutput(tt.want)
		})
	}
}

func TestTools_Update(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		args  string
		patch service.IdentityPatch
	}{
		{
			name:  "both",
			args:  `{"id":"` + ada.ID.String() + `","username":"ada","displayName":"Ada Lovelace"}`,
			patch: service.IdentityPatch{Username: new("ada"), DisplayName: new("Ada Lovelace")},
		},
		{
			name:  "display name only",
			args:  `{"id":"` + ada.ID.String() + `","displayName":"Ada Lovelace"}`,
			patch: service.IdentityPatch{DisplayName: new("Ada Lovelace")},
		},
		{
			name:  "nothing",
			args:  `{"id":"` + ada.ID.String() + `"}`,
			patch: service.IdentityPatch{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().Patch(mock.Anything, apitest.Actor, ada.ID, tt.patch).Return(ada, nil)

			mcp.Call("identity_update", tt.args).AssertOutput(adaJSON)
		})
	}
}

// TestTools_InvalidArguments checks that identity_create and identity_update validate their arguments with
// the rules of the service, and that identity_update validates only those that it sets.
func TestTools_InvalidArguments(t *testing.T) {
	t.Parallel()
	usernameErr := errors.New(
		"must consist of lowercase letters and digits, optionally separated by single '.', '_' or '-'")
	tests := []struct {
		name string
		tool string
		args string
		want validation.Errors
	}{
		{
			name: "create",
			tool: "identity_create",
			args: `{"username":"Ada","displayName":"Ada\nLovelace"}`,
			want: validation.Errors{
				"username":    usernameErr,
				"displayName": errors.New("must not contain control characters"),
			},
		},
		{
			name: "create blank",
			tool: "identity_create",
			args: `{"username":"","displayName":" "}`,
			want: validation.Errors{
				"username":    errors.New("cannot be blank"),
				"displayName": errors.New("cannot be blank"),
			},
		},
		{
			name: "update",
			tool: "identity_update",
			args: `{"id":"` + ada.ID.String() + `","username":"ab","displayName":""}`,
			want: validation.Errors{
				"username":    errors.New("the length must be between 3 and 64"),
				"displayName": errors.New("cannot be blank"),
			},
		},
		{
			name: "update one",
			tool: "identity_update",
			args: `{"id":"` + ada.ID.String() + `","username":"ada!"}`,
			want: validation.Errors{"username": usernameErr},
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

// TestTools_MissingArguments checks that the SDK answers a call without a required argument before the tool
// is called.
func TestTools_MissingArguments(t *testing.T) {
	t.Parallel()
	mcp, _ := newTestClient(t)

	mcp.Call("identity_create", `{"username":"ada"}`).
		AssertErrorText(`validating "arguments": validating root: required: missing properties: ["displayName"]`)
}

func TestTools_Whoami(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().Get(mock.Anything, apitest.Actor, apitest.Actor).Return(ada, nil)

	mcp.Call("identity_whoami", "").AssertOutput(adaJSON)
}

// TestTools_Whoami_HiddenFields checks that a field that the actor may not read is absent from the output.
func TestTools_Whoami_HiddenFields(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().Get(mock.Anything, apitest.Actor, apitest.Actor).
		Return(service.Identity{ID: ada.ID, DisplayName: ada.DisplayName}, nil)

	mcp.Call("identity_whoami", "").
		AssertOutput(`{"id":"5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b","displayName":"Ada Lovelace"}`)
}

// TestTools_ServiceErrors checks that each tool answers the errors of the service with its tool errors, and
// leaves denials and unexpected errors to mcpserver.
func TestTools_ServiceErrors(t *testing.T) {
	t.Parallel()
	var (
		notFound     = fmt.Errorf("get identity: %w", service.ErrIdentityNotFound)
		taken        = fmt.Errorf("create identity: %w", service.ErrUsernameTaken)
		fieldsDenied = fmt.Errorf("create identity: %w", &authz.FieldsDeniedError{Fields: []string{"username"}})
		unexpected   = errors.New("connection refused")
		idArgs       = `{"id":"` + ada.ID.String() + `"}`
		createArgs   = `{"username":"ada","displayName":"Ada Lovelace"}`
		updateArgs   = `{"id":"` + ada.ID.String() + `","username":"ada"}`
		input        = service.IdentityInput{Username: "ada", DisplayName: "Ada Lovelace"}
		patch        = service.IdentityPatch{Username: new("ada")}
		wantNotFound = func(r *mcptest.Result) { r.AssertToolError(errIdentityNotFound) }
		wantTaken    = func(r *mcptest.Result) { r.AssertToolError(errUsernameTaken) }
		wantDenied   = func(r *mcptest.Result) { r.AssertToolError(mcpserver.FieldsDeniedError("username")) }
		wantInternal = func(r *mcptest.Result) { r.AssertInternalError() }
		expectList   = func(svc *mocks.MockIdentityService, err error) {
			svc.EXPECT().List(mock.Anything, apitest.Actor, 0, 50).Return(pagination.Page[service.Identity]{}, err)
		}
		expectGet = func(svc *mocks.MockIdentityService, err error) {
			svc.EXPECT().Get(mock.Anything, apitest.Actor, ada.ID).Return(service.Identity{}, err)
		}
		expectCreate = func(svc *mocks.MockIdentityService, err error) {
			svc.EXPECT().Create(mock.Anything, apitest.Actor, input).Return(service.Identity{}, err)
		}
		expectPatch = func(svc *mocks.MockIdentityService, err error) {
			svc.EXPECT().Patch(mock.Anything, apitest.Actor, ada.ID, patch).Return(service.Identity{}, err)
		}
	)
	tests := []struct {
		name   string
		tool   string
		args   string
		expect func(svc *mocks.MockIdentityService, err error)
		err    error
		assert func(r *mcptest.Result)
	}{
		{name: "list unexpected", tool: "identity_list", expect: expectList, err: unexpected, assert: wantInternal},
		{name: "get not found", tool: "identity_get", args: idArgs, expect: expectGet, err: notFound, assert: wantNotFound},
		{
			name: "get unexpected", tool: "identity_get", args: idArgs, expect: expectGet, err: unexpected,
			assert: wantInternal,
		},
		{
			name: "create taken", tool: "identity_create", args: createArgs, expect: expectCreate, err: taken,
			assert: wantTaken,
		},
		{
			name: "create fields denied", tool: "identity_create", args: createArgs, expect: expectCreate,
			err: fieldsDenied, assert: wantDenied,
		},
		{
			name: "update not found", tool: "identity_update", args: updateArgs, expect: expectPatch, err: notFound,
			assert: wantNotFound,
		},
		{name: "update taken", tool: "identity_update", args: updateArgs, expect: expectPatch, err: taken, assert: wantTaken},
		{
			name: "update fields denied", tool: "identity_update", args: updateArgs, expect: expectPatch,
			err: fieldsDenied, assert: wantDenied,
		},
		{
			name: "whoami not found", tool: "identity_whoami",
			expect: func(svc *mocks.MockIdentityService, err error) {
				svc.EXPECT().Get(mock.Anything, apitest.Actor, apitest.Actor).Return(service.Identity{}, err)
			},
			err: notFound, assert: wantNotFound,
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
