// Package servicetest provides assertions and mock helpers for service tests.
package servicetest

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"pkgsilo/core/authz"
	authzmocks "pkgsilo/core/authz/mocks"
)

// AssertTranslated asserts that err, returned by a service after the layer below failed with cause,
// matches want. Unless want is cause, as for an unexpected error that the service only wraps, err
// must no longer match cause: errors of the layer below must not leak into the domain.
func AssertTranslated(tb testing.TB, err, want, cause error) {
	tb.Helper()
	if !errors.Is(want, cause) && errors.Is(err, cause) {
		tb.Errorf("error %q still matches %q: errors of the layer below must not leak into the domain", err, cause)
	}
	assert.ErrorIs(tb, err, want)
}

// Capture returns a mock argument matcher that matches any value of type T and stores it in dst,
// so that an argument can be asserted on as a whole after the call, such as a record whose ID the
// service generates.
//
// testify runs the matcher whenever it compares a call with the expectation, so use it in the
// method's only expectation: dst then holds the argument of the method's latest call.
func Capture[T any](dst *T) any {
	return mock.MatchedBy(func(v T) bool {
		*dst = v
		return true
	})
}

// ExpectReadableFields makes the decider allow reading the fields with the given names of the resource, and
// deny reading its other fields out of fields, the read field actions of a service. Each field is expected
// to be checked once, as when a service builds the visible object.
func ExpectReadableFields(
	decider *authzmocks.MockDecider, fields authz.FieldActions, resource authz.Resource, readable ...string,
) {
	names := fields.Fields()
	for i, action := range fields.Actions() {
		decider.EXPECT().Can(action, resource).Return(slices.Contains(readable, names[i]), nil).Once()
	}
}
