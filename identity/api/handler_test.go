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

	"pkgsilo/core/envelope"
	"pkgsilo/core/pagination"
	"pkgsilo/identity/service"
	"pkgsilo/identity/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

var (
	ada = service.Identity{
		ID:          uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		Username:    "ada",
		DisplayName: "Ada Lovelace",
		CreatedBy:   service.SystemActor,
		UpdatedBy:   uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"),
		CreatedAt:   time.Date(2026, time.October, 24, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, time.October, 24, 11, 30, 0, 123456000, time.UTC),
	}
	adaJSON = `{
		"id": "0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4",
		"username": "ada",
		"displayName": "Ada Lovelace",
		"createdBy": "00000000-0000-0000-0000-000000000000",
		"updatedBy": "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
		"createdAt": "2026-10-24T10:00:00Z",
		"updatedAt": "2026-10-24T11:30:00.123456Z"
	}`
	adaPath = "/identities/0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"

	adaInput = service.IdentityInput{Username: "ada", DisplayName: "Ada Lovelace"}
	adaBody  = `{"username":"ada","displayName":"Ada Lovelace"}`
)

// newTestClient returns a client for a handler that is backed by a mock service and mounted on the
// application router.
func newTestClient(t *testing.T, logger *slog.Logger) (*apitest.Client, *mocks.MockIdentityService) {
	t.Helper()

	svc := mocks.NewMockIdentityService(t)

	return apitest.NewClient(t, apitest.Mount("/identities", NewHandler(svc, logger).Routes())), svc
}

func TestHandler_List(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().List(mock.Anything, 0, 50).Return(pagination.Page[service.Identity]{Items: []service.Identity{ada}}, nil)

	api.Get("/identities").AssertCollection(`[`+adaJSON+`]`, envelope.Pagination{Offset: 0, Limit: 50})
}

func TestHandler_List_NextPage(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	page := pagination.Page[service.Identity]{Items: []service.Identity{ada}, HasMore: true}
	svc.EXPECT().List(mock.Anything, 2, 1).Return(page, nil)

	api.Get("/identities?offset=2&limit=1").
		AssertCollection(`[`+adaJSON+`]`, envelope.Pagination{Offset: 2, Limit: 1, NextOffset: new(3)})
}

func TestHandler_List_Empty(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().List(mock.Anything, 0, 50).Return(pagination.Page[service.Identity]{}, nil)

	api.Get("/identities").AssertCollection(`[]`, envelope.Pagination{Offset: 0, Limit: 50})
}

func TestHandler_List_InvalidPagination(t *testing.T) {
	t.Parallel()
	api, _ := newTestClient(t, loggertest.Discard())

	api.Get("/identities?limit=0").
		AssertInvalidQuery(envelope.ValidationError{Parameter: "limit", Detail: "must be no less than 1"})
}

func TestHandler_Create(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Create(mock.Anything, service.SystemActor, adaInput).Return(ada, nil)

	api.Post("/identities", adaBody).AssertCreated(adaPath, adaJSON)
}

func TestHandler_Get(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Get(mock.Anything, ada.ID).Return(ada, nil)

	api.Get(adaPath).AssertResource(adaJSON)
}

func TestHandler_Replace(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Replace(mock.Anything, service.SystemActor, ada.ID, adaInput).Return(ada, nil)

	api.Put(adaPath, adaBody).AssertResource(adaJSON)
}

func TestHandler_Delete(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Delete(mock.Anything, ada.ID).Return(nil)

	api.Delete(adaPath).AssertNoContent()
}

func TestHandler_InvalidBody(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test if the service is called.
			api, _ := newTestClient(t, loggertest.Discard())
			target := "/identities"
			if method == http.MethodPut {
				target = adaPath
			}

			api.Send(method, target, `{"username":"ab"}`).AssertInvalidBody(
				envelope.ValidationError{Pointer: "#/displayName", Detail: "cannot be blank"},
				envelope.ValidationError{Pointer: "#/username", Detail: "the length must be between 3 and 64"},
			)
		})
	}
}

func TestHandler_InvalidID(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			api, _ := newTestClient(t, loggertest.Discard())

			api.Send(method, "/identities/not-a-uuid", adaBody).AssertProblem(identityNotFoundProblem)
		})
	}
}

func TestHandler_ServiceErrors(t *testing.T) {
	t.Parallel()
	type endpoint struct {
		method, target, body string
		expect               func(svc *mocks.MockIdentityService, err error)
	}
	var (
		list = endpoint{
			method: http.MethodGet, target: "/identities",
			expect: func(svc *mocks.MockIdentityService, err error) {
				svc.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return(pagination.Page[service.Identity]{}, err)
			},
		}
		create = endpoint{
			method: http.MethodPost, target: "/identities", body: adaBody,
			expect: func(svc *mocks.MockIdentityService, err error) {
				svc.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(service.Identity{}, err)
			},
		}
		get = endpoint{
			method: http.MethodGet, target: adaPath,
			expect: func(svc *mocks.MockIdentityService, err error) {
				svc.EXPECT().Get(mock.Anything, mock.Anything).Return(service.Identity{}, err)
			},
		}
		replace = endpoint{
			method: http.MethodPut, target: adaPath, body: adaBody,
			expect: func(svc *mocks.MockIdentityService, err error) {
				svc.EXPECT().Replace(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(service.Identity{}, err)
			},
		}
		del = endpoint{
			method: http.MethodDelete, target: adaPath,
			expect: func(svc *mocks.MockIdentityService, err error) {
				svc.EXPECT().Delete(mock.Anything, mock.Anything).Return(err)
			},
		}
	)

	internal := envelope.InternalServerErrorProblem.New(envelope.Occurrence{})
	// Wrapped like the service wraps its errors.
	notFoundErr := fmt.Errorf("get identity: %w", service.ErrIdentityNotFound)
	takenErr := fmt.Errorf("create identity: %w", service.ErrUsernameTaken)
	unexpectedErr := errors.New("connection refused")

	tests := []struct {
		name     string
		endpoint endpoint
		err      error
		want     envelope.Problem
	}{
		{name: "list unexpected", endpoint: list, err: unexpectedErr, want: internal},
		{name: "create username taken", endpoint: create, err: takenErr, want: usernameTakenProblem},
		{name: "create unexpected", endpoint: create, err: unexpectedErr, want: internal},
		{name: "get not found", endpoint: get, err: notFoundErr, want: identityNotFoundProblem},
		{name: "get unexpected", endpoint: get, err: unexpectedErr, want: internal},
		{name: "replace not found", endpoint: replace, err: notFoundErr, want: identityNotFoundProblem},
		{name: "replace username taken", endpoint: replace, err: takenErr, want: usernameTakenProblem},
		{name: "replace unexpected", endpoint: replace, err: unexpectedErr, want: internal},
		{name: "delete not found", endpoint: del, err: notFoundErr, want: identityNotFoundProblem},
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
	svc.EXPECT().Get(mock.Anything, ada.ID).Return(service.Identity{}, errors.New("connection refused"))

	api.Get(adaPath).AssertInternalError()

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "identity request failed", entries[0]["msg"])
	assert.Equal(t, "connection refused", entries[0]["error"])
}
