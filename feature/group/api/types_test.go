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

func TestGroupRequests_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		groupName string
		// wantInvalid are the members that fail validation.
		wantInvalid []string
	}{
		{name: "valid", groupName: "Maintainers"},
		{name: "free text", groupName: "Release managers (EU) / on-call"},
		{name: "shortest", groupName: "a"},
		{name: "longest", groupName: strings.Repeat("é", 128)},
		{name: "blank", wantInvalid: []string{"name"}},
		{name: "too long", groupName: strings.Repeat("é", 129), wantInvalid: []string{"name"}},
		{name: "whitespace", groupName: " \t ", wantInvalid: []string{"name"}},
		{name: "NUL", groupName: "Main\x00tainers", wantInvalid: []string{"name"}},
		{name: "line break", groupName: "Main\ntainers", wantInvalid: []string{"name"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			requests := map[string]validation.Validatable{
				"create":  CreateGroupRequest{Name: tt.groupName},
				"replace": ReplaceGroupRequest{Name: tt.groupName},
				"patch":   PatchGroupRequest{Name: request.Optional[string]{Value: tt.groupName, Set: true}},
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
	errs, _ := errors.AsType[validation.Errors](err)

	return slices.Collect(maps.Keys(errs))
}
