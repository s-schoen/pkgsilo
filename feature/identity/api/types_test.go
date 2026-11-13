package api

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/request"
)

func TestIdentityRequests_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		username    string
		displayName string
		// wantInvalid are the members that fail validation.
		wantInvalid []string
	}{
		{name: "valid", username: "ada", displayName: "Ada Lovelace"},
		{name: "separators", username: "ada.love-lace_1", displayName: "Ada"},
		{name: "longest", username: strings.Repeat("a", 64), displayName: strings.Repeat("é", 128)},
		{name: "blank", wantInvalid: []string{"displayName", "username"}},
		{name: "username too short", username: "ab", displayName: "Ada", wantInvalid: []string{"username"}},
		{name: "username too long", username: strings.Repeat("a", 65), displayName: "Ada", wantInvalid: []string{"username"}},
		{name: "username uppercase", username: "Ada", displayName: "Ada", wantInvalid: []string{"username"}},
		{name: "username special character", username: "ada!", displayName: "Ada", wantInvalid: []string{"username"}},
		{name: "username space", username: "ada lovelace", displayName: "Ada", wantInvalid: []string{"username"}},
		{name: "username leading separator", username: "-ada", displayName: "Ada", wantInvalid: []string{"username"}},
		{name: "username trailing separator", username: "ada.", displayName: "Ada", wantInvalid: []string{"username"}},
		{name: "username double separator", username: "ada..lovelace", displayName: "Ada", wantInvalid: []string{"username"}},
		{
			name: "display name too long", username: "ada", displayName: strings.Repeat("é", 129),
			wantInvalid: []string{"displayName"},
		},
		{name: "display name whitespace", username: "ada", displayName: " \t ", wantInvalid: []string{"displayName"}},
		{name: "display name NUL", username: "ada", displayName: "Ada\x00", wantInvalid: []string{"displayName"}},
		{name: "display name line break", username: "ada", displayName: "Ada\nL", wantInvalid: []string{"displayName"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			requests := map[string]validation.Validatable{
				"create":  CreateIdentityRequest{Username: tt.username, DisplayName: tt.displayName},
				"replace": ReplaceIdentityRequest{Username: tt.username, DisplayName: tt.displayName},
				"patch": PatchIdentityRequest{
					Username:    request.Optional[string]{Value: tt.username, Set: true},
					DisplayName: request.Optional[string]{Value: tt.displayName, Set: true},
				},
			}
			for kind, req := range requests {
				err := req.Validate()
				if len(tt.wantInvalid) == 0 {
					require.NoError(t, err, kind)
					continue
				}
				assert.ElementsMatch(t, tt.wantInvalid, invalidMembers(err), kind)
			}
		})
	}
}

// TestPatchIdentityRequest_Validate checks that a member of a patch may be left out, but not set to null.
func TestPatchIdentityRequest_Validate(t *testing.T) {
	t.Parallel()
	absent := request.Optional[string]{}
	null := request.Optional[string]{Set: true, Null: true}
	tests := []struct {
		name        string
		req         PatchIdentityRequest
		wantInvalid []string
	}{
		{name: "empty", req: PatchIdentityRequest{Username: absent, DisplayName: absent}},
		{name: "username", req: PatchIdentityRequest{Username: request.Optional[string]{Value: "ada", Set: true}}},
		{
			name: "null", req: PatchIdentityRequest{Username: null, DisplayName: null},
			wantInvalid: []string{"displayName", "username"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.req.Validate()

			if len(tt.wantInvalid) == 0 {
				require.NoError(t, err)
				return
			}
			assert.ElementsMatch(t, tt.wantInvalid, invalidMembers(err))
		})
	}
}

// invalidMembers returns the members that err reports as invalid.
func invalidMembers(err error) []string {
	errs, _ := errors.AsType[validation.Errors](err)

	return slices.Collect(maps.Keys(errs))
}
