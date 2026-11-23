package apitest

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
	"pkgsilo/core/typeschema"
)

// AssertReadableMembers asserts that every member of the response body type except id is an envelope.Field,
// so that a hidden field is absent from the response, and that these members, except allowedActions, are
// exactly the readable fields, which each have a read action of their own in the service. A member that's no
// envelope.Field couldn't be hidden, and one without a read action would always be.
func AssertReadableMembers(tb testing.TB, body reflect.Type, readable []string) {
	tb.Helper()

	var members []string
	for field := range body.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "id" {
			continue
		}
		if !isField(field.Type) {
			tb.Errorf("member %q of %s is a %s, not an envelope.Field, so it can't be hidden", name, body, field.Type)
		}
		if name != allowedActionsMember {
			members = append(members, name)
		}
	}

	assert.ElementsMatch(tb, readable, members, "the members of %s except id and %s must be the readable fields",
		body, allowedActionsMember)
}

// AssertAllowedActionsMember asserts that the response body type has the member allowedActions, an
// envelope.Field of a slice of a string type that implements typeschema.Enum with the given values, the
// names that service's TypeActions.Allowable returns, so that the schema names the actions that it can hold.
func AssertAllowedActionsMember(tb testing.TB, body reflect.Type, allowable []string) {
	tb.Helper()

	for field := range body.Fields() {
		if name, _, _ := strings.Cut(field.Tag.Get("json"), ","); name != allowedActionsMember {
			continue
		}

		value := field.Type
		if isField(value) {
			value = value.Field(0).Type
		}
		if !isField(field.Type) || value.Kind() != reflect.Slice || value.Elem().Kind() != reflect.String ||
			!value.Elem().Implements(reflect.TypeFor[typeschema.Enum]()) {
			tb.Errorf("member %s of %s is a %s, not an envelope.Field of a slice of a string type that "+
				"implements typeschema.Enum", allowedActionsMember, body, field.Type)
			return
		}
		enum := reflect.Zero(value.Elem()).Interface().(typeschema.Enum) //nolint:forcetypeassert // Checked above.
		assert.Equal(tb, allowable, enum.EnumValues(), "the enum of %s of %s", allowedActionsMember, body)
		return
	}

	tb.Errorf("%s has no member %s", body, allowedActionsMember)
}

// allowedActionsMember is the JSON name of the member of a response body that lists the allowed actions.
const allowedActionsMember = "allowedActions"

// isField reports whether t is an envelope.Field.
func isField(t reflect.Type) bool {
	return t.PkgPath() == reflect.TypeFor[envelope.Field[any]]().PkgPath() && strings.HasPrefix(t.Name(), "Field[")
}
