package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/feature/password/service"
	"pkgsilo/feature/password/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

var (
	// adaID is the identity of the password.
	adaID = uuid.MustParse("7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c")
	setAt = time.Date(2026, time.November, 17, 11, 30, 0, 123456000, time.UTC)

	adaPassword = service.Password{
		SetAt: authz.Visible(setAt), AllowedActions: authz.AllowedActions{"password:delete", "password:change"},
	}
	adaJSON = `{"setAt":"2026-11-17T11:30:00.123456Z","allowedActions":["password:delete","password:change"]}`

	passwordPath = "/identities/7a2d3b4c-5e6f-4071-8b8c-0d1e2f3a4b5c/password"
)

// newTestClient returns a client for a handler that is backed by a mock service and mounted on the
// application router, which checks every response against Docs.
func newTestClient(t *testing.T, logger *slog.Logger) (*apitest.Client, *mocks.MockPasswordService) {
	t.Helper()

	svc := mocks.NewMockPasswordService(t)
	routes := NewHandler(svc, logger).Routes()

	return apitest.NewClient(t, apitest.Mount("/identities/{identityID}/password", routes)).
		WithDocs("/identities/{identityID}/password", Docs()), svc
}

func TestHandler_Get(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Get(mock.Anything, apitest.Actor, adaID).Return(adaPassword, nil)

	api.Get(passwordPath).AssertResource(adaJSON)
}

// TestHandler_Get_HiddenFields checks that a hidden field is absent from the response, and that a password
// that the caller may do nothing to has an empty list of allowed actions.
func TestHandler_Get_HiddenFields(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Get(mock.Anything, apitest.Actor, adaID).
		Return(service.Password{AllowedActions: authz.AllowedActions{}}, nil)

	api.Get(passwordPath).AssertResource(`{"allowedActions":[]}`)
}

// TestHandler_Set_Unreadable checks that a password that the caller set but may not read is answered
// without fields and without allowed actions.
func TestHandler_Set_Unreadable(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Set(mock.Anything, apitest.Actor, adaID, mock.Anything).Return(service.Password{}, nil)

	api.Put(passwordPath, `{"newPassword":"battery staple"}`).AssertResource(`{}`)
}

// TestHandler_ReadableMembers checks that every member of the response can be hidden and has an action for
// reading it.
func TestHandler_ReadableMembers(t *testing.T) {
	t.Parallel()

	apitest.AssertReadableMembers(t, reflect.TypeFor[PasswordResponse](), service.Actions().ReadFields().Fields())
}

// TestHandler_AllowedActionsMember checks that the response lists the allowed actions, with the actions on
// passwords as the enum of its schema.
func TestHandler_AllowedActionsMember(t *testing.T) {
	t.Parallel()

	apitest.AssertAllowedActionsMember(t, reflect.TypeFor[PasswordResponse](), service.Actions().Allowable())
}

func TestHandler_Set(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want service.PasswordInput
	}{
		{
			name: "without current password", body: `{"newPassword":"battery staple"}`,
			want: service.PasswordInput{NewPassword: "battery staple"},
		},
		{
			name: "with current password", body: `{"newPassword":"battery staple","currentPassword":"correct horse"}`,
			want: service.PasswordInput{NewPassword: "battery staple", CurrentPassword: new("correct horse")},
		},
		{
			// Passwords are used exactly as sent: neither trimmed nor normalized.
			name: "untrimmed", body: `{"newPassword":" x ","currentPassword":""}`,
			want: service.PasswordInput{NewPassword: " x ", CurrentPassword: new("")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api, svc := newTestClient(t, loggertest.Discard())
			svc.EXPECT().Set(mock.Anything, apitest.Actor, adaID, tt.want).Return(adaPassword, nil)

			api.Put(passwordPath, tt.body).AssertResource(adaJSON)
		})
	}
}

