package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	"pkgsilo/core/pagination"
	"pkgsilo/core/request"
	"pkgsilo/feature/policy/service"
	"pkgsilo/feature/policy/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

var (
	everything = service.Policy{
		ID:        uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		Name:      authz.Visible("everything"),
		Source:    authz.Visible(`permit (principal, action, resource);`),
		CreatedBy: authz.Visible(audit.SystemActor()),
		UpdatedBy: authz.Visible(uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")),
		CreatedAt: authz.Visible(time.Date(2026, time.November, 7, 10, 0, 0, 0, time.UTC)),
		UpdatedAt: authz.Visible(time.Date(2026, time.November, 7, 11, 30, 0, 123456000, time.UTC)),
	}
	everythingJSON = `{
		"id": "0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4",
		"name": "everything",
		"source": "permit (principal, action, resource);",
		"createdBy": "00000000-0000-0000-0000-000000000000",
		"updatedBy": "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
		"createdAt": "2026-11-07T10:00:00Z",
		"updatedAt": "2026-11-07T11:30:00.123456Z"
	}`
	everythingPath = "/policies/0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"

	everythingInput = service.PolicyInput{Name: "everything", Source: `permit (principal, action, resource);`}
	everythingBody  = `{"name":"everything","source":"permit (principal, action, resource);"}`
)

// newTestClient returns a client for a handler that is backed by a mock service and mounted on the
// application router.
func newTestClient(t *testing.T, logger *slog.Logger) (*apitest.Client, *mocks.MockPolicyService) {
	t.Helper()

	svc := mocks.NewMockPolicyService(t)

	return apitest.NewClient(t, apitest.Mount("/policies", NewHandler(svc, logger).Routes())), svc
}

func TestHandler_List(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().List(mock.Anything, apitest.Actor, 0, 50).
		Return(pagination.Page[service.Policy]{Items: []service.Policy{everything}}, nil)

	api.Get("/policies").AssertCollection(`[`+everythingJSON+`]`, envelope.Pagination{Offset: 0, Limit: 50})
}

func TestHandler_List_NextPage(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	page := pagination.Page[service.Policy]{Items: []service.Policy{everything}, HasMore: true}
	svc.EXPECT().List(mock.Anything, apitest.Actor, 2, 1).Return(page, nil)

	api.Get("/policies?offset=2&limit=1").
		AssertCollection(`[`+everythingJSON+`]`, envelope.Pagination{Offset: 2, Limit: 1, NextOffset: new(3)})
}

func TestHandler_List_InvalidPagination(t *testing.T) {
	t.Parallel()
	api, _ := newTestClient(t, loggertest.Discard())

	api.Get("/policies?limit=0").
		AssertInvalidQuery(envelope.ValidationError{Parameter: "limit", Detail: "must be no less than 1"})
}

func TestHandler_Create(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Create(mock.Anything, apitest.Actor, everythingInput).Return(everything, nil)

	api.Post("/policies", everythingBody).AssertCreated(everythingPath, everythingJSON)
}

func TestHandler_Get(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Get(mock.Anything, apitest.Actor, everything.ID).Return(everything, nil)

	api.Get(everythingPath).AssertResource(everythingJSON)
}

// TestHandler_Get_HiddenFields checks that a hidden field is absent from the response.
func TestHandler_Get_HiddenFields(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Get(mock.Anything, apitest.Actor, everything.ID).
		Return(service.Policy{ID: everything.ID, Name: everything.Name}, nil)

	api.Get(everythingPath).AssertResource(`{"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4","name":"everything"}`)
}

// TestHandler_Create_Unreadable checks that a new policy that the caller may not read is answered with only
// its ID and Location.
func TestHandler_Create_Unreadable(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Create(mock.Anything, apitest.Actor, everythingInput).Return(service.Policy{ID: everything.ID}, nil)

	api.Post("/policies", everythingBody).
		AssertCreated(everythingPath, `{"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"}`)
}

// TestHandler_ReadableMembers checks that every member of the response but the ID can be hidden and has
// an action for reading it.
func TestHandler_ReadableMembers(t *testing.T) {
	t.Parallel()

	apitest.AssertReadableMembers(t, reflect.TypeFor[PolicyResponse](), service.ReadableFields())
}

func TestHandler_Replace(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Replace(mock.Anything, apitest.Actor, everything.ID, everythingInput).Return(everything, nil)

	api.Put(everythingPath, everythingBody).AssertResource(everythingJSON)
}

func TestHandler_Delete(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Delete(mock.Anything, apitest.Actor, everything.ID).Return(nil)

	api.Delete(everythingPath).AssertNoContent()
}

func TestHandler_InvalidBody(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test if the service is called.
			api, _ := newTestClient(t, loggertest.Discard())
			target := "/policies"
			if method == http.MethodPut {
				target = everythingPath
			}

			api.Send(method, target, `{"name":" "}`).AssertInvalidBody(
				envelope.ValidationError{Pointer: "#/name", Detail: "cannot be blank"},
				envelope.ValidationError{Pointer: "#/source", Detail: "cannot be blank"},
			)
		})
	}
}

