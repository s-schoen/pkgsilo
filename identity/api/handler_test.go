package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"pkgsilo/core/config"
	"pkgsilo/core/envelope"
	"pkgsilo/core/router"
	"pkgsilo/identity/service"
	"pkgsilo/identity/service/mocks"
)

var (
	cest = time.FixedZone("CEST", 2*60*60)

	ada = service.Identity{
		ID:          uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		Username:    "ada",
		DisplayName: "Ada Lovelace",
		CreatedBy:   service.SystemActor,
		UpdatedBy:   uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"),
		// Not in UTC, to show that responses are.
		CreatedAt: time.Date(2026, 10, 24, 12, 0, 0, 0, cest),
		UpdatedAt: time.Date(2026, 10, 24, 13, 30, 0, 123456000, cest),
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

	grace = service.Identity{
		ID:        uuid.MustParse("2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e"),
		Username:  "grace",
		CreatedAt: time.Date(2026, 10, 24, 14, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 10, 24, 14, 0, 0, 0, time.UTC),
	}

	adaInput = service.IdentityInput{Username: "ada", DisplayName: "Ada Lovelace"}
	adaBody  = `{"username":"ada","displayName":"Ada Lovelace"}`
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestRouter mounts a handler backed by a mock service on the application router.
func newTestRouter(t *testing.T, logger *slog.Logger) (http.Handler, *mocks.MockIdentityService) {
	svc := mocks.NewMockIdentityService(t)
	r := router.New(config.Config{}, discardLogger())
	r.Mount("/identities", NewHandler(svc, logger).Routes())

	return r, svc
}

// serve sends a request with an optional JSON body to h.
func serve(h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func TestHandler_List(t *testing.T) {
	r, svc := newTestRouter(t, discardLogger())
	svc.EXPECT().List(mock.Anything, 0, 51).Return([]service.Identity{ada}, nil)

	rec := serve(r, http.MethodGet, "/identities", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"data":[`+adaJSON+`],"pagination":{"offset":0,"limit":50,"nextOffset":null}}`, rec.Body.String())
}

func TestHandler_List_NextPage(t *testing.T) {
	r, svc := newTestRouter(t, discardLogger())
	// The service is asked for one more identity than the page holds.
	svc.EXPECT().List(mock.Anything, 2, 2).Return([]service.Identity{ada, grace}, nil)

	rec := serve(r, http.MethodGet, "/identities?offset=2&limit=1", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"data":[`+adaJSON+`],"pagination":{"offset":2,"limit":1,"nextOffset":3}}`, rec.Body.String())
}

func TestHandler_List_Empty(t *testing.T) {
	r, svc := newTestRouter(t, discardLogger())
	svc.EXPECT().List(mock.Anything, 0, 51).Return(nil, nil)

	rec := serve(r, http.MethodGet, "/identities", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"data":[],"pagination":{"offset":0,"limit":50,"nextOffset":null}}`, rec.Body.String())
}

func TestHandler_List_InvalidPagination(t *testing.T) {
	r, _ := newTestRouter(t, discardLogger())

	rec := serve(r, http.MethodGet, "/identities?limit=0", "")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"type":"about:blank","title":"Bad Request","status":400,
		"detail":"The request contains invalid query parameters.",
		"errors":[{"detail":"must be no less than 1","parameter":"limit"}]}`, rec.Body.String())
}

func TestHandler_Create(t *testing.T) {
	r, svc := newTestRouter(t, discardLogger())
	svc.EXPECT().Create(mock.Anything, service.SystemActor, adaInput).Return(ada, nil)

	rec := serve(r, http.MethodPost, "/identities", adaBody)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, adaPath, rec.Header().Get("Location"))
	assert.JSONEq(t, `{"data":`+adaJSON+`}`, rec.Body.String())
}