func TestHandler_Set_InvalidBody(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"missing": `{}`,
		"empty":   `{"newPassword":""}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on any call.
			api, _ := newTestClient(t, loggertest.Discard())

			api.Put(passwordPath, body).
				AssertInvalidBody(envelope.ValidationError{Pointer: "#/newPassword", Detail: "cannot be blank"})
		})
	}
}

func TestHandler_Set_NullCurrentPassword(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Set(mock.Anything, apitest.Actor, adaID, service.PasswordInput{NewPassword: "battery staple"}).
		Return(adaPassword, nil)

	api.Put(passwordPath, `{"newPassword":"battery staple","currentPassword":null}`).AssertResource(adaJSON)
}

func TestHandler_Delete(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Delete(mock.Anything, apitest.Actor, adaID).Return(nil)

	api.Delete(passwordPath).AssertNoContent()
}

// TestHandler_Me checks that "me" in the path stands for the actor's identity.
func TestHandler_Me(t *testing.T) {
	t.Parallel()
	const mePath = "/identities/me/password"
	t.Run("get", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().Get(mock.Anything, apitest.Actor, apitest.Actor).Return(adaPassword, nil)

		api.Get(mePath).AssertResource(adaJSON)
	})
	t.Run("set", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().Set(mock.Anything, apitest.Actor, apitest.Actor, service.PasswordInput{NewPassword: "battery staple"}).
			Return(adaPassword, nil)

		api.Put(mePath, `{"newPassword":"battery staple"}`).AssertResource(adaJSON)
	})
	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().Delete(mock.Anything, apitest.Actor, apitest.Actor).Return(nil)

		api.Delete(mePath).AssertNoContent()
	})
}

func TestHandler_InvalidIdentityID(t *testing.T) {
	t.Parallel()
	const invalidPath = "/identities/not-a-uuid/password"
	tests := []struct {
		method string
		body   string
	}{
		{http.MethodGet, ""},
		{http.MethodPut, `{"newPassword":"battery staple"}`},
		{http.MethodDelete, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on any call.
			api, _ := newTestClient(t, loggertest.Discard())

			api.Send(tt.method, invalidPath, tt.body).AssertProblem(identityapi.IdentityNotFoundProblem)
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
		{name: "password not found", err: service.ErrPasswordNotFound, want: passwordNotFoundProblem},
		{name: "identity not found", err: service.ErrIdentityNotFound, want: identityapi.IdentityNotFoundProblem},
		{name: "denied", err: fmt.Errorf("set password: %w", authz.ErrDenied), want: forbiddenProblem},
		{
			name: "current password required", err: fmt.Errorf("set password: %w", service.ErrCurrentPasswordRequired),
			want: currentPasswordRequiredProblem,
		},
		{
			name: "wrong current password", err: fmt.Errorf("set password: %w", service.ErrWrongCurrentPassword),
			want: wrongCurrentPasswordProblem,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api, svc := newTestClient(t, loggertest.Discard())
			svc.EXPECT().Set(mock.Anything, apitest.Actor, adaID, mock.Anything).Return(service.Password{}, tt.err)

			api.Put(passwordPath, `{"newPassword":"battery staple"}`).AssertProblem(tt.want)
		})
	}
}

// TestHandler_EndpointErrors checks that every endpoint answers service errors through writeError.
func TestHandler_EndpointErrors(t *testing.T) {
	t.Parallel()

	t.Run("get", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().Get(mock.Anything, apitest.Actor, adaID).Return(service.Password{}, service.ErrPasswordNotFound)

		api.Get(passwordPath).AssertProblem(passwordNotFoundProblem)
	})
	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().Delete(mock.Anything, apitest.Actor, adaID).Return(fmt.Errorf("x: %w", authz.ErrDenied))

		api.Delete(passwordPath).AssertProblem(forbiddenProblem)
	})
}

// TestHandler_UnexpectedErrorIsLoggedButNotExposed checks that an unexpected error is logged and answered
// with a 500 without details, and that neither holds the passwords of the request.
func TestHandler_UnexpectedErrorIsLoggedButNotExposed(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	api, svc := newTestClient(t, logger)
	svc.EXPECT().Set(mock.Anything, apitest.Actor, adaID, mock.Anything).
		Return(service.Password{}, errors.New("connection refused"))

	resp := api.Put(passwordPath, `{"newPassword":"battery staple","currentPassword":"correct horse"}`)

	resp.AssertInternalError()
	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "password request failed", entries[0]["msg"])
	assert.Equal(t, "connection refused", entries[0]["error"])
	for _, secret := range []string{"battery", "horse"} {
		assert.NotContains(t, buf.String(), secret)
		assert.NotContains(t, resp.Body.String(), secret)
	}
}

// TestHandler_InvalidBodyDoesNotEchoPasswords checks that a problem about the body never repeats a password.
func TestHandler_InvalidBodyDoesNotEchoPasswords(t *testing.T) {
	t.Parallel()
	api, _ := newTestClient(t, loggertest.Discard())

	resp := api.Put(passwordPath, `{"newPassword":"","currentPassword":"correct horse"}`)

	resp.AssertStatus(http.StatusUnprocessableEntity)
	assert.NotContains(t, resp.Body.String(), "horse")
}