// TestHandler_SettableMembersHaveActions checks that every member that a request body sets has an action of
// its own in the service, and that every such action has a member. A member without one would be written
// without being authorized, and a field denial couldn't point to a member that doesn't exist.
func TestHandler_SettableMembersHaveActions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    reflect.Type
		actions []string
	}{
		{name: "create", body: reflect.TypeFor[CreatePolicyRequest](), actions: service.CreatableFields()},
		{name: "replace", body: reflect.TypeFor[ReplacePolicyRequest](), actions: service.UpdatableFields()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var members []string
			for field := range tt.body.Fields() {
				name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				members = append(members, name)
			}

			assert.ElementsMatch(t, members, tt.actions)
		})
	}
}

func TestHandler_InvalidID(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			api, _ := newTestClient(t, loggertest.Discard())

			api.Send(method, "/policies/not-a-uuid", everythingBody).AssertProblem(policyNotFoundProblem)
		})
	}
}

func TestHandler_ServiceErrors(t *testing.T) {
	t.Parallel()
	type endpoint struct {
		method, target, body string
		expect               func(svc *mocks.MockPolicyService, err error)
	}
	var (
		list = endpoint{
			method: http.MethodGet, target: "/policies",
			expect: func(svc *mocks.MockPolicyService, err error) {
				svc.EXPECT().List(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(pagination.Page[service.Policy]{}, err)
			},
		}
		create = endpoint{
			method: http.MethodPost, target: "/policies", body: everythingBody,
			expect: func(svc *mocks.MockPolicyService, err error) {
				svc.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(service.Policy{}, err)
			},
		}
		get = endpoint{
			method: http.MethodGet, target: everythingPath,
			expect: func(svc *mocks.MockPolicyService, err error) {
				svc.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(service.Policy{}, err)
			},
		}
		replace = endpoint{
			method: http.MethodPut, target: everythingPath, body: everythingBody,
			expect: func(svc *mocks.MockPolicyService, err error) {
				svc.EXPECT().Replace(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(service.Policy{}, err)
			},
		}
		del = endpoint{
			method: http.MethodDelete, target: everythingPath,
			expect: func(svc *mocks.MockPolicyService, err error) {
				svc.EXPECT().Delete(mock.Anything, mock.Anything, mock.Anything).Return(err)
			},
		}
	)

	internal := envelope.InternalServerErrorProblem.New(envelope.Occurrence{})
	// Wrapped like the service wraps its errors.
	notFoundErr := fmt.Errorf("get policy: %w", service.ErrPolicyNotFound)
	takenErr := fmt.Errorf("create policy: %w", service.ErrPolicyNameTaken)
	invalidErr := fmt.Errorf("create policy: %w", &service.InvalidPolicyError{Reason: "the source holds no statements"})
	invalid := invalidPolicyType.New(envelope.Occurrence{Detail: "The policy is invalid: the source holds no statements"})
	fieldsDeniedErr := fmt.Errorf("create policy: %w", &authz.FieldsDeniedError{Fields: []string{"name", "source"}})
	fieldsDenied := request.FieldsDeniedProblem("name", "source")
	deniedErr := fmt.Errorf("delete policy: %w", authz.ErrDenied)
	unexpectedErr := errors.New("connection refused")

	tests := []struct {
		name     string
		endpoint endpoint
		err      error
		want     envelope.Problem
	}{
		{name: "list unexpected", endpoint: list, err: unexpectedErr, want: internal},
		{name: "create name taken", endpoint: create, err: takenErr, want: policyNameTakenProblem},
		{name: "create invalid", endpoint: create, err: invalidErr, want: invalid},
		{name: "create fields denied", endpoint: create, err: fieldsDeniedErr, want: fieldsDenied},
		{name: "create unexpected", endpoint: create, err: unexpectedErr, want: internal},
		{name: "get not found", endpoint: get, err: notFoundErr, want: policyNotFoundProblem},
		{name: "get unexpected", endpoint: get, err: unexpectedErr, want: internal},
		{name: "replace not found", endpoint: replace, err: notFoundErr, want: policyNotFoundProblem},
		{name: "replace name taken", endpoint: replace, err: takenErr, want: policyNameTakenProblem},
		{name: "replace invalid", endpoint: replace, err: invalidErr, want: invalid},
		{name: "replace fields denied", endpoint: replace, err: fieldsDeniedErr, want: fieldsDenied},
		{name: "replace unexpected", endpoint: replace, err: unexpectedErr, want: internal},
		{name: "delete not found", endpoint: del, err: notFoundErr, want: policyNotFoundProblem},
		{name: "delete denied", endpoint: del, err: deniedErr, want: forbiddenProblem},
		{name: "delete unexpected", endpoint: del, err: unexpectedErr, want: internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api, svc := newTestClient(t, loggertest.Discard())
			tt.endpoint.expect(svc, tt.err)

			api.Send(tt.endpoint.method, tt.endpoint.target, tt.endpoint.body).AssertProblem(tt.want)
		})
	}
}

func TestHandler_UnexpectedErrorIsLogged(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	api, svc := newTestClient(t, logger)
	svc.EXPECT().Get(mock.Anything, apitest.Actor, everything.ID).
		Return(service.Policy{}, errors.New("connection refused"))

	api.Get(everythingPath).AssertInternalError()

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "policy request failed", entries[0]["msg"])
	assert.Equal(t, "connection refused", entries[0]["error"])
}
