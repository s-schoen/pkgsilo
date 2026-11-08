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

	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	"pkgsilo/core/pagination"
	"pkgsilo/core/request"
	"pkgsilo/feature/apikey/service"
	"pkgsilo/feature/apikey/service/mocks"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

var (
	ciExpiresAt  = time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	ciLastUsedAt = time.Date(2026, time.November, 5, 11, 30, 0, 123456000, time.UTC)

	// ciIdentityID is the identity of ciKey.
	ciIdentityID = uuid.MustParse("7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c")

	ciKey = service.APIKey{
		ID:         uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"),
		IdentityID: authz.Visible(ciIdentityID),
		Name:       authz.Visible("ci"),
		KeySuffix:  authz.Visible("WXYZ"),
		ExpiresAt:  authz.Visible(&ciExpiresAt),
		LastUsedAt: authz.Visible(&ciLastUsedAt),
		CreatedBy:  authz.Visible(uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")),
		CreatedAt:  authz.Visible(time.Date(2026, time.November, 5, 10, 0, 0, 0, time.UTC)),
	}
	ciJSON = `{
		"id": "0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4",
		"identityId": "7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c",
		"name": "ci",
		"keySuffix": "WXYZ",
		"expiresAt": "2027-01-01T00:00:00Z",
		"lastUsedAt": "2026-11-05T11:30:00.123456Z",
		"createdBy": "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
		"createdAt": "2026-11-05T10:00:00Z"
	}`

	keysPath = "/identities/7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c/api-keys"
	ciPath   = keysPath + "/0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"
)

// newTestClient returns a client for a handler that is backed by a mock service and mounted on the
// application router.
func newTestClient(t *testing.T, logger *slog.Logger) (*apitest.Client, *mocks.MockAPIKeyService) {
	t.Helper()

	svc := mocks.NewMockAPIKeyService(t)
	routes := NewHandler(svc, logger).Routes()

	return apitest.NewClient(t, apitest.Mount("/identities/{identityID}/api-keys", routes)), svc
}

func TestHandler_List(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().List(mock.Anything, apitest.Actor, ciIdentityID, 0, 50).
		Return(pagination.Page[service.APIKey]{Items: []service.APIKey{ciKey}}, nil)

	api.Get(keysPath).AssertCollection(`[`+ciJSON+`]`, envelope.Pagination{Offset: 0, Limit: 50})
}

func TestHandler_List_NextPage(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	page := pagination.Page[service.APIKey]{Items: []service.APIKey{ciKey}, HasMore: true}
	svc.EXPECT().List(mock.Anything, apitest.Actor, ciIdentityID, 2, 1).Return(page, nil)

	api.Get(keysPath+"?offset=2&limit=1").
		AssertCollection(`[`+ciJSON+`]`, envelope.Pagination{Offset: 2, Limit: 1, NextOffset: new(3)})
}

func TestHandler_List_InvalidPagination(t *testing.T) {
	t.Parallel()
	api, _ := newTestClient(t, loggertest.Discard())

	api.Get(keysPath + "?limit=0").
		AssertInvalidQuery(envelope.ValidationError{Parameter: "limit", Detail: "must be no less than 1"})
}

func TestHandler_Create(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	created := service.CreatedAPIKey{APIKey: ciKey, Key: "psk_NEWKEY"}
	expiresAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	in := service.APIKeyInput{Name: "ci", ExpiresAt: &expiresAt}
	svc.EXPECT().Create(mock.Anything, apitest.Actor, ciIdentityID, in).Return(created, nil)

	resp := api.Post(keysPath, `{"name":"ci","expiresAt":"`+expiresAt.Format(time.RFC3339)+`"}`)

	resp.AssertCreated(ciPath, ciJSON[:len(ciJSON)-1]+`,"key":"psk_NEWKEY"}`)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"), "the key must not end up in a cache")
}

func TestHandler_Create_NeverExpires(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	created := service.APIKey{
		ID:         ciKey.ID,
		ExpiresAt:  authz.Visible[*time.Time](nil),
		LastUsedAt: authz.Visible[*time.Time](nil),
	}
	svc.EXPECT().Create(mock.Anything, apitest.Actor, ciIdentityID, service.APIKeyInput{Name: "ci"}).
		Return(service.CreatedAPIKey{APIKey: created, Key: "psk_NEWKEY"}, nil)

	api.Post(keysPath, `{"name":"ci"}`).AssertCreated(ciPath, `{
		"id": "0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4",
		"expiresAt": null,
		"lastUsedAt": null,
		"key": "psk_NEWKEY"
	}`)
}

// TestHandler_Create_Unreadable checks that a new API key that the caller may not read is answered with only
// its ID, Location and key, since the key can't be retrieved later.
func TestHandler_Create_Unreadable(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Create(mock.Anything, apitest.Actor, ciIdentityID, service.APIKeyInput{Name: "ci"}).
		Return(service.CreatedAPIKey{ID: ciKey.ID, Key: "psk_NEWKEY"}, nil)

	api.Post(keysPath, `{"name":"ci"}`).
		AssertCreated(ciPath, `{"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4","key":"psk_NEWKEY"}`)
}

// TestHandler_Get_HiddenFields checks that a hidden field is absent from the response, unlike a visible
// null.
func TestHandler_Get_HiddenFields(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Get(mock.Anything, apitest.Actor, ciIdentityID, ciKey.ID).
		Return(service.APIKey{ID: ciKey.ID, Name: ciKey.Name, LastUsedAt: authz.Visible[*time.Time](nil)}, nil)

	api.Get(ciPath).
		AssertResource(`{"id":"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4","name":"ci","lastUsedAt":null}`)
}

// TestHandler_ReadableMembers checks that every member of the response but the ID can be hidden and has
// an action for reading it, and that a new API key adds nothing to it but its key.
func TestHandler_ReadableMembers(t *testing.T) {
	t.Parallel()

	apitest.AssertReadableMembers(t, reflect.TypeFor[KeyResponse](), service.ReadableFields())

	var members []string
	for field := range reflect.TypeFor[CreatedKeyResponse]().Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		members = append(members, name)
	}
	assert.Equal(t, []string{"", "key"}, members, "a new API key must be the API key and its key")
}

