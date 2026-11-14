package tools

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
	"pkgsilo/core/mcpserver"
	"pkgsilo/core/pagination"
	"pkgsilo/feature/group/service"
	"pkgsilo/feature/group/service/mocks"
	identitytools "pkgsilo/feature/identity/tools"
	"pkgsilo/test/apitest"
	"pkgsilo/test/mcptest"
)

var (
	maintainers = service.Group{
		ID:        uuid.MustParse("3f4a5b6c-7d8e-4f9a-8b0c-1d2e3f4a5b6c"),
		Name:      authz.Visible("maintainers"),
		CreatedBy: authz.Visible(audit.SystemActor()),
		UpdatedBy: authz.Visible(apitest.Actor),
		CreatedAt: authz.Visible(time.Date(2026, time.October, 24, 10, 0, 0, 0, time.UTC)),
		UpdatedAt: authz.Visible(time.Date(2026, time.October, 24, 11, 30, 0, 123456000, time.UTC)),
	}
	maintainersJSON = `{
		"id": "3f4a5b6c-7d8e-4f9a-8b0c-1d2e3f4a5b6c",
		"name": "maintainers",
		"createdBy": "00000000-0000-0000-0000-000000000000",
		"updatedBy": "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b",
		"createdAt": "2026-10-24T10:00:00Z",
		"updatedAt": "2026-10-24T11:30:00.123456Z"
	}`
	// reviewers is a group that the actor may read only the name of.
	reviewers = service.Group{
		ID:   uuid.MustParse("9a8b7c6d-5e4f-4a3b-9c2d-1e0f9a8b7c6d"),
		Name: authz.Visible("reviewers"),
	}
	reviewersJSON = `{"id":"9a8b7c6d-5e4f-4a3b-9c2d-1e0f9a8b7c6d","name":"reviewers"}`

	ada = service.Member{
		IdentityID: uuid.MustParse("7c8d9e0f-1a2b-4c3d-9e4f-5a6b7c8d9e0f"),
		CreatedBy:  apitest.Actor,
		CreatedAt:  time.Date(2026, time.October, 25, 9, 0, 0, 0, time.UTC),
	}
	adaJSON = `{
		"identityId": "7c8d9e0f-1a2b-4c3d-9e4f-5a6b7c8d9e0f",
		"createdBy": "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b",
		"createdAt": "2026-10-25T09:00:00Z"
	}`
	grace = service.Member{
		IdentityID: uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"),
		CreatedBy:  audit.SystemActor(),
		CreatedAt:  time.Date(2026, time.October, 26, 9, 0, 0, 0, time.UTC),
	}
	graceJSON = `{
		"identityId": "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
		"createdBy": "00000000-0000-0000-0000-000000000000",
		"createdAt": "2026-10-26T09:00:00Z"
	}`

	memberArgs = `{"groupId":"` + maintainers.ID.String() + `","identityId":"` + ada.IdentityID.String() + `"}`
)

// newTestClient returns a client for the group tools, backed by a mock service.
func newTestClient(t *testing.T) (*mcptest.Client, *mocks.MockGroupService) {
	t.Helper()

	svc := mocks.NewMockGroupService(t)

	return mcptest.New(t, New(svc)), svc
}

// TestTools_ReadableMembers checks that every member of the output but the ID can be hidden and has an
// action for reading it.
func TestTools_ReadableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertReadableMembers(t, reflect.TypeFor[GroupOutput](), service.Actions().ReadFields().Fields())
}

// TestTools_SettableMembers checks that every member that group_create and group_update set has an action
// for setting it.
func TestTools_SettableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertSettableMembers(t, reflect.TypeFor[CreateInput](), service.Actions().CreateFields().Fields())
	mcptest.AssertSettableMembers(t, reflect.TypeFor[UpdateInput](), service.Actions().UpdateFields().Fields())
}

// TestTools_List checks that group_list pages through the groups, with a nextOffset while more follow, each
// masked like group_get.
func TestTools_List(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		args          string
		offset, limit int
		page          pagination.Page[service.Group]
		want          string
	}{
		{
			name: "defaults", args: "", offset: 0, limit: 50,
			page: pagination.Page[service.Group]{Items: []service.Group{maintainers, reviewers}},
			want: `{"items":[` + maintainersJSON + `,` + reviewersJSON + `]}`,
		},
		{
			name: "more follow", args: `{"limit":1}`, offset: 0, limit: 1,
			page: pagination.Page[service.Group]{Items: []service.Group{maintainers}, HasMore: true},
			want: `{"items":[` + maintainersJSON + `],"nextOffset":1}`,
		},
		{
			name: "last page", args: `{"offset":1,"limit":1}`, offset: 1, limit: 1,
			page: pagination.Page[service.Group]{Items: []service.Group{reviewers}},
			want: `{"items":[` + reviewersJSON + `]}`,
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

			mcp.Call("group_list", tt.args).AssertOutput(tt.want)
		})
	}
}

