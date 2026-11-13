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

func TestPolicyRequests_Validate(t *testing.T) {
	t.Parallel()
	const source = `permit (principal, action, resource);`
	tests := []struct {
		name, policyName, source string
		// wantInvalid are the members that fail validation.
		wantInvalid []string
	}{
		{name: "valid", policyName: "everything", source: source},
		{name: "longest name", policyName: strings.Repeat("é", 128), source: source},
		// The service checks the source, and answers a mistake with its own problem.
		{name: "source that isn't Cedar", policyName: "everything", source: "not cedar"},
		{name: "blank", wantInvalid: []string{"name", "source"}},
		{name: "name too long", policyName: strings.Repeat("é", 129), source: source, wantInvalid: []string{"name"}},
		{name: "name whitespace", policyName: " \t ", source: source, wantInvalid: []string{"name"}},
		{name: "name line break", policyName: "every\nthing", source: source, wantInvalid: []string{"name"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			requests := map[string]validation.Validatable{
				"create":  CreatePolicyRequest{Name: tt.policyName, Source: tt.source},
				"replace": ReplacePolicyRequest{Name: tt.policyName, Source: tt.source},
				"patch": PatchPolicyRequest{
					Name:   request.Optional[string]{Value: tt.policyName, Set: true},
					Source: request.Optional[string]{Value: tt.source, Set: true},
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

// invalidMembers returns the members that err reports as invalid.
func invalidMembers(err error) []string {
	errs, _ := errors.AsType[validation.Errors](err)

	return slices.Collect(maps.Keys(errs))
}