func TestHandler_Create_InvalidBody(t *testing.T) {
	t.Parallel()
	api, _ := newTestClient(t, loggertest.Discard())
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)

	api.Post(keysPath, `{"name":" ","expiresAt":"`+past+`"}`).AssertInvalidBody(
		envelope.ValidationError{Pointer: "#/expiresAt", Detail: "must be in the future"},
		envelope.ValidationError{Pointer: "#/name", Detail: "cannot be blank"},
	)
}

func TestHandler_Create_MalformedExpiry(t *testing.T) {
	t.Parallel()
	api, _ := newTestClient(t, loggertest.Discard())

	api.Post(keysPath, `{"name":"ci","expiresAt":"tomorrow"}`).AssertStatus(http.StatusBadRequest)
}

func TestHandler_Get(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Get(mock.Anything, apitest.Actor, ciIdentityID, ciKey.ID).Return(ciKey, nil)

	api.Get(ciPath).AssertResource(ciJSON)
}

func TestHandler_Delete(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Delete(mock.Anything, apitest.Actor, ciIdentityID, ciKey.ID).Return(nil)

	api.Delete(ciPath).AssertNoContent()
}

func TestHandler_NoUpdateRoute(t *testing.T) {
	t.Parallel()
	api, _ := newTestClient(t, loggertest.Discard())

	api.Put(ciPath, `{"name":"renamed"}`).AssertStatus(http.StatusMethodNotAllowed)
}

