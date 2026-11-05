package api

import (
	"errors"
	"strings"
	"testing"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateKeyRequest_Validate(t *testing.T) {
	t.Parallel()
	future, past, zero := time.Now().Add(time.Minute), time.Now().Add(-time.Minute), time.Time{}
	tests := []struct {
		name      string
		keyName   string
		expiresAt *time.Time
		// wantInvalid are the members that fail validation.
		wantInvalid []string
	}{
		{name: "valid", keyName: "ci", expiresAt: &future},
		{name: "never expires", keyName: "ci"},
		{name: "longest name", keyName: strings.Repeat("é", 128)},
		{name: "name missing", wantInvalid: []string{"name"}},
		{name: "name too long", keyName: strings.Repeat("é", 129), wantInvalid: []string{"name"}},
		{name: "name whitespace", keyName: " \t ", wantInvalid: []string{"name"}},
		{name: "name NUL", keyName: "ci\x00", wantInvalid: []string{"name"}},
		{name: "name line break", keyName: "ci\nkey", wantInvalid: []string{"name"}},
		{name: "expired", keyName: "ci", expiresAt: &past, wantInvalid: []string{"expiresAt"}},
		{name: "expires at zero time", keyName: "ci", expiresAt: &zero, wantInvalid: []string{"expiresAt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := CreateKeyRequest{Name: tt.keyName, ExpiresAt: tt.expiresAt}.Validate()
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
	var members []string
	if errs, ok := errors.AsType[validation.Errors](err); ok {
		for member := range errs {
			members = append(members, member)
		}
	}

	return members
}
