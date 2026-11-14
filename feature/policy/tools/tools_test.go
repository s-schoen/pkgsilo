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
	"github.com/stretchr/testify/mock"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
	"pkgsilo/core/mcpserver"
	"pkgsilo/core/pagination"
	"pkgsilo/feature/policy/service"
	"pkgsilo/feature/policy/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/mcptest"
)

const readIdentitiesSource = `permit (principal, action == Action::"identity:read", resource);`

var (
	readIdentities = service.Policy{
		ID:        uuid.MustParse("3f4a5b6c-7d8e-4f9a-8b0c-1d2e3f4a5b6c"),
		Name:      authz.Visible("read-identities"),
		Source:    authz.Visible(readIdentitiesSource),
		CreatedBy: authz.Visible(audit.SystemActor()),
		UpdatedBy: authz.Visible(apitest.Actor),
		CreatedAt: authz.Visible(time.Date(2026, time.October, 24, 10, 0, 0, 0, time.UTC)),
		UpdatedAt: authz.Visible(time.Date(2026, time.October, 24, 11, 30, 0, 123456000, time.UTC)),
	}
	readIdentitiesJSON = `{
		"id": "3f4a5b6c-7d8e-4f9a-8b0c-1d2e3f4a5b6c",
		"name": "read-identities",
		"source": "permit (principal, action == Action::\"identity:read\", resource);",
		"createdBy": "00000000-0000-0000-0000-000000000000",
		"updatedBy": "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b",
		"createdAt": "2026-10-24T10:00:00Z",
		"updatedAt": "2026-10-24T11:30:00.123456Z"
	}`
	// administrators is a policy that the actor may read only the name of.
	administrators = service.Policy{
		ID:   uuid.MustParse("9a8b7c6d-5e4f-4a3b-9c2d-1e0f9a8b7c6d"),
		Name: authz.Visible("administrators"),
	}
	administratorsJSON = `{"id":"9a8b7c6d-5e4f-4a3b-9c2d-1e0f9a8b7c6d","name":"administrators"}`

	createArgs = `{"name":"read-identities","source":"permit (principal, action == Action::\"identity:read\", resource);"}`
	idArgs     = `{"id":"` + readIdentities.ID.String() + `"}`
)

// newTestClient returns a client for the policy tools, backed by a mock service.
func newTestClient(t *testing.T) (*mcptest.Client, *mocks.MockPolicyService) {
	t.Helper()

	svc := mocks.NewMockPolicyService(t)

	return mcptest.New(t, New(svc)), svc
}

// TestTools_ReadableMembers checks that every member of the output but the ID can be hidden and has an
// action for reading it.
func TestTools_ReadableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertReadableMembers(t, reflect.TypeFor[PolicyOutput](), service.Actions().ReadFields().Fields())
}

// TestTools_SettableMembers checks that every member that policy_create and policy_update set has an action
// for setting it.
func TestTools_SettableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertSettableMembers(t, reflect.TypeFor[CreateInput](), service.Actions().CreateFields().Fields())
	mcptest.AssertSettableMembers(t, reflect.TypeFor[UpdateInput](), service.Actions().UpdateFields().Fields())
}

// TestTools_GetSchema checks that policy_get_schema returns the schema of the service as it is.
func TestTools_GetSchema(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().Schema().Return("entity Identity;\naction \"identity:read\";\n")

	mcp.Call("policy_get_schema", "").AssertOutput(`{"schema":"entity Identity;\naction \"identity:read\";\n"}`)
}

// TestTools_List checks that policy_list pages through the policies, with a nextOffset while more follow,
// each masked like policy_get.
func TestTools_List(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		args          string
		offset, limit int
		page          pagination.Page[service.Policy]
		want          string
	}{
		{
			name: "defaults", args: "", offset: 0, limit: 50,
			page: pagination.Page[service.Policy]{Items: []service.Policy{readIdentities, administrators}},
			want: `{"items":[` + readIdentitiesJSON + `,` + administratorsJSON + `]}`,
		},
		{
			name: "more follow", args: `{"limit":1}`, offset: 0, limit: 1,
			page: pagination.Page[service.Policy]{Items: []service.Policy{readIdentities}, HasMore: true},
			want: `{"items":[` + readIdentitiesJSON + `],"nextOffset":1}`,
		},
		{
			name: "last page", args: `{"offset":1,"limit":1}`, offset: 1, limit: 1,
			page: pagination.Page[service.Policy]{Items: []service.Policy{administrators}},
			want: `{"items":[` + administratorsJSON + `]}`,
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

			mcp.Call("policy_list", tt.args).AssertOutput(tt.want)
		})
	}
}