func TestTools_Get(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().Get(mock.Anything, apitest.Actor, reviewers.ID).Return(reviewers, nil)

	mcp.Call("group_get", `{"id":"`+reviewers.ID.String()+`"}`).AssertOutput(reviewersJSON)
}

func TestTools_Create(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		group service.Group
		want  string
	}{
		{name: "readable", group: maintainers, want: maintainersJSON},
		{
			name:  "unreadable",
			group: service.Group{ID: maintainers.ID},
			want:  `{"id":"3f4a5b6c-7d8e-4f9a-8b0c-1d2e3f4a5b6c"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().Create(mock.Anything, apitest.Actor, service.GroupInput{Name: "maintainers"}).
				Return(tt.group, nil)

			mcp.Call("group_create", `{"name":"maintainers"}`).AssertOutput(tt.want)
		})
	}
}

func TestTools_Update(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		args  string
		patch service.GroupPatch
	}{
		{
			name:  "name",
			args:  `{"id":"` + maintainers.ID.String() + `","name":"maintainers"}`,
			patch: service.GroupPatch{Name: new("maintainers")},
		},
		{
			name:  "nothing",
			args:  `{"id":"` + maintainers.ID.String() + `"}`,
			patch: service.GroupPatch{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().Patch(mock.Anything, apitest.Actor, maintainers.ID, tt.patch).Return(maintainers, nil)

			mcp.Call("group_update", tt.args).AssertOutput(maintainersJSON)
		})
	}
}

// TestTools_ListMembers checks that group_list_members pages through the members of the group, with a
// nextOffset while more follow.
func TestTools_ListMembers(t *testing.T) {
	t.Parallel()
	group := `"groupId":"` + maintainers.ID.String() + `"`
	tests := []struct {
		name          string
		args          string
		offset, limit int
		page          pagination.Page[service.Member]
		want          string
	}{
		{
			name: "defaults", args: `{` + group + `}`, offset: 0, limit: 50,
			page: pagination.Page[service.Member]{Items: []service.Member{ada, grace}},
			want: `{"items":[` + adaJSON + `,` + graceJSON + `]}`,
		},
		{
			name: "more follow", args: `{` + group + `,"limit":1}`, offset: 0, limit: 1,
			page: pagination.Page[service.Member]{Items: []service.Member{ada}, HasMore: true},
			want: `{"items":[` + adaJSON + `],"nextOffset":1}`,
		},
		{
			name: "last page", args: `{` + group + `,"offset":1,"limit":1}`, offset: 1, limit: 1,
			page: pagination.Page[service.Member]{Items: []service.Member{grace}},
			want: `{"items":[` + graceJSON + `]}`,
		},
		{
			name: "empty", args: `{` + group + `}`, offset: 0, limit: 50,
			want: `{"items":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().ListMembers(mock.Anything, apitest.Actor, maintainers.ID, tt.offset, tt.limit).
				Return(tt.page, nil)

			mcp.Call("group_list_members", tt.args).AssertOutput(tt.want)
		})
	}
}

func TestTools_AddMember(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().AddMember(mock.Anything, apitest.Actor, maintainers.ID, ada.IdentityID).Return(nil)

	mcp.Call("group_add_member", memberArgs).AssertOutput(`{}`)
}

func TestTools_RemoveMember(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().RemoveMember(mock.Anything, apitest.Actor, maintainers.ID, ada.IdentityID).Return(nil)

	mcp.Call("group_remove_member", memberArgs).AssertOutput(`{}`)
}