func TestHandler_Get(t *testing.T) {
	r, svc := newTestRouter(t, discardLogger())
	svc.EXPECT().Get(mock.Anything, ada.ID).Return(ada, nil)

	rec := serve(r, http.MethodGet, adaPath, "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"data":`+adaJSON+`}`, rec.Body.String())
}

func TestHandler_Replace(t *testing.T) {
	r, svc := newTestRouter(t, discardLogger())
	svc.EXPECT().Replace(mock.Anything, service.SystemActor, ada.ID, adaInput).Return(ada, nil)

	rec := serve(r, http.MethodPut, adaPath, adaBody)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"data":`+adaJSON+`}`, rec.Body.String())
}

func TestHandler_Delete(t *testing.T) {
	r, svc := newTestRouter(t, discardLogger())
	svc.EXPECT().Delete(mock.Anything, ada.ID).Return(nil)

	rec := serve(r, http.MethodDelete, adaPath, "")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestHandler_InvalidBody(t *testing.T) {
	const want = `{"type":"about:blank","title":"Unprocessable Entity","status":422,
		"detail":"The request body contains invalid members.",
		"errors":[
			{"detail":"cannot be blank","pointer":"#/displayName"},
			{"detail":"the length must be between 3 and 64","pointer":"#/username"}
		]}`

	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			// The mock fails the test if the service is called.
			r, _ := newTestRouter(t, discardLogger())
			target := "/identities"
			if method == http.MethodPut {
				target = adaPath
			}

			rec := serve(r, method, target, `{"username":"ab"}`)

			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
			assert.JSONEq(t, want, rec.Body.String())
		})
	}
}

func TestHandler_InvalidID(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			r, _ := newTestRouter(t, discardLogger())

			rec := serve(r, method, "/identities/not-a-uuid", adaBody)

			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, `{"type":"about:blank","title":"Not Found","status":404,
				"detail":"The identity does not exist."}`, rec.Body.String())
		})
	}
}

func TestHandler_ServiceErrors(t *testing.T) {
	type endpoint struct {
		method, target, body string
		expect               func(svc *mocks.MockIdentityService, err error)
	}
	var (
		list = endpoint{method: http.MethodGet, target: "/identities", expect: func(svc *mocks.MockIdentityService, err error) {
			svc.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return(nil, err)
		}}
		create = endpoint{method: http.MethodPost, target: "/identities", body: adaBody, expect: func(svc *mocks.MockIdentityService, err error) {
			svc.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(service.Identity{}, err)
		}}
		get = endpoint{method: http.MethodGet, target: adaPath, expect: func(svc *mocks.MockIdentityService, err error) {
			svc.EXPECT().Get(mock.Anything, mock.Anything).Return(service.Identity{}, err)
		}}
		replace = endpoint{method: http.MethodPut, target: adaPath, body: adaBody, expect: func(svc *mocks.MockIdentityService, err error) {
			svc.EXPECT().Replace(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(service.Identity{}, err)
		}}
		del = endpoint{method: http.MethodDelete, target: adaPath, expect: func(svc *mocks.MockIdentityService, err error) {
			svc.EXPECT().Delete(mock.Anything, mock.Anything).Return(err)
		}}
	)

	const (
		notFound = `{"type":"about:blank","title":"Not Found","status":404,"detail":"The identity does not exist."}`
		conflict = `{"type":"about:blank","title":"Conflict","status":409,"detail":"The username is already taken."}`
		internal = `{"type":"about:blank","title":"Internal Server Error","status":500}`
	)
	// Wrapped like the service wraps its errors.
	notFoundErr := fmt.Errorf("get identity: %w", service.ErrIdentityNotFound)
	takenErr := fmt.Errorf("create identity: %w", service.ErrUsernameTaken)
	unexpectedErr := errors.New("connection refused")

	tests := []struct {
		name       string
		endpoint   endpoint
		err        error
		wantStatus int
		want       string
	}{
		{name: "list unexpected", endpoint: list, err: unexpectedErr, wantStatus: http.StatusInternalServerError, want: internal},
		{name: "create username taken", endpoint: create, err: takenErr, wantStatus: http.StatusConflict, want: conflict},
		{name: "create unexpected", endpoint: create, err: unexpectedErr, wantStatus: http.StatusInternalServerError, want: internal},
		{name: "get not found", endpoint: get, err: notFoundErr, wantStatus: http.StatusNotFound, want: notFound},
		{name: "get unexpected", endpoint: get, err: unexpectedErr, wantStatus: http.StatusInternalServerError, want: internal},
		{name: "replace not found", endpoint: replace, err: notFoundErr, wantStatus: http.StatusNotFound, want: notFound},
		{name: "replace username taken", endpoint: replace, err: takenErr, wantStatus: http.StatusConflict, want: conflict},
		{name: "replace unexpected", endpoint: replace, err: unexpectedErr, wantStatus: http.StatusInternalServerError, want: internal},
		{name: "delete not found", endpoint: del, err: notFoundErr, wantStatus: http.StatusNotFound, want: notFound},
		{name: "delete unexpected", endpoint: del, err: unexpectedErr, wantStatus: http.StatusInternalServerError, want: internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, svc := newTestRouter(t, discardLogger())
			tt.endpoint.expect(svc, tt.err)

			rec := serve(r, tt.endpoint.method, tt.endpoint.target, tt.endpoint.body)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.want, rec.Body.String())
		})
	}
}

func TestHandler_UnexpectedErrorIsLogged(t *testing.T) {
	var buf bytes.Buffer
	r, svc := newTestRouter(t, slog.New(slog.NewTextHandler(&buf, nil)))
	svc.EXPECT().Get(mock.Anything, ada.ID).Return(service.Identity{}, errors.New("connection refused"))

	serve(r, http.MethodGet, adaPath, "")

	assert.Contains(t, buf.String(), "identity request failed")
	assert.Contains(t, buf.String(), "connection refused")
}
