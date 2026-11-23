package apitest

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
	"pkgsilo/test/tbtest"
)

func TestAssertReadableMembers(t *testing.T) {
	t.Parallel()
	type masked struct {
		ID             string                 `json:"id"`
		Name           envelope.Field[string] `json:"name"`
		AllowedActions envelope.Field[[]enum] `json:"allowedActions,omitzero"`
	}
	type plain struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	tests := []struct {
		name        string
		body        reflect.Type
		readable    []string
		wantMessage string
	}{
		{name: "masked", body: reflect.TypeFor[masked](), readable: []string{"name"}},
		{
			name: "plain member", body: reflect.TypeFor[plain](), readable: []string{"name"},
			wantMessage: "not an envelope.Field",
		},
		{name: "member without action", body: reflect.TypeFor[masked](), wantMessage: "must be the readable fields"},
		{
			name: "action without member", body: reflect.TypeFor[masked](), readable: []string{"name", "email"},
			wantMessage: "must be the readable fields",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := tbtest.Run(t, func(tb *tbtest.Recorder) { AssertReadableMembers(tb, tt.body, tt.readable) })

			assert.Equal(t, tt.wantMessage != "", rec.Failed(), rec.Messages())
			assert.Contains(t, rec.Messages(), tt.wantMessage)
		})
	}
}

// enum is a string type with the values of an enum.
type enum string

func (enum) EnumValues() []string {
	return []string{"thing:delete", "thing:archive"}
}

func TestAssertAllowedActionsMember(t *testing.T) {
	t.Parallel()
	type valid struct {
		AllowedActions envelope.Field[[]enum] `json:"allowedActions,omitzero"`
	}
	type missing struct {
		ID string `json:"id"`
	}
	type plain struct {
		AllowedActions []enum `json:"allowedActions"`
	}
	type strings struct {
		AllowedActions envelope.Field[[]string] `json:"allowedActions,omitzero"`
	}
	allowable := []string{"thing:delete", "thing:archive"}
	tests := []struct {
		name        string
		body        reflect.Type
		allowable   []string
		wantMessage string
	}{
		{name: "valid", body: reflect.TypeFor[valid](), allowable: allowable},
		{name: "missing", body: reflect.TypeFor[missing](), allowable: allowable, wantMessage: "has no member"},
		{name: "not a field", body: reflect.TypeFor[plain](), allowable: allowable, wantMessage: "not an envelope.Field"},
		{name: "no enum", body: reflect.TypeFor[strings](), allowable: allowable, wantMessage: "typeschema.Enum"},
		{
			name: "other values", body: reflect.TypeFor[valid](), allowable: []string{"thing:delete"},
			wantMessage: "the enum of allowedActions",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := tbtest.Run(t, func(tb *tbtest.Recorder) { AssertAllowedActionsMember(tb, tt.body, tt.allowable) })

			assert.Equal(t, tt.wantMessage != "", rec.Failed(), rec.Messages())
			assert.Contains(t, rec.Messages(), tt.wantMessage)
		})
	}
}