// TestTools_InvalidArguments checks that the tools validate their arguments: group_create and group_update
// with the rules of the service, group_update only those that it sets, and the list tools their page.
func TestTools_InvalidArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tool string
		args string
		want validation.Errors
	}{
		{
			name: "create",
			tool: "group_create",
			args: `{"name":"main\ntainers"}`,
			want: validation.Errors{"name": errors.New("must not contain control characters")},
		},
		{
			name: "create blank",
			tool: "group_create",
			args: `{"name":" "}`,
			want: validation.Errors{"name": errors.New("cannot be blank")},
		},
		{
			name: "update",
			tool: "group_update",
			args: `{"id":"` + maintainers.ID.String() + `","name":"` + strings.Repeat("m", 129) + `"}`,
			want: validation.Errors{"name": errors.New("the length must be between 1 and 128")},
		},
		{
			name: "list",
			tool: "group_list",
			args: `{"limit":101}`,
			want: validation.Errors{"limit": errors.New("must be no greater than 100")},
		},
		{
			name: "list members",
			tool: "group_list_members",
			args: `{"groupId":"` + maintainers.ID.String() + `","offset":-1,"limit":101}`,
			want: validation.Errors{
				"offset": errors.New("must be no less than 0"),
				"limit":  errors.New("must be no greater than 100"),
			},
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
			name: "list members",
			tool: "group_list_members",
			args: `{"limit":1}`,
			want: validation.Errors{"groupId": errors.New("is required")},
		},
		{
			name: "add member",
			tool: "group_add_member",
			args: `{"groupId":"` + maintainers.ID.String() + `"}`,
			want: validation.Errors{"identityId": errors.New("is required")},
		},
		{
			name: "malformed ID",
			tool: "group_add_member",
			args: `{"groupId":"` + maintainers.ID.String() + `","identityId":"ada"}`,
			want: validation.Errors{"identityId": errors.New("must be a UUID")},
		},
		// A member left out keeps its value, so null, as in the HTTP API's patch, can't clear it.
		{
			name: "null",
			tool: "group_update",
			args: `{"id":"` + maintainers.ID.String() + `","name":null}`,
			want: validation.Errors{"name": errors.New("cannot be null")},
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

// TestTools_DescribedLimits checks that the descriptions of the arguments name the limits of the service's
// rules.
func TestTools_DescribedLimits(t *testing.T) {
	t.Parallel()

	mcptest.AssertDescriptionContains(t, reflect.TypeFor[CreateInput](), "name",
		fmt.Sprintf("1 to %d characters", service.MaxNameLength))
}

// TestTools_Hints checks what the annotations of the tools tell clients: group_create only adds a group,
// while group_update overwrites its name and group_add_member can take away access, if a policy forbids
// something to the group's members.
func TestTools_Hints(t *testing.T) {
	t.Parallel()
	mcp, _ := newTestClient(t)

	assert.Equal(t, map[string]string{
		"group_list":          "read-only",
		"group_get":           "read-only",
		"group_list_members":  "read-only",
		"group_create":        "additive",
		"group_update":        "destructive, idempotent",
		"group_add_member":    "destructive, idempotent",
		"group_remove_member": "destructive, idempotent",
	}, mcp.Hints())
}

// TestTools_ServiceErrors checks that each tool answers the errors of the service with its tool errors, and
// leaves denials and unexpected errors to mcpserver.
func TestTools_ServiceErrors(t *testing.T) {
	t.Parallel()
	var (
		groupNotFound    = fmt.Errorf("get group: %w", service.ErrGroupNotFound)
		nameTaken        = fmt.Errorf("create group: %w", service.ErrGroupNameTaken)
		identityNotFound = fmt.Errorf("add group member: %w", service.ErrIdentityNotFound)
		memberNotFound   = fmt.Errorf("remove group member: %w", service.ErrMemberNotFound)
		denied           = fmt.Errorf("add group member: %w", authz.ErrDenied)
		fieldsDenied     = fmt.Errorf("create group: %w", &authz.FieldsDeniedError{Fields: []string{"name"}})
		unexpected       = errors.New("connection refused")
		idArgs           = `{"id":"` + maintainers.ID.String() + `"}`
		createArgs       = `{"name":"maintainers"}`
		updateArgs       = `{"id":"` + maintainers.ID.String() + `","name":"maintainers"}`
		listMembersArgs  = `{"groupId":"` + maintainers.ID.String() + `"}`

		wantGroupNotFound    = func(r *mcptest.Result) { r.AssertToolError(errGroupNotFound) }
		wantNameTaken        = func(r *mcptest.Result) { r.AssertToolError(errGroupNameTaken) }
		wantIdentityNotFound = func(r *mcptest.Result) { r.AssertToolError(identitytools.ErrIdentityNotFound) }
		wantMemberNotFound   = func(r *mcptest.Result) { r.AssertToolError(errMemberNotFound) }
		wantForbidden        = func(r *mcptest.Result) { r.AssertToolError(mcpserver.ErrForbidden) }
		wantFieldsDenied     = func(r *mcptest.Result) { r.AssertToolError(mcpserver.FieldsDeniedError("name")) }
		wantInternal         = func(r *mcptest.Result) { r.AssertInternalError() }

		expectList = func(svc *mocks.MockGroupService, err error) {
			svc.EXPECT().List(mock.Anything, apitest.Actor, 0, 50).Return(pagination.Page[service.Group]{}, err)
		}
		expectGet = func(svc *mocks.MockGroupService, err error) {
			svc.EXPECT().Get(mock.Anything, apitest.Actor, maintainers.ID).Return(service.Group{}, err)
		}
		expectCreate = func(svc *mocks.MockGroupService, err error) {
			svc.EXPECT().Create(mock.Anything, apitest.Actor, service.GroupInput{Name: "maintainers"}).
				Return(service.Group{}, err)
		}
		expectPatch = func(svc *mocks.MockGroupService, err error) {
			svc.EXPECT().Patch(mock.Anything, apitest.Actor, maintainers.ID, service.GroupPatch{Name: new("maintainers")}).
				Return(service.Group{}, err)
		}
		expectListMembers = func(svc *mocks.MockGroupService, err error) {
			svc.EXPECT().ListMembers(mock.Anything, apitest.Actor, maintainers.ID, 0, 50).
				Return(pagination.Page[service.Member]{}, err)
		}
		expectAdd = func(svc *mocks.MockGroupService, err error) {
			svc.EXPECT().AddMember(mock.Anything, apitest.Actor, maintainers.ID, ada.IdentityID).Return(err)
		}
		expectRemove = func(svc *mocks.MockGroupService, err error) {
			svc.EXPECT().RemoveMember(mock.Anything, apitest.Actor, maintainers.ID, ada.IdentityID).Return(err)
		}
	)
	tests := []struct {
		name   string
		tool   string
		args   string
		expect func(svc *mocks.MockGroupService, err error)
		err    error
		assert func(r *mcptest.Result)
	}{
		{name: "list unexpected", tool: "group_list", expect: expectList, err: unexpected, assert: wantInternal},
		{
			name: "get not found", tool: "group_get", args: idArgs, expect: expectGet, err: groupNotFound,
			assert: wantGroupNotFound,
		},
		{name: "get unexpected", tool: "group_get", args: idArgs, expect: expectGet, err: unexpected, assert: wantInternal},
		{
			name: "create taken", tool: "group_create", args: createArgs, expect: expectCreate, err: nameTaken,
			assert: wantNameTaken,
		},
		{
			name: "create fields denied", tool: "group_create", args: createArgs, expect: expectCreate,
			err: fieldsDenied, assert: wantFieldsDenied,
		},
		{
			name: "update not found", tool: "group_update", args: updateArgs, expect: expectPatch, err: groupNotFound,
			assert: wantGroupNotFound,
		},
		{
			name: "update taken", tool: "group_update", args: updateArgs, expect: expectPatch, err: nameTaken,
			assert: wantNameTaken,
		},
		{
			name: "update fields denied", tool: "group_update", args: updateArgs, expect: expectPatch,
			err: fieldsDenied, assert: wantFieldsDenied,
		},
		{
			name: "list members not found", tool: "group_list_members", args: listMembersArgs,
			expect: expectListMembers, err: groupNotFound, assert: wantGroupNotFound,
		},
		{
			name: "list members forbidden", tool: "group_list_members", args: listMembersArgs,
			expect: expectListMembers, err: denied, assert: wantForbidden,
		},
		{
			name: "list members unexpected", tool: "group_list_members", args: listMembersArgs,
			expect: expectListMembers, err: unexpected, assert: wantInternal,
		},
		{
			name: "add member group not found", tool: "group_add_member", args: memberArgs, expect: expectAdd,
			err: groupNotFound, assert: wantGroupNotFound,
		},
		{
			name: "add member identity not found", tool: "group_add_member", args: memberArgs, expect: expectAdd,
			err: identityNotFound, assert: wantIdentityNotFound,
		},
		{
			name: "add member forbidden", tool: "group_add_member", args: memberArgs, expect: expectAdd,
			err: denied, assert: wantForbidden,
		},
		{
			name: "add member unexpected", tool: "group_add_member", args: memberArgs, expect: expectAdd,
			err: unexpected, assert: wantInternal,
		},
		{
			name: "remove member group not found", tool: "group_remove_member", args: memberArgs,
			expect: expectRemove, err: groupNotFound, assert: wantGroupNotFound,
		},
		{
			name: "remove member not found", tool: "group_remove_member", args: memberArgs, expect: expectRemove,
			err: memberNotFound, assert: wantMemberNotFound,
		},
		{
			name: "remove member forbidden", tool: "group_remove_member", args: memberArgs, expect: expectRemove,
			err: denied, assert: wantForbidden,
		},
		{
			name: "remove member unexpected", tool: "group_remove_member", args: memberArgs, expect: expectRemove,
			err: unexpected, assert: wantInternal,
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