func TestTools_Get(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().Get(mock.Anything, apitest.Actor, administrators.ID).Return(administrators, nil)

	mcp.Call("policy_get", `{"id":"`+administrators.ID.String()+`"}`).AssertOutput(administratorsJSON)
}

func TestTools_Create(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		policy service.Policy
		want   string
	}{
		{name: "readable", policy: readIdentities, want: readIdentitiesJSON},
		{
			name:   "unreadable",
			policy: service.Policy{ID: readIdentities.ID},
			want:   `{"id":"3f4a5b6c-7d8e-4f9a-8b0c-1d2e3f4a5b6c"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().Create(mock.Anything, apitest.Actor,
				service.PolicyInput{Name: "read-identities", Source: readIdentitiesSource}).Return(tt.policy, nil)

			mcp.Call("policy_create", createArgs).AssertOutput(tt.want)
		})
	}
}

func TestTools_Update(t *testing.T) {
	t.Parallel()
	id := `"id":"` + readIdentities.ID.String() + `"`
	source := `"source":"permit (principal, action == Action::\"identity:read\", resource);"`
	tests := []struct {
		name  string
		args  string
		patch service.PolicyPatch
	}{
		{
			name:  "name and source",
			args:  `{` + id + `,"name":"read-identities",` + source + `}`,
			patch: service.PolicyPatch{Name: new("read-identities"), Source: new(readIdentitiesSource)},
		},
		{
			name:  "name",
			args:  `{` + id + `,"name":"read-identities"}`,
			patch: service.PolicyPatch{Name: new("read-identities")},
		},
		{
			name:  "source",
			args:  `{` + id + `,` + source + `}`,
			patch: service.PolicyPatch{Source: new(readIdentitiesSource)},
		},
		{
			name:  "nothing",
			args:  `{` + id + `}`,
			patch: service.PolicyPatch{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().Patch(mock.Anything, apitest.Actor, readIdentities.ID, tt.patch).Return(readIdentities, nil)

			mcp.Call("policy_update", tt.args).AssertOutput(readIdentitiesJSON)
		})
	}
}

func TestTools_Delete(t *testing.T) {
	t.Parallel()
	mcp, svc := newTestClient(t)
	svc.EXPECT().Delete(mock.Anything, apitest.Actor, readIdentities.ID).Return(nil)

	mcp.Call("policy_delete", idArgs).AssertOutput(`{}`)
}

// TestTools_InvalidArguments checks that the tools validate their arguments: policy_create and
// policy_update with the rules of the service, policy_update only those that it sets, and policy_list its
// page.
func TestTools_InvalidArguments(t *testing.T) {
	t.Parallel()
	id := `"id":"` + readIdentities.ID.String() + `"`
	tests := []struct {
		name string
		tool string
		args string
		want validation.Errors
	}{
		{
			name: "create",
			tool: "policy_create",
			args: `{"name":"read\nidentities","source":""}`,
			want: validation.Errors{
				"name":   errors.New("must not contain control characters"),
				"source": errors.New("cannot be blank"),
			},
		},
		{
			name: "create blank name",
			tool: "policy_create",
			args: `{"name":" ","source":"permit (principal, action, resource);"}`,
			want: validation.Errors{"name": errors.New("cannot be blank")},
		},
		{
			name: "update name",
			tool: "policy_update",
			args: `{` + id + `,"name":"` + strings.Repeat("p", 129) + `"}`,
			want: validation.Errors{"name": errors.New("the length must be between 1 and 128")},
		},
		{
			name: "update source",
			tool: "policy_update",
			args: `{` + id + `,"source":""}`,
			want: validation.Errors{"source": errors.New("cannot be blank")},
		},
		{
			name: "list",
			tool: "policy_list",
			args: `{"offset":-1,"limit":101}`,
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

// TestTools_MissingArguments checks that the SDK answers a call without a required argument before the tool
// is called.
func TestTools_MissingArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tool string
		args string
		want string
	}{
		{
			name: "create",
			tool: "policy_create",
			args: `{"name":"read-identities"}`,
			want: `validating "arguments": validating root: required: missing properties: ["source"]`,
		},
		{
			name: "delete",
			tool: "policy_delete",
			args: `{}`,
			want: `validating "arguments": validating root: required: missing properties: ["id"]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, _ := newTestClient(t)

			mcp.Call(tt.tool, tt.args).AssertErrorText(tt.want)
		})
	}
}

