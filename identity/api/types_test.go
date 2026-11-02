package api

import (
	"errors"
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// invalidMembers returns the members that err reports as invalid.
func invalidMembers(err error) []string {
	var members []string
	if errs, ok := errors.AsType[validation.Errors](err); ok {
		for member := range errs {
			members = append(members, member)
		}
	}

	return members
}