func TestHandler_InvalidIDs(t *testing.T) {
	t.Parallel()
	const invalidKeysPath = "/identities/not-a-uuid/api-keys"
	invalidKeyPath := invalidKeysPath + "/" + ciKey.ID.String()
	identityProblem := identityapi.IdentityNotFoundProblem
	tests := []struct {
		name   string
		method string
		target string
		body   string
		want   envelope.Problem
	}{
		{"list, identity", http.MethodGet, invalidKeysPath, "", identityProblem},
		{"create, identity", http.MethodPost, invalidKeysPath, `{"name":"ci"}`, identityProblem},
		{"get, identity", http.MethodGet, invalidKeyPath, "", identityProblem},
		{"delete, identity", http.MethodDelete, invalidKeyPath, "", identityProblem},
		{"get, api key", http.MethodGet, keysPath + "/not-a-uuid", "", apiKeyNotFoundProblem},
		{"delete, api key", http.MethodDelete, keysPath + "/not-a-uuid", "", apiKeyNotFoundProblem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on any call.
			api, _ := newTestClient(t, loggertest.Discard())

			api.Send(tt.method, tt.target, tt.body).AssertProblem(tt.want)
		})
	}
}

func TestHandler_ServiceErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want envelope.Problem
	}{
		{name: "api key not found", err: service.ErrAPIKeyNotFound, want: apiKeyNotFoundProblem},
		{name: "identity not found", err: service.ErrIdentityNotFound, want: identityapi.IdentityNotFoundProblem},
		{name: "denied", err: fmt.Errorf("delete api key: %w", authz.ErrDenied), want: forbiddenProblem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api, svc := newTestClient(t, loggertest.Discard())
			svc.EXPECT().Delete(mock.Anything, apitest.Actor, ciIdentityID, ciKey.ID).Return(tt.err)

			api.Delete(ciPath).AssertProblem(tt.want)
		})
	}
}

func TestHandler_Create_FieldsDenied(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Create(mock.Anything, apitest.Actor, ciIdentityID, service.APIKeyInput{Name: "ci"}).
		Return(service.CreatedAPIKey{}, fmt.Errorf("create api key: %w",
			&authz.FieldsDeniedError{Fields: []string{"name", "expiresAt"}}))

	resp := api.Post(keysPath, `{"name":"ci"}`)

	resp.AssertProblem(request.FieldsDeniedProblem("name", "expiresAt"))
	assert.Empty(t, resp.Header().Get("Location"))
}

// TestHandler_SettableMembersHaveActions checks that every member that the request body sets has an action
// of its own in the service, and that every such action has a member. A member without one would be
// written without being authorized, and a field denial couldn't point to a member that doesn't exist.
func TestHandler_SettableMembersHaveActions(t *testing.T) {
	t.Parallel()
	var members []string
	for field := range reflect.TypeFor[CreateKeyRequest]().Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		members = append(members, name)
	}

	assert.ElementsMatch(t, members, service.CreatableFields())
}

// TestHandler_EndpointErrors checks that every endpoint answers service errors through writeError.
func TestHandler_EndpointErrors(t *testing.T) {
	t.Parallel()
	notFound := service.ErrIdentityNotFound

	t.Run("list", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().List(mock.Anything, apitest.Actor, ciIdentityID, 0, 50).
			Return(pagination.Page[service.APIKey]{}, notFound)

		api.Get(keysPath).AssertProblem(identityapi.IdentityNotFoundProblem)
	})
	t.Run("create", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().Create(mock.Anything, apitest.Actor, ciIdentityID, service.APIKeyInput{Name: "ci"}).
			Return(service.CreatedAPIKey{}, notFound)

		resp := api.Post(keysPath, `{"name":"ci"}`)

		resp.AssertProblem(identityapi.IdentityNotFoundProblem)
		assert.Empty(t, resp.Header().Get("Location"))
	})
	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().Delete(mock.Anything, apitest.Actor, ciIdentityID, ciKey.ID).Return(notFound)

		api.Delete(ciPath).AssertProblem(identityapi.IdentityNotFoundProblem)
	})
}

func TestHandler_UnexpectedErrorIsLoggedButNotExposed(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	api, svc := newTestClient(t, logger)
	svc.EXPECT().Get(mock.Anything, apitest.Actor, ciIdentityID, ciKey.ID).
		Return(service.APIKey{}, errors.New("connection refused"))

	api.Get(ciPath).AssertInternalError()

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "api key request failed", entries[0]["msg"])
	assert.Equal(t, "connection refused", entries[0]["error"])
}
