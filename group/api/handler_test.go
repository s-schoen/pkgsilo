package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/audit"
	"pkgsilo/core/envelope"
	"pkgsilo/core/pagination"
	"pkgsilo/group/service"
	"pkgsilo/group/service/mocks"
	identityapi "pkgsilo/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

var (
	maintainers = service.Group{
		ID:        uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		Name:      "Maintainers",
		CreatedBy: audit.SystemActor(),
		UpdatedBy: uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"),
		CreatedAt: time.Date(2026, time.November, 2, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.November, 2, 11, 30, 0, 123456000, time.UTC),
	}
	maintainersJSON = `{
		"id": "0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4",
		"name": "Maintainers",
		"createdBy": "00000000-0000-0000-0000-000000000000",
		"updatedBy": "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
		"createdAt": "2026-11-02T10:00:00Z",
		"updatedAt": "2026-11-02T11:30:00.123456Z"
	}`

	maintainersInput = service.GroupInput{Name: "Maintainers"}
	maintainersBody  = `{"name":"Maintainers"}`

	ada = service.Member{
		IdentityID: uuid.MustParse("3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f"),
		CreatedBy:  audit.SystemActor(),
		CreatedAt:  time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC),
	}
	adaJSON = `{
		"identityId": "3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f",
		"createdBy": "00000000-0000-0000-0000-000000000000",
		"createdAt": "2026-11-02T12:00:00Z"
	}`
)

const (
	maintainersPath = "/groups/0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"
	membersPath     = maintainersPath + "/members"
	adaPath         = membersPath + "/3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f"
)

// newTestClient returns a client for a handler that is backed by a mock service and mounted on the
// application router.
func newTestClient(t *testing.T, logger *slog.Logger) (*apitest.Client, *mocks.MockGroupService) {
	t.Helper()

	svc := mocks.NewMockGroupService(t)

	return apitest.NewClient(t, apitest.Mount("/groups", NewHandler(svc, logger).Routes())), svc
}

func TestHandler_List(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().List(mock.Anything, 0, 50).
		Return(pagination.Page[service.Group]{Items: []service.Group{maintainers}}, nil)

	api.Get("/groups").AssertCollection(`[`+maintainersJSON+`]`, envelope.Pagination{Offset: 0, Limit: 50})
}

func TestHandler_List_NextPage(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	page := pagination.Page[service.Group]{Items: []service.Group{maintainers}, HasMore: true}
	svc.EXPECT().List(mock.Anything, 2, 1).Return(page, nil)

	api.Get("/groups?offset=2&limit=1").
		AssertCollection(`[`+maintainersJSON+`]`, envelope.Pagination{Offset: 2, Limit: 1, NextOffset: new(3)})
}

func TestHandler_List_Empty(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().List(mock.Anything, 0, 50).Return(pagination.Page[service.Group]{}, nil)

	api.Get("/groups").AssertCollection(`[]`, envelope.Pagination{Offset: 0, Limit: 50})
}

func TestHandler_Create(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Create(mock.Anything, audit.SystemActor(), maintainersInput).Return(maintainers, nil)

	api.Post("/groups", maintainersBody).AssertCreated(maintainersPath, maintainersJSON)
}

func TestHandler_Get(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Get(mock.Anything, maintainers.ID).Return(maintainers, nil)

	api.Get(maintainersPath).AssertResource(maintainersJSON)
}

func TestHandler_Replace(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Replace(mock.Anything, audit.SystemActor(), maintainers.ID, maintainersInput).Return(maintainers, nil)

	api.Put(maintainersPath, maintainersBody).AssertResource(maintainersJSON)
}

func TestHandler_Delete(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Delete(mock.Anything, maintainers.ID).Return(nil)

	api.Delete(maintainersPath).AssertNoContent()
}

func TestHandler_ListMembers(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().ListMembers(mock.Anything, maintainers.ID, 0, 50).
		Return(pagination.Page[service.Member]{Items: []service.Member{ada}}, nil)

	api.Get(membersPath).AssertCollection(`[`+adaJSON+`]`, envelope.Pagination{Offset: 0, Limit: 50})
}

func TestHandler_ListMembers_NextPage(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	page := pagination.Page[service.Member]{Items: []service.Member{ada}, HasMore: true}
	svc.EXPECT().ListMembers(mock.Anything, maintainers.ID, 2, 1).Return(page, nil)

	api.Get(membersPath+"?offset=2&limit=1").
		AssertCollection(`[`+adaJSON+`]`, envelope.Pagination{Offset: 2, Limit: 1, NextOffset: new(3)})
}

