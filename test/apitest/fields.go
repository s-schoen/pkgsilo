package apitest

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
)

// AssertReadableMembers asserts that every member of the response body type except id is an envelope.Field
// with the omitzero option, so that a hidden field is absent from the response, and that these members are
// exactly the readable fields, which each have a read action of their own in the service. A member that's
// no envelope.Field couldn't be hidden, and one without a read action would always be.
func AssertReadableMembers(tb testing.TB, body reflect.Type, readable []string) {
	tb.Helper()

	fieldPkg := reflect.TypeFor[envelope.Field[any]]().PkgPath()
	var members []string
	for field := range body.Fields() {
		name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "id" {
			continue
		}
		members = append(members, name)

		if field.Type.PkgPath() != fieldPkg || !strings.HasPrefix(field.Type.Name(), "Field[") {
			tb.Errorf("member %q of %s is a %s, not an envelope.Field, so it can't be hidden", name, body, field.Type)
		}
		if !strings.Contains(","+options+",", ",omitzero,") {
			tb.Errorf("member %q of %s lacks the omitzero option, so it would be null when hidden", name, body)
		}
	}

	assert.ElementsMatch(tb, readable, members, "the members of %s except id must be the readable fields", body)
}
