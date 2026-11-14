package tools

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/mock"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
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
)

// newTestClient returns a client for the identity tools, backed by a mock service.
func newTestClient(t *testing.T) (*mcptest.Client, *mocks.MockIdentityService) {
	t.Helper()

	svc := mocks.NewMockIdentityService(t)

	return mcptest.New(t, New(svc)), svc
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

// TestTools_ReadableMembers checks that every member of the output but the ID can be hidden and has an
// action for reading it.
func TestTools_ReadableMembers(t *testing.T) {
	t.Parallel()

	mcptest.AssertReadableMembers(t, reflect.TypeFor[IdentityOutput](), service.Actions().ReadFields().Fields())
}

func TestTools_ServiceErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		assert func(r *mcptest.Result)
	}{
		{
			name:   "not found",
			err:    fmt.Errorf("get identity: %w", service.ErrIdentityNotFound),
			assert: func(r *mcptest.Result) { r.AssertToolError(errIdentityNotFound) },
		},
		{
			name:   "unexpected",
			err:    errors.New("connection refused"),
			assert: func(r *mcptest.Result) { r.AssertInternalError() },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mcp, svc := newTestClient(t)
			svc.EXPECT().Get(mock.Anything, apitest.Actor, apitest.Actor).Return(service.Identity{}, tt.err)

			tt.assert(mcp.Call("identity_whoami", ""))
		})
	}
}