func TestHandler_ListMembers_Empty(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().ListMembers(mock.Anything, maintainers.ID, 0, 50).Return(pagination.Page[service.Member]{}, nil)

	api.Get(membersPath).AssertCollection(`[]`, envelope.Pagination{Offset: 0, Limit: 50})
}

func TestHandler_AddMember(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().AddMember(mock.Anything, audit.SystemActor(), maintainers.ID, ada.IdentityID).Return(nil)

	api.Put(adaPath, "").AssertNoContent()
}

func TestHandler_AddMember_Body(t *testing.T) {
	t.Parallel()
	// The mock fails the test if the service is called.
	api, _ := newTestClient(t, loggertest.Discard())

	api.Put(adaPath, `{"role":"admin"}`).AssertProblem(envelope.BadRequestProblem.New(envelope.Occurrence{
		Detail: "The request body must be empty.",
	}))
}

func TestHandler_RemoveMember(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().RemoveMember(mock.Anything, maintainers.ID, ada.IdentityID).Return(nil)

	api.Delete(adaPath).AssertNoContent()
}

func TestHandler_InvalidPagination(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"/groups", membersPath} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test if the service is called.
			api, _ := newTestClient(t, loggertest.Discard())

			api.Get(target + "?limit=0").
				AssertInvalidQuery(envelope.ValidationError{Parameter: "limit", Detail: "must be no less than 1"})
		})
	}
}

func TestHandler_InvalidBody(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test if the service is called.
			api, _ := newTestClient(t, loggertest.Discard())
			target := "/groups"
			if method == http.MethodPut {
				target = maintainersPath
			}

			api.Send(method, target, `{"name":""}`).AssertInvalidBody(
				envelope.ValidationError{Pointer: "#/name", Detail: "cannot be blank"},
			)
		})
	}
}

func TestHandler_InvalidID(t *testing.T) {
	t.Parallel()
	const badGroup = "/groups/not-a-uuid"
	adaID := ada.IdentityID.String()
	tests := []struct {
		method, target string
	}{
		{method: http.MethodGet, target: badGroup},
		{method: http.MethodPut, target: badGroup},
		{method: http.MethodDelete, target: badGroup},
		{method: http.MethodGet, target: badGroup + "/members"},
		{method: http.MethodPut, target: badGroup + "/members/" + adaID},
		{method: http.MethodDelete, target: badGroup + "/members/" + adaID},
		{method: http.MethodPut, target: badGroup + "/members/not-a-uuid"},
		{method: http.MethodDelete, target: badGroup + "/members/not-a-uuid"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test if the service is called.
			api, _ := newTestClient(t, loggertest.Discard())

			api.Send(tt.method, tt.target, maintainersBody).AssertProblem(groupNotFoundProblem)
		})
	}
}

// TestHandler_InvalidMemberID checks that an identity ID that isn't a UUID is answered like an
// unknown identity, once the group was found to exist.
func TestHandler_InvalidMemberID(t *testing.T) {
	t.Parallel()
	const badMember = membersPath + "/not-a-uuid"
	groupNotFoundErr := fmt.Errorf("get group: %w", service.ErrGroupNotFound)
	tests := []struct {
		name     string
		method   string
		getErr   error
		want     envelope.Problem
		wantLogs int
	}{
		{name: "add", method: http.MethodPut, want: identityapi.IdentityNotFoundProblem},
		{name: "remove", method: http.MethodDelete, want: memberNotFoundProblem},
		{name: "add to missing group", method: http.MethodPut, getErr: groupNotFoundErr, want: groupNotFoundProblem},
		{
			name: "remove from missing group", method: http.MethodDelete,
			getErr: groupNotFoundErr, want: groupNotFoundProblem,
		},
		{
			name: "group unexpected", method: http.MethodPut, getErr: errors.New("connection refused"),
			want: envelope.InternalServerErrorProblem.New(envelope.Occurrence{}), wantLogs: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger, buf := loggertest.NewJSON()
			api, svc := newTestClient(t, logger)
			svc.EXPECT().Get(mock.Anything, maintainers.ID).Return(maintainers, tt.getErr)

			api.Send(tt.method, badMember, "").AssertProblem(tt.want)
			assert.Len(t, loggertest.Entries(t, buf), tt.wantLogs)
		})
	}
}

