package api

import (
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
)

func TestIdentityRequests_Validate(t *testing.T) {
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
		{name: "display name too long", username: "ada", displayName: strings.Repeat("é", 129), wantInvalid: []string{"displayName"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := map[string]validation.Validatable{
				"create":  CreateIdentityRequest{Username: tt.username, DisplayName: tt.displayName},
				"replace": ReplaceIdentityRequest{Username: tt.username, DisplayName: tt.displayName},
			}
			for kind, req := range requests {
				err := req.Validate()
				if len(tt.wantInvalid) == 0 {
					assert.NoError(t, err, kind)
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
	if errs, ok := err.(validation.Errors); ok {
		for member := range errs {
			members = append(members, member)
		}
	}

	return members
}