// TestTools_ServiceErrors checks that each tool answers the errors of the service with its tool errors, and
// leaves denials and unexpected errors to mcpserver.
func TestTools_ServiceErrors(t *testing.T) {
	t.Parallel()
	var (
		policyNotFound = fmt.Errorf("get policy: %w", service.ErrPolicyNotFound)
		nameTaken      = fmt.Errorf("create policy: %w", service.ErrPolicyNameTaken)
		invalid        = fmt.Errorf("create policy: %w", &service.InvalidPolicyError{
			Reason: "the source holds no statements",
		})
		denied       = fmt.Errorf("delete policy: %w", authz.ErrDenied)
		fieldsDenied = fmt.Errorf("create policy: %w", &authz.FieldsDeniedError{Fields: []string{"source"}})
		unexpected   = errors.New("connection refused")
		updateArgs   = `{"id":"` + readIdentities.ID.String() + `","name":"read-identities"}`

		wantPolicyNotFound = func(r *mcptest.Result) { r.AssertToolError(errPolicyNotFound) }
		wantNameTaken      = func(r *mcptest.Result) { r.AssertToolError(errPolicyNameTaken) }
		wantInvalid        = func(r *mcptest.Result) {
			r.AssertErrorText("invalid_policy: The policy is invalid: the source holds no statements. Nothing was " +
				"changed. Fix the source, with the schema that policy_get_schema returns, and call again.")
		}
		wantForbidden    = func(r *mcptest.Result) { r.AssertToolError(mcpserver.ErrForbidden) }
		wantFieldsDenied = func(r *mcptest.Result) { r.AssertToolError(mcpserver.FieldsDeniedError("source")) }
		wantInternal     = func(r *mcptest.Result) { r.AssertInternalError() }

		expectList = func(svc *mocks.MockPolicyService, err error) {
			svc.EXPECT().List(mock.Anything, apitest.Actor, 0, 50).Return(pagination.Page[service.Policy]{}, err)
		}
		expectGet = func(svc *mocks.MockPolicyService, err error) {
			svc.EXPECT().Get(mock.Anything, apitest.Actor, readIdentities.ID).Return(service.Policy{}, err)
		}
		expectCreate = func(svc *mocks.MockPolicyService, err error) {
			svc.EXPECT().Create(mock.Anything, apitest.Actor,
				service.PolicyInput{Name: "read-identities", Source: readIdentitiesSource}).Return(service.Policy{}, err)
		}
		expectPatch = func(svc *mocks.MockPolicyService, err error) {
			svc.EXPECT().Patch(mock.Anything, apitest.Actor, readIdentities.ID,
				service.PolicyPatch{Name: new("read-identities")}).Return(service.Policy{}, err)
		}
		expectDelete = func(svc *mocks.MockPolicyService, err error) {
			svc.EXPECT().Delete(mock.Anything, apitest.Actor, readIdentities.ID).Return(err)
		}
	)
	tests := []struct {
		name   string
		tool   string
		args   string
		expect func(svc *mocks.MockPolicyService, err error)
		err    error
		assert func(r *mcptest.Result)
	}{
		{name: "list unexpected", tool: "policy_list", expect: expectList, err: unexpected, assert: wantInternal},
		{
			name: "get not found", tool: "policy_get", args: idArgs, expect: expectGet, err: policyNotFound,
			assert: wantPolicyNotFound,
		},
		{name: "get unexpected", tool: "policy_get", args: idArgs, expect: expectGet, err: unexpected, assert: wantInternal},
		{
			name: "create taken", tool: "policy_create", args: createArgs, expect: expectCreate, err: nameTaken,
			assert: wantNameTaken,
		},
		{
			name: "create invalid", tool: "policy_create", args: createArgs, expect: expectCreate, err: invalid,
			assert: wantInvalid,
		},
		{
			name: "create fields denied", tool: "policy_create", args: createArgs, expect: expectCreate,
			err: fieldsDenied, assert: wantFieldsDenied,
		},
		{
			name: "create unexpected", tool: "policy_create", args: createArgs, expect: expectCreate,
			err: unexpected, assert: wantInternal,
		},
		{
			name: "update not found", tool: "policy_update", args: updateArgs, expect: expectPatch,
			err: policyNotFound, assert: wantPolicyNotFound,
		},
		{
			name: "update taken", tool: "policy_update", args: updateArgs, expect: expectPatch, err: nameTaken,
			assert: wantNameTaken,
		},
		{
			name: "update invalid", tool: "policy_update", args: updateArgs, expect: expectPatch, err: invalid,
			assert: wantInvalid,
		},
		{
			name: "update fields denied", tool: "policy_update", args: updateArgs, expect: expectPatch,
			err: fieldsDenied, assert: wantFieldsDenied,
		},
		{
			name: "delete not found", tool: "policy_delete", args: idArgs, expect: expectDelete,
			err: policyNotFound, assert: wantPolicyNotFound,
		},
		{
			name: "delete forbidden", tool: "policy_delete", args: idArgs, expect: expectDelete, err: denied,
			assert: wantForbidden,
		},
		{
			name: "delete unexpected", tool: "policy_delete", args: idArgs, expect: expectDelete, err: unexpected,
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