func TestHandler_ServiceErrors(t *testing.T) {
	t.Parallel()
	type endpoint struct {
		method, target, body string
		expect               func(svc *mocks.MockGroupService, err error)
	}
	var (
		list = endpoint{
			method: http.MethodGet, target: "/groups",
			expect: func(svc *mocks.MockGroupService, err error) {
				svc.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return(pagination.Page[service.Group]{}, err)
			},
		}
		create = endpoint{
			method: http.MethodPost, target: "/groups", body: maintainersBody,
			expect: func(svc *mocks.MockGroupService, err error) {
				svc.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(service.Group{}, err)
			},
		}
		get = endpoint{
			method: http.MethodGet, target: maintainersPath,
			expect: func(svc *mocks.MockGroupService, err error) {
				svc.EXPECT().Get(mock.Anything, mock.Anything).Return(service.Group{}, err)
			},
		}
		replace = endpoint{
			method: http.MethodPut, target: maintainersPath, body: maintainersBody,
			expect: func(svc *mocks.MockGroupService, err error) {
				svc.EXPECT().Replace(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(service.Group{}, err)
			},
		}
		del = endpoint{
			method: http.MethodDelete, target: maintainersPath,
			expect: func(svc *mocks.MockGroupService, err error) {
				svc.EXPECT().Delete(mock.Anything, mock.Anything).Return(err)
			},
		}
		listMembers = endpoint{
			method: http.MethodGet, target: membersPath,
			expect: func(svc *mocks.MockGroupService, err error) {
				svc.EXPECT().ListMembers(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(pagination.Page[service.Member]{}, err)
			},
		}
		addMember = endpoint{
			method: http.MethodPut, target: adaPath,
			expect: func(svc *mocks.MockGroupService, err error) {
				svc.EXPECT().AddMember(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(err)
			},
		}
		removeMember = endpoint{
			method: http.MethodDelete, target: adaPath,
			expect: func(svc *mocks.MockGroupService, err error) {
				svc.EXPECT().RemoveMember(mock.Anything, mock.Anything, mock.Anything).Return(err)
			},
		}
	)

	internal := envelope.InternalServerErrorProblem.New(envelope.Occurrence{})
	// Wrapped like the service wraps its errors.
	groupNotFoundErr := fmt.Errorf("get group: %w", service.ErrGroupNotFound)
	nameTakenErr := fmt.Errorf("create group: %w", service.ErrGroupNameTaken)
	identityNotFoundErr := fmt.Errorf("add group member: %w", service.ErrIdentityNotFound)
	memberNotFoundErr := fmt.Errorf("remove group member: %w", service.ErrMemberNotFound)
	unexpectedErr := errors.New("connection refused")

	tests := []struct {
		name     string
		endpoint endpoint
		err      error
		want     envelope.Problem
	}{
		{name: "list unexpected", endpoint: list, err: unexpectedErr, want: internal},
		{name: "create name taken", endpoint: create, err: nameTakenErr, want: groupNameTakenProblem},
		{name: "create unexpected", endpoint: create, err: unexpectedErr, want: internal},
		{name: "get not found", endpoint: get, err: groupNotFoundErr, want: groupNotFoundProblem},
		{name: "get unexpected", endpoint: get, err: unexpectedErr, want: internal},
		{name: "replace not found", endpoint: replace, err: groupNotFoundErr, want: groupNotFoundProblem},
		{name: "replace name taken", endpoint: replace, err: nameTakenErr, want: groupNameTakenProblem},
		{name: "replace unexpected", endpoint: replace, err: unexpectedErr, want: internal},
		{name: "delete not found", endpoint: del, err: groupNotFoundErr, want: groupNotFoundProblem},
		{name: "delete unexpected", endpoint: del, err: unexpectedErr, want: internal},
		{
			name: "list members group not found", endpoint: listMembers,
			err: groupNotFoundErr, want: groupNotFoundProblem,
		},
		{name: "list members unexpected", endpoint: listMembers, err: unexpectedErr, want: internal},
		{
			name: "add member group not found", endpoint: addMember,
			err: groupNotFoundErr, want: groupNotFoundProblem,
		},
		{
			name: "add member identity not found", endpoint: addMember,
			err: identityNotFoundErr, want: identityapi.IdentityNotFoundProblem,
		},
		{name: "add member unexpected", endpoint: addMember, err: unexpectedErr, want: internal},
		{
			name: "remove member group not found", endpoint: removeMember,
			err: groupNotFoundErr, want: groupNotFoundProblem,
		},
		{
			name: "remove member not found", endpoint: removeMember,
			err: memberNotFoundErr, want: memberNotFoundProblem,
		},
		{name: "remove member unexpected", endpoint: removeMember, err: unexpectedErr, want: internal},
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
	svc.EXPECT().Get(mock.Anything, maintainers.ID).Return(service.Group{}, errors.New("connection refused"))

	api.Get(maintainersPath).AssertInternalError()

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "group request failed", entries[0]["msg"])
	assert.Equal(t, "connection refused", entries[0]["error"])
}
